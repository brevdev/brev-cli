package store

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/jarcoal/httpmock"

	"github.com/stretchr/testify/assert"
)

func TestGetActiveOrganization(t *testing.T) {
	fs := MakeMockAuthHTTPStore()
	org, err := fs.GetActiveOrganizationOrNil()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Nil(t, org) {
		return
	}
}

func TestGetOrganizations(t *testing.T) {
	fs := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(fs.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	expected := []brevapi.Organization{{
		ID:   "1",
		Name: "test",
	}}
	res, err := httpmock.NewJsonResponder(500, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", fs.authHTTPClient.restyClient.BaseURL, orgPath)
	httpmock.RegisterResponder("GET", url, res)

	org, err := fs.GetOrganizations()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, expected, org) {
		return
	}
}
