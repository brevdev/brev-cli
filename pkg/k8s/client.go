// k8s.
package k8s

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sClient interface {
	GetK8sClient() *kubernetes.Clientset
	GetK8sRestConfig() *rest.Config
}

type WorkspaceGroupClientMapper interface {
	GetK8sClient(workspaceGroupID string) (K8sClient, error)
	GetK8sAPIURL(workspaceGroupID string) (string, error)
}

type DefaultClient struct {
	k8sClientset  *kubernetes.Clientset
	k8sRestConfig *rest.Config
}

type K8sClientConfig interface {
	GetHost() string
	GetCert() []byte
	GetKey() []byte
	GetCA() []byte
}

func NewDefaultClient(config K8sClientConfig) K8sClient {
	restConfig := dynamic.ConfigFor(&rest.Config{
		Host:    config.GetHost(),
		APIPath: "/api",
		TLSClientConfig: rest.TLSClientConfig{
			CertData: config.GetCert(),
			KeyData:  config.GetKey(),
			CAData:   config.GetCA(),
		},
	})

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}

	return &DefaultClient{
		k8sClientset:  k8sClient,
		k8sRestConfig: restConfig,
	}
}

func (c DefaultClient) GetK8sClient() *kubernetes.Clientset {
	return c.k8sClientset
}

func (c DefaultClient) GetK8sRestConfig() *rest.Config {
	return c.k8sRestConfig
}

type DefaultWorkspaceGroupClientMapper struct {
	workspaceK8sClientMap map[string]K8sClient
	workspaceK8sAPIURLMap map[string]string
	privateKey            string
}

type KeyResolver interface {
	GetMeKeys() (*brevapi.UserKeys, error)
}

func NewDefaultWorkspaceGroupClientMapper(keyResolver KeyResolver) (*DefaultWorkspaceGroupClientMapper, error) {
	keys, err := keyResolver.GetMeKeys()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	wkc := make(map[string]K8sClient)
	wka := make(map[string]string)
	for _, wk := range keys.WorkspaceGroups {
		rcc := RemoteK8sClientConfig{
			host: wk.APIURL,
			cert: []byte(wk.Cert),
			key:  []byte(keys.PrivateKey),
			ca:   []byte(wk.CA),
		}
		wkc[wk.GroupID] = NewDefaultClient(rcc)
		wka[wk.GroupID] = wk.APIURL
	}

	return &DefaultWorkspaceGroupClientMapper{workspaceK8sClientMap: wkc, workspaceK8sAPIURLMap: wka, privateKey: keys.PrivateKey}, nil
}

func (d DefaultWorkspaceGroupClientMapper) GetK8sClient(workspaceGroupID string) (K8sClient, error) {
	client, doesExists := d.workspaceK8sClientMap[workspaceGroupID]
	if !doesExists {
		return nil, fmt.Errorf("client for workspace group does not exist [workspace group id=%s]", workspaceGroupID)
	}
	return client, nil
}

func (d DefaultWorkspaceGroupClientMapper) GetK8sAPIURL(workspaceGroupID string) (string, error) {
	url, doesExists := d.workspaceK8sAPIURLMap[workspaceGroupID]
	if !doesExists {
		return "", fmt.Errorf("k8s api url for workspace group does not exist [workspace group id=%s]", workspaceGroupID)
	}
	return url, nil
}

func (d DefaultWorkspaceGroupClientMapper) GetPrivateKey() string {
	return d.privateKey
}

type RemoteK8sClientConfig struct {
	host string
	cert []byte
	key  []byte
	ca   []byte
}

func NewRemoteK8sClientConfig(clusterID string) (*RemoteK8sClientConfig, error) {
	c, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	keys, err := c.GetMeKeys()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cluserKeys, err := keys.GetWorkspaceGroupKeysByGroupID(clusterID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &RemoteK8sClientConfig{
		host: cluserKeys.APIURL,
		cert: []byte(cluserKeys.Cert),
		key:  []byte(keys.PrivateKey),
		ca:   []byte(cluserKeys.CA),
	}, nil
}

func (k RemoteK8sClientConfig) GetHost() string {
	return k.host
}

func (k RemoteK8sClientConfig) GetCert() []byte {
	return k.cert
}

func (k RemoteK8sClientConfig) GetKey() []byte {
	return k.key
}

func (k RemoteK8sClientConfig) GetCA() []byte {
	return k.ca
}
