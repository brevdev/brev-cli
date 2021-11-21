package store

import breverrors "github.com/brevdev/brev-cli/pkg/errors"

type CreateSecretRequest struct {
	Name          string        `json:"name"`
	HierarchyType HierarchyType `json:"hierarchyType"`
	HierarchyId   string        `json:"hierarchyId"`
	Src           SecretReqSrc  `json:"src"`
	Dest          SecretReqDest `json:"dest"`
}

type HierarchyType string

const (
	Org  HierarchyType = "org"
	User HierarchyType = "user"
)

type SecretReqDest struct {
	Type   DestType   `json:"type"`
	Config DestConfig `json:"config"`
}

type DestConfig struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

type SecretReqSrc struct {
	Type   SrcType   `json:"type"`
	Config SrcConfig `json:"config"`
}

type SrcConfig struct {
	Value string `json:"value"`
}

type SrcType string

const (
	KeyValue SrcType = "kv2"
)

type DestType string

const (
	File        DestType = "file"
	EnvVariable DestType = "env"
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
