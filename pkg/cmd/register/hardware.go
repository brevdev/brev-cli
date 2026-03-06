package register

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// HardwareProfiler abstracts system hardware detection so it can be
// replaced in tests and implemented per-platform via build tags.
type HardwareProfiler interface {
	Profile() (*HardwareProfile, error)
}

// GPU describes a group of identical GPUs on the system.
type GPU struct {
	Model        string `json:"model"`
	Architecture string `json:"architecture,omitempty"`
	Count        int32  `json:"count"`
	MemoryBytes  *int64 `json:"memory_bytes,omitempty"`
}

// Interconnect describes a GPU interconnect (NVLink, PCIe, etc.).
type Interconnect struct {
	Type        string `json:"type"`
	Device      string `json:"device"`
	ActiveLinks int    `json:"active_links,omitempty"`
	Version     uint32 `json:"version,omitempty"`
	Generation  int    `json:"generation,omitempty"`
	Width       int    `json:"width,omitempty"`
}

// StorageDevice represents a single block storage device.
type StorageDevice struct {
	Name         string `json:"name,omitempty"`
	StorageBytes int64  `json:"storage_bytes"`
	StorageType  string `json:"storage_type,omitempty"` // "SSD", "HDD", or "NVMe"
}

// HardwareProfile is the full hardware snapshot collected by a HardwareProfiler.
type HardwareProfile struct {
	GPUs          []GPU           `json:"gpus"`
	RAMBytes      *int64          `json:"ram_bytes,omitempty"`
	CPUCount      *int32          `json:"cpu_count,omitempty"`
	Architecture  string          `json:"architecture,omitempty"`
	Storage       []StorageDevice `json:"storage,omitempty"`
	OS            string          `json:"os,omitempty"`
	OSVersion     string          `json:"os_version,omitempty"`
	ProductName   string          `json:"product_name,omitempty"`
	Interconnects []Interconnect  `json:"interconnects,omitempty"`
}

// FormatHardwareProfile returns a human-readable summary of the hardware profile.
func FormatHardwareProfile(s *HardwareProfile) string {
	var b strings.Builder
	if s.ProductName != "" {
		_, _ = fmt.Fprintf(&b, "    Product: %s\n", s.ProductName)
	}
	if s.CPUCount != nil {
		_, _ = fmt.Fprintf(&b, "    CPU:     %d cores\n", *s.CPUCount)
	}
	if s.RAMBytes != nil {
		_, _ = fmt.Fprintf(&b, "    RAM:     %.1f GB\n", float64(*s.RAMBytes)/(1024*1024*1024))
	}
	for _, gpu := range s.GPUs {
		if gpu.MemoryBytes != nil {
			memGB := float64(*gpu.MemoryBytes) / (1024 * 1024 * 1024)
			_, _ = fmt.Fprintf(&b, "    GPUs:    %d x %s (%.1f GB)", gpu.Count, gpu.Model, memGB)
		} else {
			_, _ = fmt.Fprintf(&b, "    GPUs:    %d x %s", gpu.Count, gpu.Model)
		}
		if gpu.Architecture != "" {
			_, _ = fmt.Fprintf(&b, " [%s]", gpu.Architecture)
		}
		b.WriteString("\n")
	}
	_, _ = fmt.Fprintf(&b, "    Arch:    %s\n", s.Architecture)
	if s.OS != "" || s.OSVersion != "" {
		_, _ = fmt.Fprintf(&b, "    OS:      %s %s\n", s.OS, s.OSVersion)
	}
	for _, ic := range s.Interconnects {
		_, _ = fmt.Fprintf(&b, "    Link:    %s", ic.Type)
		if ic.Device != "" {
			_, _ = fmt.Fprintf(&b, " (%s)", ic.Device)
		}
		if ic.ActiveLinks > 0 {
			_, _ = fmt.Fprintf(&b, " x%d", ic.ActiveLinks)
		}
		b.WriteString("\n")
	}
	for _, st := range s.Storage {
		_, _ = fmt.Fprintf(&b, "    Storage: %.1f GB", float64(st.StorageBytes)/(1024*1024*1024))
		if st.StorageType != "" {
			_, _ = fmt.Fprintf(&b, " (%s)", st.StorageType)
		}
		if st.Name != "" {
			_, _ = fmt.Fprintf(&b, " [%s]", st.Name)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// --- Content-parsing helpers (pure functions, used by Linux adapter and tests) ---

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
	return strings.Trim(strings.TrimSpace(s), "\"")
}

// parseStorageOutput parses lsblk output (NAME,SIZE,TYPE,ROTA columns),
// returning one StorageDevice entry per disk device. ROTA=0 → SSD, ROTA=1 → HDD.
func parseStorageOutput(output string) []StorageDevice {
	var devices []StorageDevice
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[2] != "disk" {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		entry := StorageDevice{Name: fields[0], StorageBytes: size}
		rota, err := strconv.Atoi(fields[3])
		if err == nil {
			if rota == 0 {
				entry.StorageType = "SSD"
			} else {
				entry.StorageType = "HDD"
			}
		}
		devices = append(devices, entry)
	}
	return devices
}
