package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient is a powerful and versatile Kubernetes client that allows
// you to interact with your Kubernetes cluster effortlessly.
type K8sClient struct {
	RestConfig        *rest.Config          `json:"restconfig,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamic_kubeclient,omitempty"`
}

// IK8sProvider is an interface that defines the contract for initializing
// Kubernetes clients, providing flexibility and extensibility.
type IK8sProvider interface {
	InClusterClient() (*K8sClient, error)
	OutOfClusterClient(kubeconfigPath string) (*K8sClient, error)
}

// k8sProvider is a concrete implementation of the IK8sProvider interface,
// encapsulating the logic for creating Kubernetes clients.
type k8sProvider struct{}

// NewK8sProvider is a factory function that creates a new instance of IK8sProvider,
// abstracting the creation process and providing a convenient way to obtain a provider.
func NewK8sProvider() IK8sProvider {
	return &k8sProvider{}
}

// InClusterClient is a method of the k8sProvider that initializes a Kubernetes client
// using the in-cluster configuration, allowing seamless communication within the cluster.
func (k *k8sProvider) InClusterClient() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return newClient(config)
}

// OutOfClusterClient is a method of the k8sProvider that initializes a Kubernetes client
// using the provided kubeconfig file path, enabling communication from outside the cluster.
func (k *k8sProvider) OutOfClusterClient(kubeconfigPath string) (*K8sClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return newClient(config)
}

// newClient is an internal function that creates a new K8sClient instance
// using the provided REST config, encapsulating the creation logic.
func newClient(config *rest.Config) (*K8sClient, error) {
	config.QPS = float32(50)
	config.Burst = int(100)

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dyclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		RestConfig:        config,
		DynamicKubeClient: dyclient,
		KubeClient:        kclient,
	}, nil
}
