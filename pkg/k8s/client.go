package k8s

import (
	"github.com/brevdev/brev-cli/pkg/entity"
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

type DefaultWorkspaceGroupClientMapper struct {
	workspaceK8sClientMap map[string]K8sClient
	workspaceK8sAPIURLMap map[string]string
	privateKey            string
}

type K8sStore interface {
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

type RemoteK8sClientConfig struct {
	host string
	cert []byte
	key  []byte
	ca   []byte
}
