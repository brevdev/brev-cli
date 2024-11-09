// Package cloudflared provides functions to download and manage the Cloudflared binary.
package cloudflared

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const CloudflaredVersion = "2024.10.0"

var binaryPath string = filepath.Join(os.Getenv("HOME"), ".brev", "cloudflared")

// DownloadCloudflaredBinaryIfItDNE downloads the Cloudflared binary if it does not exist.
// This function does not check the version of the binary.
func DownloadCloudflaredBinaryIfItDNE(ctx context.Context) error {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		if err := downloadBinary(ctx, binaryPath); err != nil {
			return fmt.Errorf("cloudflared: %w", err)
		}
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

func downloadBinary(ctx context.Context, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	downloadURL, err := getCloudflaredBinaryDownloadURL()
	if err != nil {
		return err
	}

	// Validate URL
	if _, err := url.Parse(downloadURL); err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}

	return downloadAndExtractFile(ctx, downloadURL, destPath)
}

func downloadAndExtractFile(ctx context.Context, downloadURL, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading cloudflared: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("closing response body: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body

	// Handle compressed files
	if strings.HasSuffix(downloadURL, ".tgz") || strings.HasSuffix(downloadURL, ".gz") {
		gzReader, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return fmt.Errorf("creating gzip reader: %w", gzErr)
		}
		defer func() {
			if err := gzReader.Close(); err != nil {
				fmt.Printf("error closing gzip reader: %v\n", err)
			}
		}()

		if strings.HasSuffix(downloadURL, ".tgz") {
			tarReader := tar.NewReader(gzReader)
			header, tarErr := tarReader.Next()
			if tarErr != nil {
				return fmt.Errorf("reading tar header: %w", tarErr)
			}
			if header.Typeflag != tar.TypeReg {
				return fmt.Errorf("unexpected file type in archive")
			}
			reader = tarReader
		} else {
			reader = gzReader
		}
	}

	//nolint:gosec // destPath is validated by the caller
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("creating binary file: %w", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			fmt.Printf("error closing output file: %v\n", err)
		}
	}()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("writing binary: %w", err)
	}

	return nil
}
