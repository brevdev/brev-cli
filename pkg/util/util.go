package util

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/hashicorp/go-multierror"
)

// This package should only be used as a holding pattern to be later moved into more specific packages

func MapAppend(m map[string]interface{}, n ...map[string]interface{}) map[string]interface{} {
	if m == nil { // we may get nil maps from legacy users not having user.OnboardingStatus set
		m = make(map[string]interface{})
	}
	for _, item := range n {
		for key, value := range item {
			m[key] = value
		}
	}
	return m
}

// checks if noun or pulural version of noun (checks if s at end)
func IsSingularOrPlural(check, noun string) bool {
	// TODO complex logic
	return check == noun || fmt.Sprintf("%ss", noun) == check
}

func DecodeBase64OrReturnSelf(maybeBase64 string) []byte {
	res, err := base64.StdEncoding.DecodeString(maybeBase64)
	if err != nil {
		fmt.Println("could not decode base64 assuming regular string")
		return []byte(maybeBase64)
	}
	return res
}

func RemoveFileExtenstion(path string) string {
	return strings.TrimRight(path, filepath.Ext(path))
}

type RunEResult struct {
	errChan chan error
	num     int
}

func (r RunEResult) Await() error {
	var allErr error
	for i := 0; i < r.num; i++ {
		err := <-r.errChan
		if err != nil {
			allErr = multierror.Append(err)
		}
	}
	if allErr != nil {
		return errors.WrapAndTrace(allErr)
	}
	return nil
}

func RunEAsync(calls ...func() error) RunEResult {
	res := RunEResult{make(chan error), len(calls)}
	for _, c := range calls {
		go func(cl func() error) {
			err := cl()
			res.errChan <- err
		}(c)
	}
	return res
}

func IsGitURL(u string) bool {
	return strings.Contains(u, "https://") || strings.Contains(u, "git@")
}

func DoesPathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
