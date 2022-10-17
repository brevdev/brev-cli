package store

import (
	"encoding/json"
	"net/http"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type instanceIdentityDocument struct {
	InstanceType string `json:"instanceType"`
}

func (n *NoAuthHTTPStore) GetInstanceType() (string, error) {
	var iid instanceIdentityDocument
	resp, err := http.Get(" http://169.254.169.254/latest/dynamic/instance-identity/document")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&iid)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return iid.InstanceType, nil
}
