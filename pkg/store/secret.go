package store

import breverrors "github.com/brevdev/brev-cli/pkg/errors"

type CreateSecretRequest struct {
	Name          string        `json:"name"`
	HierarchyType HierarchyType `json:"HierarchyType"`
	HierarchyId   string        `json:"hierarchyId"`
	Src           SecretReqSrc  `json:"src"`
	Dest          SecretReqDest `json:"dest"`
}

type HierarchyType string

const (
	Org  HierarchyType = "org"
	User HierarchyType = "user"
)

type SecretReqSrc struct {
	Type   SrcType   `json:"type"`
	Config SrcConfig `json:"config"`
}

type SecretReqDest struct {
	Type   DestType   `json:"type"`
	Config DestConfig `json:"config"`
}

type SrcConfig struct {
	Name string `json:"name"`
}

type DestConfig struct {
	Value string `json:"value"`
}

type DestType string

const (
	KeyValue DestType = "kv2"
)

type SrcType string

const (
	File        SrcType = "file"
	EnvVariable SrcType = "env"
)

var secretsPath = "api/secrets"

func (s AuthHTTPStore) CreateSecret(req CreateSecretRequest) (*CreateSecretRequest, error) {
	var result CreateSecretRequest
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(req).
		Post(secretsPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
