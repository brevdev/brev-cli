package register

import "os"

// readOSFile reads a file from the real filesystem.
func readOSFile(path string) ([]byte, error) {
	return os.ReadFile(path) // #nosec G304
}
