//go:build !linux && !darwin

package register

import "runtime"

// SystemHardwareProfiler is a no-op adapter for unsupported platforms.
type SystemHardwareProfiler struct{}

func (p *SystemHardwareProfiler) Profile() (*HardwareProfile, error) {
	return &HardwareProfile{Architecture: runtime.GOARCH}, nil
}
