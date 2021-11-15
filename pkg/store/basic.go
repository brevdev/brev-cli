package store

import "github.com/brevdev/brev-cli/pkg/config"

type BasicStore struct {
	config config.ConstantsConfig
}

func NewBasicStore(config config.ConstantsConfig) *BasicStore {
	return &BasicStore{config: config}
}
