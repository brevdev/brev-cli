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

// HardwareProfile mirrors the proto HardwareInfo message.
type HardwareProfile struct {
	GPUs         []GPUInfo     `json:"gpus"`
	RAMBytes     int64         `json:"ram_bytes"`
	CPUCount     int           `json:"cpu_count"`
	CPUModel     string        `json:"cpu_model,omitempty"`
	SystemVendor string        `json:"system_vendor,omitempty"`
	SystemModel  string        `json:"system_model,omitempty"`
	Architecture string        `json:"architecture,omitempty"`
	OSName       string        `json:"os_name,omitempty"`
	OSVersion    string        `json:"os_version,omitempty"`
	Storage      []StorageInfo `json:"storage,omitempty"`
}

// GPUInfo describes a single GPU.
type GPUInfo struct {
	Name          string `json:"name"`
	MemoryMB      int64  `json:"memory_mb"`
	DriverVersion string `json:"driver_version,omitempty"`
	PCIBusID      string `json:"pci_bus_id,omitempty"`
}

// StorageInfo describes a block storage device.
type StorageInfo struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	Type  string `json:"type"`
}

// FileReader abstracts file reading for testability.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// CollectHardwareProfile gathers system hardware information.
// CPU count/model and RAM are required; everything else is best-effort.
func CollectHardwareProfile(runner CommandRunner, reader FileReader) (*HardwareProfile, error) {
	profile := &HardwareProfile{
		Architecture: runtime.GOARCH,
	}

	cpuCount, cpuModel, err := parseCPUInfo(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read CPU info: %w", err)
	}
	profile.CPUCount = cpuCount
	profile.CPUModel = cpuModel

	ramBytes, err := parseMemInfo(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory info: %w", err)
	}
	profile.RAMBytes = ramBytes

	osName, osVersion := parseOSRelease(reader)
	profile.OSName = osName
	profile.OSVersion = osVersion

	profile.SystemVendor = readSysFile(reader, "/sys/class/dmi/id/sys_vendor")
	profile.SystemModel = readSysFile(reader, "/sys/class/dmi/id/product_name")

	profile.GPUs = parseNvidiaSMI(runner)
	profile.Storage = parseLsblk(runner)

	return profile, nil
}

// parseCPUInfo reads /proc/cpuinfo and returns (count, model).
func parseCPUInfo(reader FileReader) (int, string, error) {
	data, err := reader.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, "", breverrors.WrapAndTrace(err)
	}
	return parseCPUInfoContent(string(data))
}

// parseCPUInfoContent parses the content of /proc/cpuinfo.
func parseCPUInfoContent(content string) (int, string, error) {
	count := 0
	model := ""
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			count++
		}
		if strings.HasPrefix(line, "model name") && model == "" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
			}
		}
	}
	if count == 0 {
		return 0, "", fmt.Errorf("no processors found in /proc/cpuinfo")
	}
	return count, model, nil
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

// readSysFile reads a single-line sysfs file, returning empty string on failure.
func readSysFile(reader FileReader, path string) string {
	data, err := reader.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parseNvidiaSMI queries nvidia-smi for GPU information.
func parseNvidiaSMI(runner CommandRunner) []GPUInfo {
	out, err := runner.Run("nvidia-smi",
		"--query-gpu=name,memory.total,driver_version,pci.bus_id",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil
	}
	return parseNvidiaSMIOutput(string(out))
}

// parseNvidiaSMIOutput parses nvidia-smi CSV output into GPUInfo slices.
func parseNvidiaSMIOutput(output string) []GPUInfo {
	var gpus []GPUInfo
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
		gpu := GPUInfo{
			Name: strings.TrimSpace(parts[0]),
		}
		memMB, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err == nil {
			gpu.MemoryMB = memMB
		}
		if len(parts) >= 3 {
			gpu.DriverVersion = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			gpu.PCIBusID = strings.TrimSpace(parts[3])
		}
		gpus = append(gpus, gpu)
	}
	return gpus
}

// parseLsblk queries lsblk for block device information.
func parseLsblk(runner CommandRunner) []StorageInfo {
	out, err := runner.Run("lsblk", "-b", "-d", "-n", "-o", "NAME,SIZE,TYPE")
	if err != nil {
		return nil
	}
	return parseLsblkOutput(string(out))
}

// parseLsblkOutput parses lsblk output into StorageInfo slices.
func parseLsblkOutput(output string) []StorageInfo {
	var devices []StorageInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		devices = append(devices, StorageInfo{
			Name:  fields[0],
			Bytes: size,
			Type:  fields[2],
		})
	}
	return devices
}

// FormatHardwareProfile returns a human-readable summary of the hardware profile.
func FormatHardwareProfile(p *HardwareProfile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    CPU:     %d x %s\n", p.CPUCount, p.CPUModel)
	fmt.Fprintf(&b, "    RAM:     %d GB\n", p.RAMBytes/(1024*1024*1024))
	if len(p.GPUs) > 0 {
		// Group GPUs by name
		gpuCounts := make(map[string]int)
		gpuMemory := make(map[string]int64)
		var gpuOrder []string
		for _, gpu := range p.GPUs {
			if gpuCounts[gpu.Name] == 0 {
				gpuOrder = append(gpuOrder, gpu.Name)
			}
			gpuCounts[gpu.Name]++
			gpuMemory[gpu.Name] = gpu.MemoryMB
		}
		for _, name := range gpuOrder {
			memGB := gpuMemory[name] / 1024
			fmt.Fprintf(&b, "    GPUs:    %d x %s (%d GB)\n", gpuCounts[name], name, memGB)
		}
	} else {
		b.WriteString("    GPUs:    none detected\n")
	}
	fmt.Fprintf(&b, "    Arch:    %s\n", p.Architecture)
	if p.OSName != "" || p.OSVersion != "" {
		fmt.Fprintf(&b, "    OS:      %s %s\n", p.OSName, p.OSVersion)
	}
	return b.String()
}
