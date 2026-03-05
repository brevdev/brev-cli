//go:build linux || windows

package register

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// archNames maps NVML compute capability (major version) to GPU architecture name.
var archNames = map[int]string{
	1:  "Tesla",
	2:  "Fermi",
	3:  "Kepler",
	5:  "Maxwell",
	6:  "Pascal",
	7:  "Volta/Turing",
	8:  "Ampere",
	9:  "Hopper/Ada Lovelace",
	10: "Blackwell",
	12: "Vera Rubin",
}

// probeGPUsNVML uses NVML to detect GPUs and interconnects.
// Returns (nil, nil) if NVML is unavailable (e.g. no driver installed).
func probeGPUsNVML() ([]GPU, []Interconnect) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, nil
	}
	defer func() { _ = nvml.Shutdown() }()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS || count == 0 {
		return nil, nil
	}

	type gpuKey struct {
		model string
		arch  string
		mem   int64
	}
	counts := make(map[gpuKey]int32)
	var order []gpuKey
	var interconnects []Interconnect

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			name = "Unknown"
		}

		var memBytes int64
		memInfo, ret := device.GetMemoryInfo()
		if ret == nvml.SUCCESS {
			memBytes = int64(memInfo.Total)
		}

		arch := ""
		major, minor, ret := device.GetCudaComputeCapability()
		if ret == nvml.SUCCESS {
			if archName, ok := archNames[major]; ok {
				arch = archName
			} else {
				arch = fmt.Sprintf("sm_%d%d", major, minor)
			}
		}

		key := gpuKey{model: name, arch: arch, mem: memBytes}
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++

		// Probe NVLink interconnects for this device.
		interconnects = append(interconnects, probeNVLink(device, i)...)

		// Probe PCIe interconnect for this device.
		if ic := probePCIe(device, i); ic != nil {
			interconnects = append(interconnects, *ic)
		}
	}

	gpus := make([]GPU, 0, len(order))
	for _, key := range order {
		mem := key.mem
		g := GPU{
			Model:        key.model,
			Architecture: key.arch,
			Count:        counts[key],
		}
		if mem > 0 {
			g.MemoryBytes = &mem
		}
		gpus = append(gpus, g)
	}

	return gpus, interconnects
}

// probeNVLink checks NVLink connections for a device.
func probeNVLink(device nvml.Device, deviceIdx int) []Interconnect {
	var ics []Interconnect
	activeLinks := 0

	// NVLink link count varies by architecture; try up to 18 links.
	var nvlinkVersion uint32
	for link := 0; link < 18; link++ {
		state, ret := device.GetNvLinkState(link)
		if ret != nvml.SUCCESS {
			break
		}
		if state == nvml.FEATURE_ENABLED {
			activeLinks++
			if nvlinkVersion == 0 {
				ver, ret := device.GetNvLinkVersion(link)
				if ret == nvml.SUCCESS {
					nvlinkVersion = ver
				}
			}
		}
	}

	if activeLinks > 0 {
		ics = append(ics, Interconnect{
			Type:        "NVLink",
			Device:      fmt.Sprintf("GPU %d", deviceIdx),
			ActiveLinks: activeLinks,
			Version:     nvlinkVersion,
		})
	}

	return ics
}

// probePCIe reads PCIe generation and width for a device.
func probePCIe(device nvml.Device, deviceIdx int) *Interconnect {
	gen, ret := device.GetCurrPcieLinkGeneration()
	if ret != nvml.SUCCESS {
		return nil
	}
	width, ret := device.GetCurrPcieLinkWidth()
	if ret != nvml.SUCCESS {
		return nil
	}
	return &Interconnect{
		Type:       "PCIe",
		Device:     fmt.Sprintf("GPU %d", deviceIdx),
		Generation: gen,
		Width:      width,
	}
}
