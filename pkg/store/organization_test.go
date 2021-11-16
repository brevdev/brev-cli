package store

import (
	"testing"

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
