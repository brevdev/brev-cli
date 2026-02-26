package register

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// ExecCommandRunner is the real implementation that runs OS commands.
type ExecCommandRunner struct{}

func (r ExecCommandRunner) Run(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output() // #nosec G204
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

// NodeSpec matches the proto NodeSpec message from dev-plane.
// All fields are best-effort.
type NodeSpec struct {
	GPUs         []NodeGPU `json:"gpus"`
	RAMBytes     *int64    `json:"ram_bytes,omitempty"`
	CPUCount     *int32    `json:"cpu_count,omitempty"`
	Architecture string    `json:"architecture,omitempty"`
	StorageBytes *int64    `json:"storage_bytes,omitempty"`
	StorageType  string    `json:"storage_type,omitempty"`
	OS           string    `json:"os,omitempty"`
	OSVersion    string    `json:"os_version,omitempty"`
}

// NodeGPU matches the proto NodeGPU message.
type NodeGPU struct {
	Model       string `json:"model"`
	Count       int32  `json:"count"`
	MemoryBytes *int64 `json:"memory_bytes,omitempty"`
}

// FileReader abstracts file reading for testability.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// CollectHardwareProfile gathers system hardware information.
// All fields are best-effort; failures are silently ignored.
func CollectHardwareProfile(runner CommandRunner, reader FileReader) (*NodeSpec, error) {
	spec := &NodeSpec{
		Architecture: runtime.GOARCH,
	}

	if gpus, err := parseNvidiaSMI(runner); err == nil {
		spec.GPUs = gpus
	}

	if cpuCount, err := parseCPUCount(reader); err == nil {
		count32 := int32(cpuCount)
		spec.CPUCount = &count32
	}

	if ramBytes, err := parseMemInfo(reader); err == nil {
		spec.RAMBytes = &ramBytes
	}

	osName, osVersion := parseOSRelease(reader)
	spec.OS = osName
	spec.OSVersion = osVersion

	storageBytes, storageType := collectStorage(runner)
	if storageBytes > 0 {
		spec.StorageBytes = &storageBytes
		spec.StorageType = storageType
	}

	return spec, nil
}

// parseCPUCount reads /proc/cpuinfo and returns the number of logical processors.
func parseCPUCount(reader FileReader) (int, error) {
	data, err := reader.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}
	return parseCPUCountContent(string(data))
}

// parseCPUCountContent parses the content of /proc/cpuinfo for processor count.
func parseCPUCountContent(content string) (int, error) {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("no processors found in /proc/cpuinfo")
	}
	return count, nil
}

// parseMemInfo reads /proc/meminfo and returns total RAM in bytes.
func parseMemInfo(reader FileReader) (int64, error) {
	data, err := reader.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}
	return parseMemInfoContent(string(data))
}

// parseMemInfoContent parses the content of /proc/meminfo.
func parseMemInfoContent(content string) (int64, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected MemTotal format: %s", line)
			}
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse MemTotal value: %w", err)
			}
			return kb * 1024, nil // convert kB to bytes
		}
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// parseOSRelease reads /etc/os-release and returns (name, version).
func parseOSRelease(reader FileReader) (string, string) {
	data, err := reader.ReadFile("/etc/os-release")
	if err != nil {
		return "", ""
	}
	return parseOSReleaseContent(string(data))
}

// parseOSReleaseContent parses the content of /etc/os-release.
func parseOSReleaseContent(content string) (string, string) {
	name := ""
	version := ""
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if val, ok := strings.CutPrefix(line, "NAME="); ok {
			name = unquote(val)
		}
		if val, ok := strings.CutPrefix(line, "VERSION_ID="); ok {
			version = unquote(val)
		}
	}
	return name, version
}

// unquote removes surrounding double quotes from a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseNvidiaSMI queries nvidia-smi for GPU information.
// Returns an error if nvidia-smi fails or no GPUs are found.
func parseNvidiaSMI(runner CommandRunner) ([]NodeGPU, error) {
	out, err := runner.Run("nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi not available: %w", err)
	}
	gpus := parseNvidiaSMIOutput(string(out))
	if len(gpus) == 0 {
		return nil, fmt.Errorf("nvidia-smi returned no GPUs")
	}
	return gpus, nil
}

// parseNvidiaSMIOutput parses nvidia-smi CSV output, grouping identical GPU
// models into a single NodeGPU with a count.
func parseNvidiaSMIOutput(output string) []NodeGPU {
	type gpuKey struct {
		model       string
		memoryBytes int64
	}

	counts := make(map[gpuKey]int32)
	var order []gpuKey

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ", ")
		if len(parts) < 2 {
			continue
		}
		model := strings.TrimSpace(parts[0])
		memMB, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		key := gpuKey{model: model, memoryBytes: memMB * 1024 * 1024}
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}

	gpus := make([]NodeGPU, 0, len(order))
	for _, key := range order {
		mem := key.memoryBytes
		gpus = append(gpus, NodeGPU{
			Model:       key.model,
			Count:       counts[key],
			MemoryBytes: &mem,
		})
	}
	return gpus
}

// collectStorage sums disk devices from lsblk to get total storage bytes
// and infers a storage type from the device names.
func collectStorage(runner CommandRunner) (int64, string) {
	out, err := runner.Run("lsblk", "-b", "-d", "-n", "-o", "NAME,SIZE,TYPE")
	if err != nil {
		return 0, ""
	}
	return parseStorageOutput(string(out))
}

// parseStorageOutput parses lsblk output, summing disk device sizes and
// inferring storage type.
func parseStorageOutput(output string) (int64, string) {
	var totalBytes int64
	storageType := ""
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[2] != "disk" {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		totalBytes += size
		if storageType == "" {
			if strings.HasPrefix(fields[0], "nvme") {
				storageType = "NVMe"
			} else {
				storageType = "SSD"
			}
		}
	}
	return totalBytes, storageType
}

// FormatNodeSpec returns a human-readable summary of the hardware profile.
func FormatNodeSpec(s *NodeSpec) string {
	var b strings.Builder
	if s.CPUCount != nil {
		_, _ = fmt.Fprintf(&b, "    CPU:     %d cores\n", *s.CPUCount)
	}
	if s.RAMBytes != nil {
		_, _ = fmt.Fprintf(&b, "    RAM:     %d GB\n", *s.RAMBytes/(1024*1024*1024))
	}
	for _, gpu := range s.GPUs {
		if gpu.MemoryBytes != nil {
			memGB := *gpu.MemoryBytes / (1024 * 1024 * 1024)
			_, _ = fmt.Fprintf(&b, "    GPUs:    %d x %s (%d GB)\n", gpu.Count, gpu.Model, memGB)
		} else {
			_, _ = fmt.Fprintf(&b, "    GPUs:    %d x %s\n", gpu.Count, gpu.Model)
		}
	}
	_, _ = fmt.Fprintf(&b, "    Arch:    %s\n", s.Architecture)
	if s.OS != "" || s.OSVersion != "" {
		_, _ = fmt.Fprintf(&b, "    OS:      %s %s\n", s.OS, s.OSVersion)
	}
	if s.StorageBytes != nil {
		_, _ = fmt.Fprintf(&b, "    Storage: %d GB", *s.StorageBytes/(1024*1024*1024))
		if s.StorageType != "" {
			_, _ = fmt.Fprintf(&b, " (%s)", s.StorageType)
		}
		b.WriteString("\n")
	}
	return b.String()
}
