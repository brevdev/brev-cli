//go:build darwin

package register

import (
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

var diskutilBytesRe = regexp.MustCompile(`\((\d[\d,]*)\s+Bytes\)`)

// SystemHardwareProfiler probes hardware on macOS using sysctl and system_profiler.
type SystemHardwareProfiler struct{}

func (p *SystemHardwareProfiler) Profile() (*HardwareProfile, error) {
	hw := &HardwareProfile{Architecture: runtime.GOARCH}

	hw.ProductName = readDarwinSysctl("hw.model")

	cpuCount := int32(runtime.NumCPU())
	hw.CPUCount = &cpuCount

	if memSize, err := unix.SysctlUint64("hw.memsize"); err == nil {
		ramBytes := int64(memSize)
		hw.RAMBytes = &ramBytes
	} else {
		log.Printf("hardware profiler: failed to read memory size: %v", err)
	}

	hw.OS = readDarwinCommand("sw_vers", "-productName")
	hw.OSVersion = readDarwinCommand("sw_vers", "-productVersion")

	hw.Storage = probeDarwinStorage()

	// No GPU/interconnect probing on macOS (NVML unavailable).

	return hw, nil
}

// readDarwinSysctl reads a string sysctl value.
func readDarwinSysctl(name string) string {
	val, err := unix.Sysctl(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// readDarwinCommand runs a command and returns trimmed stdout.
func readDarwinCommand(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output() // #nosec G204
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// probeDarwinStorage uses diskutil to discover storage devices.
func probeDarwinStorage() []StorageDevice {
	out, err := exec.Command("diskutil", "info", "-all").Output() // #nosec G204
	if err != nil {
		return nil
	}
	return parseDiskutilInfoOutput(string(out))
}

// parseDiskutilInfoOutput parses "diskutil info -all" output into StorageDevice entries.
func parseDiskutilInfoOutput(output string) []StorageDevice {
	var devices []StorageDevice
	var current *StorageDevice

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		current = parseDiskutilLine(line, current, &devices)
	}

	if current != nil && current.StorageBytes > 0 {
		devices = append(devices, *current)
	}

	return devices
}

// parseDiskutilLine processes a single line of diskutil output, updating the
// current device state. Returns the (possibly new) current device pointer.
func parseDiskutilLine(line string, current *StorageDevice, devices *[]StorageDevice) *StorageDevice {
	if val, ok := strings.CutPrefix(line, "Device Identifier:"); ok {
		return parseDiskutilDeviceID(strings.TrimSpace(val), current, devices)
	}

	if current == nil {
		return nil
	}

	if val, ok := strings.CutPrefix(line, "Disk Size:"); ok {
		parseDiskutilSize(strings.TrimSpace(val), current)
	} else if val, ok := strings.CutPrefix(line, "Solid State:"); ok {
		parseDiskutilSolidState(strings.TrimSpace(val), current)
	} else if val, ok := strings.CutPrefix(line, "Protocol:"); ok {
		if strings.Contains(strings.ToLower(strings.TrimSpace(val)), "nvme") {
			current.StorageType = "NVMe"
		}
	} else if line == "" && current.StorageBytes > 0 {
		*devices = append(*devices, *current)
		return nil
	}

	return current
}

func parseDiskutilDeviceID(val string, current *StorageDevice, devices *[]StorageDevice) *StorageDevice {
	// Flush previous device if valid.
	if current != nil && current.StorageBytes > 0 {
		*devices = append(*devices, *current)
	}
	// Only include whole disks (e.g. disk0, not disk0s1).
	if isWholeDisk(val) {
		return &StorageDevice{Name: val}
	}
	return nil
}

func isWholeDisk(name string) bool {
	if !strings.HasPrefix(name, "disk") {
		return false
	}
	suffix := name[4:]
	return !strings.ContainsAny(suffix, "s")
}

func parseDiskutilSize(val string, dev *StorageDevice) {
	// Prefer the parenthesized "(N Bytes)" form — most reliable.
	if m := diskutilBytesRe.FindStringSubmatch(val); len(m) == 2 {
		cleaned := strings.ReplaceAll(m[1], ",", "")
		if size, err := strconv.ParseInt(cleaned, 10, 64); err == nil {
			dev.StorageBytes = size
			return
		}
	}
	// Fallback: try parsing the first whitespace-delimited token as a
	// plain integer (handles "500107862016 Bytes ..." without parens).
	parts := strings.Fields(val)
	if len(parts) >= 1 {
		if size, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			dev.StorageBytes = size
		}
	}
	// If neither matches, leave StorageBytes as 0 (unknown).
}

func parseDiskutilSolidState(val string, dev *StorageDevice) {
	// Don't overwrite a more specific type (e.g. NVMe from Protocol line).
	if dev.StorageType != "" {
		return
	}
	if strings.EqualFold(val, "yes") {
		dev.StorageType = "SSD"
	} else {
		dev.StorageType = "HDD"
	}
}
