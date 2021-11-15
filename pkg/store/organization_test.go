package store

import "github.com/brevdev/brev-cli/pkg/brev_api"

func (s FileStore) GetActiveOrganization() (*brev_api.Organization, error)
