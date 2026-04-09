//go:build (linux || windows) && !nvml

package register

// probeGPUsNVML is optional because linking NVML at startup can fail on hosts
// with older NVIDIA driver libraries even for commands that do not use GPU
// probing. Enable the real implementation with `-tags nvml`.
func probeGPUsNVML() ([]GPU, []Interconnect) {
	return nil, nil
}
