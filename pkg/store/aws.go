package store

import (
	"encoding/json"
	"net/http"
)

type instanceIdentityDocument struct {
	InstanceType string `json:"instanceType"`
}

func (n *NoAuthHTTPStore) GetInstanceType() (string, error) {
	var iid instanceIdentityDocument
	resp, err := http.Get(" http://169.254.169.254/latest/dynamic/instance-identity/document")
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&iid)
	if err != nil {
		return "", err
	}
	return iid.InstanceType, nil
}
