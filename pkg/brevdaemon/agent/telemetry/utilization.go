package telemetry

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/dev-plane/pkg/errors"
	"golang.org/x/sys/unix"
)

// UtilizationInfo captures runtime metrics reported via heartbeat.
type UtilizationInfo struct {
	CPUPercent       float32
	MemoryUsedBytes  int64
	MemoryTotalBytes int64
	DiskPercent      float32
	DiskUsedBytes    int64
	DiskTotalBytes   int64
	GPUs             []GPUUtilization
}

// GPUUtilization mirrors the subset of nvidia-smi fields used in heartbeats.
type GPUUtilization struct {
	Index              int
	Model              string
	UtilizationPercent float32
	MemoryUsedBytes    int64
	MemoryTotalBytes   int64
	TemperatureCelsius *float32
}

// ToClient converts the telemetry DTO into the client payload type.
func (u UtilizationInfo) ToClient() *client.UtilizationInfo {
	out := &client.UtilizationInfo{
		CPUPercent:       u.CPUPercent,
		MemoryUsedBytes:  u.MemoryUsedBytes,
		MemoryTotalBytes: u.MemoryTotalBytes,
		DiskPercent:      u.DiskPercent,
		DiskUsedBytes:    u.DiskUsedBytes,
		DiskTotalBytes:   u.DiskTotalBytes,
	}
	if len(u.GPUs) > 0 {
		out.GPUs = make([]client.GPUUtilization, 0, len(u.GPUs))
		for _, gpu := range u.GPUs {
			out.GPUs = append(out.GPUs, client.GPUUtilization{
				Index:              gpu.Index,
				Model:              gpu.Model,
				UtilizationPercent: gpu.UtilizationPercent,
				MemoryUsedBytes:    gpu.MemoryUsedBytes,
				MemoryTotalBytes:   gpu.MemoryTotalBytes,
				TemperatureCelsius: gpu.TemperatureCelsius,
			})
		}
	}
	return out
}

const cpuSampleInterval = 200 * time.Millisecond

var (
	statfsFunc    = unix.Statfs
	cpuSampleWait = cpuSampleInterval
)

// SampleUtilization returns best-effort runtime metrics.
func SampleUtilization(ctx context.Context) (UtilizationInfo, error) {
	util := UtilizationInfo{}

	if runtimeGOOS == "linux" {
		used, total, err := readLinuxMemory()
		if err != nil {
			return UtilizationInfo{}, errors.WrapAndTrace(err)
		}
		util.MemoryUsedBytes = used
		util.MemoryTotalBytes = total

		if pct, err := sampleLinuxCPUPercent(ctx); err == nil {
			util.CPUPercent = pct
		} else {
			return UtilizationInfo{}, errors.WrapAndTrace(err)
		}
	} else {
		if total, err := systemRAMBytes(); err == nil {
			util.MemoryTotalBytes = total
		}
	}

	if used, total, pct, err := diskUsage("/"); err == nil {
		util.DiskUsedBytes = used
		util.DiskTotalBytes = total
		util.DiskPercent = pct
	}

	if gpus, err := detectGPUUtilization(ctx); err == nil {
		util.GPUs = gpus
	}

	return util, nil
}

func readLinuxMemory() (used, total int64, err error) {
	data, err := readFile("/proc/meminfo")
	if err != nil {
		return 0, 0, errors.WrapAndTrace(err)
	}
	total, available, err := parseMeminfo(data)
	if err != nil {
		return 0, 0, errors.WrapAndTrace(err)
	}
	return total - available, total, nil
}

func parseMeminfo(data []byte) (total, available int64, err error) { //nolint:gocyclo // parser handles multiple meminfo keys
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var (
		memFree int64
		buffers int64
		cached  int64
	)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total, err = parseMeminfoValue(line)
			if err != nil {
				return 0, 0, err
			}
		case strings.HasPrefix(line, "MemAvailable:"):
			available, err = parseMeminfoValue(line)
			if err != nil {
				return 0, 0, err
			}
		case strings.HasPrefix(line, "MemFree:"):
			memFree, err = parseMeminfoValue(line)
			if err != nil {
				return 0, 0, err
			}
		case strings.HasPrefix(line, "Buffers:"):
			buffers, err = parseMeminfoValue(line)
			if err != nil {
				return 0, 0, err
			}
		case strings.HasPrefix(line, "Cached:"):
			cached, err = parseMeminfoValue(line)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, errors.WrapAndTrace(err)
	}
	if total == 0 {
		return 0, 0, errors.Errorf("meminfo missing MemTotal")
	}
	if available == 0 {
		available = memFree + buffers + cached
	}
	if available > total {
		available = total
	}
	return total, available, nil
}

