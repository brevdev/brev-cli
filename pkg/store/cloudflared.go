package store

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/brevdev/brev-cli/pkg/collections"
	"github.com/brevdev/brev-cli/pkg/errors"
)

type CloudflaredStore interface {
	GetBrevCloudflaredBinaryPath() (string, error)
	FileExists(string) (bool, error)
	DownloadBinary(string, string) error
	Chmod(string, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	Create(string) (io.WriteCloser, error)
}

type Cloudflared struct {
	store CloudflaredStore
}

func NewCloudflare(store CloudflaredStore) Cloudflared {
	return Cloudflared{
		store: store,
	}
}

var CloudflaredVersion = "2024.10.0"

func (c Cloudflared) DownloadCloudflaredBinaryIfItDNE() error {
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
	err = c.DownloadBinary(context.TODO(), binaryPath, binaryURL)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	err = c.store.Chmod(binaryPath, 0o755)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func (c Cloudflared) DownloadBinary(ctx context.Context, binaryPath, binaryURL string) error {
	resp, err := collections.GetRequestWithContext(ctx, binaryURL)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	defer resp.Body.Close() //nolint:errcheck // defer

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	err = c.store.MkdirAll(filepath.Dir(binaryPath), 0o755)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	out, err := c.store.Create(binaryPath)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	defer out.Close() //nolint:errcheck // defer

	var src io.Reader
	if strings.HasSuffix(binaryURL, ".tgz") {
		src = trytoUnTarGZ(resp.Body)
	} else {
		src = resp.Body
	}

	_, err = io.Copy(out, src)
	if err != nil {
		return fmt.Errorf("error saving downloaded file: %v", err)
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
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-amd64", CloudflaredVersion), nil
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-arm64.tgz", CloudflaredVersion), nil
		}
		return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-amd64.tgz", CloudflaredVersion), nil
	default:
		return "", fmt.Errorf("unsupported OS %s for downloading Cloudflared binary", runtime.GOOS)
	}
}
