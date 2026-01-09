package telemetry

import (
	"context"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/dev-plane/pkg/errors"
	"golang.org/x/sys/unix"
)

// HardwareInfo holds the coarse specs captured during registration.
type HardwareInfo struct {
	CPUCount     int
	RAMBytes     int64
	GPUs         []GPUInfo
	MachineModel string
	Architecture string
	Storage      []StorageInfo
}

// GPUInfo aggregates GPUs with identical model/memory characteristics.
type GPUInfo struct {
	Model       string
	MemoryBytes int64
	Count       int
}

// StorageInfo captures best-effort block device capacity.
type StorageInfo struct {
	Name      string
	SizeBytes int64
	Type      string
}

// DetectHardware collects CPU count, memory, and GPU details for the host.
func DetectHardware(ctx context.Context) (HardwareInfo, error) {
	hw := HardwareInfo{
		CPUCount:     runtime.NumCPU(),
		Architecture: runtime.GOARCH,
	}

	ram, err := systemRAMBytes()
	if err != nil {
		return HardwareInfo{}, errors.WrapAndTrace(err)
	}
	hw.RAMBytes = ram

	gpus, err := detectGPUs(ctx)
	if err != nil {
		return HardwareInfo{}, errors.WrapAndTrace(err)
	}
	hw.GPUs = gpus

	hw.Storage = detectStorage(ctx)
	hw.MachineModel = detectMachineModel()

	return hw, nil
}

// ToProto converts the telemetry DTO into the protobuf request payload used by the agent RPCs.
func (h HardwareInfo) ToProto() *brevapiv2.HardwareInfo {
	out := &brevapiv2.HardwareInfo{
		CpuCount: clampToInt32(h.CPUCount),
	}
	if h.RAMBytes > 0 {
		out.RamBytes = bytesValue(h.RAMBytes)
	}
	if h.MachineModel != "" {
		out.SystemModel = protoString(h.MachineModel)
	}
	if h.Architecture != "" {
		out.Architecture = protoString(h.Architecture)
	}
	if len(h.Storage) > 0 {
		out.Storage = make([]*brevapiv2.StorageInfo, 0, len(h.Storage))
		for _, s := range h.Storage {
			entry := &brevapiv2.StorageInfo{
				Name: s.Name,
				Type: s.Type,
			}
			if s.SizeBytes > 0 {
				entry.Capacity = bytesValue(s.SizeBytes)
			}
			out.Storage = append(out.Storage, entry)
		}
	}
	if len(h.GPUs) > 0 {
		out.Gpus = make([]*brevapiv2.GPUInfo, 0, len(h.GPUs))
		for _, gpu := range h.GPUs {
			out.Gpus = append(out.Gpus, &brevapiv2.GPUInfo{
				Model:       gpu.Model,
				Count:       clampToInt32(gpu.Count),
				MemoryBytes: bytesValue(gpu.MemoryBytes),
			})
		}
	}
	return out
}

var (
	runtimeGOOS      = runtime.GOOS
	readFile         = osReadFile
	lookupExecutable = exec.LookPath
	execCommand      = runCommand
)

func detectMachineModel() string {
	if runtimeGOOS != goosLinux {
		return ""
	}
	paths := []string{
		"/sys/devices/virtual/dmi/id/product_name",
		"/sys/devices/virtual/dmi/id/board_name",
	}
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(data))
		if val != "" {
			return val
		}
	}
	return ""
}

