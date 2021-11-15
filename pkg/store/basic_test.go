package store

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewBasicStore(t *testing.T) {
	s := MakeMockBasicStore()
	if !assert.NotNil(t, s) {
		return
	}
}

func MakeMockBasicStore() *BasicStore {
	return NewBasicStore(*config.NewConstants())
}
