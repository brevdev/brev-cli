package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sClient interface {
	GetK8sClient() *kubernetes.Clientset
	GetK8sRestConfig() *rest.Config
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