// detectStorage uses lsblk (if available) to list block devices with their sizes.
// It is best-effort and ignores errors; only linux is supported.
func detectStorage(ctx context.Context) []StorageInfo {
	if runtimeGOOS != goosLinux {
		return nil
	}
	if _, err := lookupExecutable("lsblk"); err != nil {
		return nil
	}
	// -b: bytes, -d: no partitions, -o: NAME,TYPE,SIZE
	out, err := execCommand(ctx, "lsblk", "-b", "-d", "-o", "NAME,TYPE,SIZE")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return nil
	}
	var storage []StorageInfo
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		typ := fields[1]
		// Ignore loopback and other virtual block devices that are not real disks.
		if typ == "loop" {
			continue
		}
		sizeStr := fields[2]
		if name == "" || typ == "" || sizeStr == "" {
			continue
		}
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil || size <= 0 {
			continue
		}
		storage = append(storage, StorageInfo{
			Name:      name,
			SizeBytes: size,
			Type:      typ,
		})
	}
	return storage
}

func systemRAMBytes() (int64, error) {
	switch runtimeGOOS {
	case goosLinux:
		data, err := readFile("/proc/meminfo")
		if err != nil {
			return 0, errors.WrapAndTrace(err)
		}
		total, _, err := parseMeminfo(data)
		return total, err
	case darwin:
		value, err := darwinMemoryBytes()
		if err != nil {
			return 0, errors.WrapAndTrace(err)
		}
		return value, nil
	default:
		return 0, errors.Errorf("unsupported platform %s", runtimeGOOS)
	}
}

func darwinMemoryBytes() (int64, error) {
	value, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, errors.WrapAndTrace(err)
	}
	if value > math.MaxInt64 {
		return 0, errors.Errorf("memsize exceeds supported range")
	}
	return int64(value), nil
}

func detectGPUs(ctx context.Context) ([]GPUInfo, error) {
	if _, err := lookupExecutable("nvidia-smi"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, errors.WrapAndTrace(err)
	}

	out, err := execCommand(ctx, "nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	return parseGPUHardware(string(out))
}

func parseGPUHardware(raw string) ([]GPUInfo, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	counts := map[string]*GPUInfo{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		model := strings.TrimSpace(parts[0])
		if model == "" {
			continue
		}
		memStr := strings.TrimSpace(parts[1])
		if memStr == "" {
			continue
		}

		// Some drivers report "[N/A]" (e.g., GB10 unified memory). Try a known fallback before skipping.
		var memBytes int64
		if strings.EqualFold(memStr, "n/a") || strings.EqualFold(memStr, "[n/a]") {
			if mb, ok := knownMemoryForModel(model); ok {
				memBytes = mb
			} else {
				continue
			}
		} else {
			memMiB, err := strconv.ParseFloat(memStr, 64)
			if err != nil {
				if mb, ok := knownMemoryForModel(model); ok {
					memBytes = mb
				} else {
					// Skip unparsable memory values rather than aborting the agent.
					continue
				}
			} else {
				memBytes = int64(memMiB * 1024 * 1024)
			}
		}

		key := model + "|" + strconv.FormatInt(memBytes, 10)
		entry, ok := counts[key]
		if !ok {
			entry = &GPUInfo{
				Model:       model,
				MemoryBytes: memBytes,
			}
			counts[key] = entry
		}
		entry.Count++
	}

	if len(counts) == 0 {
		return nil, nil
	}

	results := make([]GPUInfo, 0, len(counts))
	for _, gpu := range counts {
		results = append(results, *gpu)
	}
	return results, nil
}

func knownMemoryForModel(model string) (int64, bool) {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "gb10"):
		// NVIDIA GB10 (Grace Blackwell Superchip) uses unified LPDDR5x shared between CPU/GPU.
		// Prefer the system RAM total (unified memory) if available; otherwise fall back to 128GB.
		if ram, err := systemRAMBytes(); err == nil && ram > 0 {
			return ram, true
		}
		return 128 * 1024 * 1024 * 1024, true
	default:
		return 0, false
	}
}

func osReadFile(path string) ([]byte, error) {
	// #nosec G304 -- path is derived from config or OS-specific defaults.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return data, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return out, nil
}

func bytesValue(v int64) *devplaneapiv1.Bytes {
	if v <= 0 {
		return nil
	}
	return &devplaneapiv1.Bytes{Value: v}
}

func clampToInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

func protoString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
