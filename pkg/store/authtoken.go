package store

import "github.com/brevdev/brev-cli/pkg/entity"

func (f FileStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	return nil
}

func (f FileStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (f FileStore) DeleteAuthTokens() error {
	return nil
}
