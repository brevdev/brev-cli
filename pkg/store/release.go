package store

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/go-resty/resty/v2"
)

type GithubReleaseMetadata struct {
	TagName      string `json:"tag_name"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
	Name         string `json:"name"`
	Body         string `json:"body"`
}

const (
	cliReleaseURL      = "https://api.github.com/repos/brevdev/brev-cli/releases/latest"
	ghAuthTokenTimeout = 60 * time.Second
)

// GitHubAPIToken returns GITHUB_TOKEN from the environment, or "gh auth token" with a timeout.
func GitHubAPIToken() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}

	ctx, cancel := context.WithTimeout(context.Background(), ghAuthTokenTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output() //nolint:gosec // intentional gh probe
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (n NoAuthHTTPStore) GetLatestReleaseMetadata() (*GithubReleaseMetadata, error) {
	var result GithubReleaseMetadata

	client := resty.New()
	req := client.R().SetResult(&result)
	if token := GitHubAPIToken(); token != "" {
		req.SetHeader("Authorization", "token "+token)
	}

	res, err := req.Get(cliReleaseURL)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
