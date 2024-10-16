package store

import (
	"fmt"
	"io/fs"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/errors"
)

type CloudflareStore interface {
	GetBrevCloudflaredBinaryPath() (string, error)
	FileExists(string) (bool, error)
	DownloadBinary(string, string) error
	Chmod(string, fs.FileMode) error
}

type Cloudflare struct {
	store CloudflareStore
}

func NewCloudflare(store CloudflareStore) Cloudflare {
	return Cloudflare{
		store: store,
	}
}

var CloudflaredVersion = "2024.10.0"

func (c Cloudflare) DownloadCloudflaredBinaryIfItDNE() error {
	binaryPath, err := c.store.GetBrevCloudflaredBinaryPath()
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	binaryExists, err := c.store.FileExists(binaryPath)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	if binaryExists {
		return nil
	}
	binaryURL, err := getCloudflaredBinaryDownloadURL()
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	err = c.store.DownloadBinary(binaryURL, binaryPath)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	err = c.store.Chmod(binaryPath, 0o755)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func getCloudflaredBinaryDownloadURL() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-fips-linux-amd64", CloudflaredVersion), nil
	case "windows":
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-amd64.exe", CloudflaredVersion), nil
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-arm64.tgz", CloudflaredVersion), nil
		}
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-amd64.tgz", CloudflaredVersion), nil
	default:
		return "", fmt.Errorf("unsupported OS %s for downloading Cloudflare binary", runtime.GOOS)
	}
}