func parseMeminfoValue(line string) (int64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, errors.Errorf("invalid meminfo line: %s", line)
	}
	value, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, errors.WrapAndTrace(err)
	}
	// Values are reported in kB.
	return value * 1024, nil
}

func sampleLinuxCPUPercent(ctx context.Context) (float32, error) {
	first, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	wait := cpuSampleWait
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return 0, errors.WrapAndTrace(ctx.Err())
		case <-timer.C:
		}
	} else {
		select {
		case <-ctx.Done():
			return 0, errors.WrapAndTrace(ctx.Err())
		default:
		}
	}

	second, err := readCPUTimes()
	if err != nil {
		return 0, err
	}

	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if totalDelta <= 0 {
		return 0, nil
	}
	usage := float32(totalDelta-idleDelta) / float32(totalDelta) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, nil
}

type cpuTimes struct {
	idle  uint64
	total uint64
}

func readCPUTimes() (cpuTimes, error) {
	data, err := readFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, errors.WrapAndTrace(err)
	}
	return parseCPUStat(data)
}

func parseCPUStat(data []byte) (cpuTimes, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return cpuTimes{}, errors.WrapAndTrace(err)
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return cpuTimes{}, errors.Errorf("invalid cpu stat line: %s", line)
	}
	var total uint64
	var idle uint64
	for idx, field := range fields[1:] {
		val, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return cpuTimes{}, errors.WrapAndTrace(parseErr)
		}
		total += val
		if idx == 3 {
			idle = val
		}
		if idx == 4 {
			idle += val // iowait counts as idle
		}
	}
	return cpuTimes{idle: idle, total: total}, nil
}

func diskUsage(path string) (used, total int64, percent float32, err error) {
	var fs unix.Statfs_t
	if err := statfsFunc(path, &fs); err != nil {
		return 0, 0, 0, errors.WrapAndTrace(err)
	}
	blockSize := int64(fs.Bsize)
	totalBlocks, err := safeUint64ToInt64(fs.Blocks)
	if err != nil {
		return 0, 0, 0, errors.WrapAndTrace(err)
	}
	freeBlocks, err := safeUint64ToInt64(fs.Bfree)
	if err != nil {
		return 0, 0, 0, errors.WrapAndTrace(err)
	}
	total, err = safeMulInt64(totalBlocks, blockSize)
	if err != nil {
		return 0, 0, 0, errors.WrapAndTrace(err)
	}
	free, err := safeMulInt64(freeBlocks, blockSize)
	if err != nil {
		return 0, 0, 0, errors.WrapAndTrace(err)
	}
	used = total - free
	if total > 0 {
		percent = float32(used) / float32(total) * 100
	}
	return used, total, percent, nil
}

func detectGPUUtilization(ctx context.Context) ([]GPUUtilization, error) {
	if _, err := lookupExecutable("nvidia-smi"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, errors.WrapAndTrace(err)
	}

	out, err := execCommand(ctx, "nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return parseGPUUtilization(string(out))
}

func parseGPUUtilization(raw string) ([]GPUUtilization, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	results := make([]GPUUtilization, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 6 {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		utilPercent, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 32)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		memUsed, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		memTotal, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		tempStr := strings.TrimSpace(fields[5])
		var tempPtr *float32
		if tempStr != "" {
			tempVal, err := strconv.ParseFloat(tempStr, 32)
			if err != nil {
				return nil, errors.WrapAndTrace(err)
			}
			tempF := float32(tempVal)
			tempPtr = &tempF
		}
		results = append(results, GPUUtilization{
			Index:              index,
			Model:              strings.TrimSpace(fields[1]),
			UtilizationPercent: float32(utilPercent),
			MemoryUsedBytes:    int64(memUsed * 1024 * 1024),
			MemoryTotalBytes:   int64(memTotal * 1024 * 1024),
			TemperatureCelsius: tempPtr,
		})
	}
	return results, nil
}

func safeUint64ToInt64(val uint64) (int64, error) {
	if val > math.MaxInt64 {
		return 0, errors.Errorf("value %d exceeds int64 range", val)
	}
	return int64(val), nil
}

func safeMulInt64(a, b int64) (int64, error) {
	result := big.NewInt(0).Mul(big.NewInt(a), big.NewInt(b))
	if !result.IsInt64() {
		return 0, errors.Errorf("multiplication overflow")
	}
	return result.Int64(), nil
}
