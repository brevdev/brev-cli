//go:build linux

package register

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	errors "github.com/brevdev/brev-cli/pkg/errors"
)

// SystemHardwareProfiler probes hardware on Linux using NVML, sysfs, and procfs.
type SystemHardwareProfiler struct{}

func (p *SystemHardwareProfiler) Profile() (*HardwareProfile, error) {
	hw := &HardwareProfile{Architecture: runtime.GOARCH}

	hw.GPUs, hw.Interconnects = probeGPUsNVML()
	hw.ProductName = readProductName()

	if cpuCount, err := readCPUCount(); err == nil {
		count32 := int32(cpuCount)
		hw.CPUCount = &count32
	}

	if ramBytes, err := readRAMBytes(); err == nil {
		hw.RAMBytes = &ramBytes
	}

	hw.OS, hw.OSVersion = readOSRelease()
	hw.Storage = probeStorageSysfs()

	return hw, nil
}

// readProductName reads the product name from DMI sysfs.
func readProductName() string {
	data, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_name")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readCPUCount reads /proc/cpuinfo and returns the number of logical processors.
func readCPUCount() (int, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, errors.WrapAndTrace(err)
	}
	return parseCPUCountContent(string(data))
}

// readRAMBytes returns total system RAM in bytes using sysinfo syscall.
func readRAMBytes() (int64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, errors.WrapAndTrace(err)
	}
	return int64(info.Totalram) * int64(info.Unit), nil
}

// readOSRelease reads /etc/os-release and returns (name, version).
func readOSRelease() (string, string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", ""
	}
	return parseOSReleaseContent(string(data))
}

// probeStorageSysfs enumerates block devices via /sys/block and returns
// storage information. This replaces the lsblk-based approach.
func probeStorageSysfs() []StorageDevice {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}

	var devices []StorageDevice
	for _, entry := range entries {
		name := entry.Name()

		// Skip virtual devices (loop, ram, dm-, etc.)
		if strings.HasPrefix(name, "loop") ||
			strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") ||
			strings.HasPrefix(name, "sr") ||
			strings.HasPrefix(name, "zram") {
			continue
		}

		sizeBytes, err := readSysfsInt(filepath.Join("/sys/block", name, "size"))
		if err != nil || sizeBytes == 0 {
			continue
		}
		// /sys/block/*/size is in 512-byte sectors.
		sizeBytes *= 512

		dev := StorageDevice{
			Name:         name,
			StorageBytes: sizeBytes,
		}

		rotational, err := readSysfsInt(filepath.Join("/sys/block", name, "queue", "rotational"))
		if err == nil {
			if rotational == 0 {
				dev.StorageType = "SSD"
			} else {
				dev.StorageType = "HDD"
			}
		}

		// Detect NVMe specifically.
		if strings.HasPrefix(name, "nvme") {
			dev.StorageType = "NVMe"
		}

		devices = append(devices, dev)
	}
	return devices
}

// readSysfsInt reads a single integer from a sysfs file.
func readSysfsInt(path string) (int64, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return 0, errors.WrapAndTrace(fmt.Errorf("read %s: %w", path, err))
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, errors.WrapAndTrace(err)
	}
	return n, nil
}
