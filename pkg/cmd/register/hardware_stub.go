//go:build !linux && !darwin && !windows

package register

// SystemHardwareProfiler is a no-op adapter for unsupported platforms.
type SystemHardwareProfiler struct{}

func (p *SystemHardwareProfiler) Profile() (*HardwareProfile, error) {
	return &HardwareProfile{}, nil
}
