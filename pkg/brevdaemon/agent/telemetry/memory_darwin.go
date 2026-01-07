//go:build darwin

package telemetry

import (
	"math"

	"github.com/brevdev/dev-plane/pkg/errors"
	"golang.org/x/sys/unix"
)

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
