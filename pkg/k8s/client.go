package k8s

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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

func NewDefaultClient(config K8sClientConfig) (K8sClient, error) {
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
		return nil, breverrors.WrapAndTrace(err)
	}

	return &DefaultClient{
		k8sClientset:  k8sClient,
		k8sRestConfig: restConfig,
	}, nil
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

type K8sStore interface {
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func NewDefaultWorkspaceGroupClientMapper(k8sStore K8sStore) (*DefaultWorkspaceGroupClientMapper, error) {
	keys, err := k8sStore.GetCurrentUserKeys()
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
		wkc[wk.GroupID], err = NewDefaultClient(rcc)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
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
