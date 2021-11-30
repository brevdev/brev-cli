package store

import (
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
	cliReleaseURL = "https://api.github.com/repos/brevdev/brev-cli/releases/latest"
)

func (n NoAuthHTTPStore) GetLatestReleaseMetadata() (*GithubReleaseMetadata, error) {
	var result GithubReleaseMetadata

	client := resty.New()

	res, err := client.R().SetBody(&result).Get(cliReleaseURL)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
