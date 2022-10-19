package store

import (
	"encoding/json"
	"net/http"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// $ curl --silent http://169.254.169.254/latest/dynamic/instance-identity/document
// {
//   "privateIp" : "172.31.2.15",
//   "instanceId" : "i-12341ee8",
//   "billingProducts" : null,
//   "instanceType" : "t2.small",
//   "accountId" : "1234567890",
//   "pendingTime" : "2015-11-03T03:09:54Z",
//   "imageId" : "ami-383c1956",
//   "kernelId" : null,
//   "ramdiskId" : null,
//   "architecture" : "x86_64",
//   "region" : "ap-northeast-1", # <- region
//   "version" : "2010-08-31",
//   "availabilityZone" : "ap-northeast-1c",
//   "devpayProductCodes" : null
// }

type InstanceIdentityDocument struct {
	InstanceType string `json:"instanceType"`
	Region       string `json:"region"`
}

const instanceIdentityURL = "http://169.254.169.254/latest/dynamic/instance-identity/document"

func (n *NoAuthHTTPStore) GetInstanceType() (string, error) {
	var iid InstanceIdentityDocument
	resp, err := http.Get(instanceIdentityURL)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	defer resp.Body.Close() //nolint:errcheck // defer
	err = json.NewDecoder(resp.Body).Decode(&iid)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return iid.InstanceType, nil
}

func (n *NoAuthHTTPStore) GetInstanceIdentity() (*InstanceIdentityDocument, error) {
	var iid InstanceIdentityDocument
	resp, err := http.Get(instanceIdentityURL)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	defer resp.Body.Close() //nolint:errcheck // defer
	err = json.NewDecoder(resp.Body).Decode(&iid)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &iid, nil
}
