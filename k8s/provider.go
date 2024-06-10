package k8s

import (
	"net/http"

	"github.com/nebucloud/pkg/logger"
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
	InClusterClient(logger *logger.Klogger) (*K8sClient, error)
	OutOfClusterClient(kubeconfigPath string, logger *logger.Klogger) (*K8sClient, error)
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
func (k *k8sProvider) InClusterClient(logger *logger.Klogger) (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Error("Failed to create in-cluster Kubernetes config", err)
		return nil, err
	}
	return newClient(config, logger)
}

// OutOfClusterClient is a method of the k8sProvider that initializes a Kubernetes client
// using the provided kubeconfig file path, enabling communication from outside the cluster.
func (k *k8sProvider) OutOfClusterClient(kubeconfigPath string, logger *logger.Klogger) (*K8sClient, error) {
	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		logger.Error("Failed to create out-of-cluster Kubernetes config", err)
		return nil, err
	}
	return newClient(clientConfig, logger)
}

// NewClientFromConfig creates a new Kubernetes client using the provided configuration.
func NewClientFromConfig(logger *logger.Klogger) (*K8sClient, *http.Client, error) {
	httpClient, clientConfig, err := NewHttpClientWithConfig(logger)
	if err != nil {
		return nil, nil, err
	}

	kubeClient, err := kubernetes.NewForConfigAndClient(clientConfig, httpClient)
	if err != nil {
		logger.Error("Failed to create Kubernetes client", err)
		return nil, nil, err
	}

	dyClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		logger.Error("Failed to create dynamic Kubernetes client", err)
		return nil, nil, err
	}

	logger.Info("Kubernetes client created successfully")
	return &K8sClient{
		RestConfig:        clientConfig,
		KubeClient:        kubeClient,
		DynamicKubeClient: dyClient,
	}, httpClient, nil
}

// newClient is an internal function that creates a new K8sClient instance
// using the provided REST config, encapsulating the creation logic.
func newClient(config *rest.Config, logger *logger.Klogger) (*K8sClient, error) {
	config.QPS = float32(50)
	config.Burst = int(100)

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("Failed to create Kubernetes clientset", err)
		return nil, err
	}

	dyclient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("Failed to create dynamic clientset", err)
		return nil, err
	}

	logger.Info("Kubernetes clients created successfully")
	return &K8sClient{
		RestConfig:        config,
		DynamicKubeClient: dyclient,
		KubeClient:        kclient,
	}, nil
}
