//go:build !darwin

package telemetry

import "github.com/brevdev/dev-plane/pkg/errors"

func darwinMemoryBytes() (int64, error) {
	return 0, errors.New("darwin memory lookup unsupported on this platform")
}
