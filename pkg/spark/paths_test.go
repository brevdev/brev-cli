package spark

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncSSHConfigLocatorPaths(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		home     string
		expected string
	}{
		{
			name:     "darwin",
			goos:     "darwin",
			home:     "/Users/tester",
			expected: filepath.Join("/Users/tester", "Library", "Application Support", "NVIDIA", "Sync", "config", "ssh_config"),
		},
		{
			name:     "linux",
			goos:     "linux",
			home:     "/home/tester",
			expected: filepath.Join("/home/tester", ".config", "NVIDIA", "Sync", "config", "ssh_config"),
		},
		{
			name:     "windows",
			goos:     "windows",
			home:     filepath.FromSlash("C:/Users/tester"),
			expected: filepath.Join(filepath.FromSlash("C:/Users/tester"), "AppData", "Local", "NVIDIA Corporation", "Sync", "config", "ssh_config"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			locator := NewSyncSSHConfigLocatorWithHome(func() (string, error) { return tt.home, nil }, tt.goos)
			path, err := locator.ConfigPath()
			require.NoError(t, err)
			require.Equal(t, tt.expected, path)
		})
	}
}

func TestSyncSSHConfigLocatorUnsupported(t *testing.T) {
	locator := NewSyncSSHConfigLocatorWithHome(func() (string, error) { return "/home/tester", nil }, "plan9")
	_, err := locator.ConfigPath()
	require.Error(t, err)
}
