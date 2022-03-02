package store

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/go-resty/resty/v2"
)

// type GithubReleaseMetadata struct {
// 	TagName      string `json:"tag_name"`
// 	IsDraft      bool   `json:"draft"`
// 	IsPrerelease bool   `json:"prerelease"`
// 	Name         string `json:"name"`
// 	Body         string `json:"body"`
// }

// const (
// 	cliReleaseURL = "https://api.github.com/repos/brevdev/brev-cli/releases/latest"
// )

func (n NoAuthHTTPStore) GetSetupScriptContentsByURL(url string) (string, error) {
	var result string

	client := resty.New()

	res, err := client.R().SetResult(&result).Get(url)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return "", NewHTTPResponseError(res)
	}

	bodyAsString := string(res.Body())

	return bodyAsString, nil
}

func (s AuthHTTPStore) GetSetupScriptContentsByURL(url string) (string, error) {
	var result string

	client := resty.New()

	res, err := client.R().SetResult(&result).Get(url)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return "", NewHTTPResponseError(res)
	}

	bodyAsString := string(res.Body())

	return bodyAsString, nil
}
