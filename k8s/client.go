package k8s

import (
	"os"
	"path/filepath"

	"github.com/nebucloud/pkg/models"
	"github.com/pkg/errors"
	"github.com/samber/oops"
	"gopkg.in/yaml.v2"
)

// getKubeconfigPath is a helper function that retrieves the path to the kubeconfig file,
// either from the KUBECONFIG environment variable or the default location.
func getKubeconfigPath() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	return kubeconfig
}

// readKubeconfigFile is a utility function that reads the contents of the kubeconfig file
// from the specified path, handling any errors that may occur during the process.
func readKubeconfigFile(path string) ([]byte, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not read kubeconfig")
	}
	return file, nil
}

// GetKubeConfig is a method of the K8sClient that retrieves the kubeconfig
// from the specified path or the default location, providing easy access to the configuration.
func (c *K8sClient) GetKubeConfig() (*models.Kubeconfig, error) {
	kubeconfig := getKubeconfigPath()
	file, err := readKubeconfigFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	var config models.Kubeconfig
	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, oops.Wrapf(err, "failed to unmarshal kubeconfig")
	}
	return &config, nil
}

// GetCurrentContext is a method of the K8sClient that retrieves the current context
// from the kubeconfig, allowing you to determine the active context.
func (c *K8sClient) GetCurrentContext() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

// GetCurrentNamespace is a method of the K8sClient that retrieves the current namespace
// from the kubeconfig, providing information about the active namespace.
func (c *K8sClient) GetCurrentNamespace() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}
	currentContext := config.CurrentContext
	for _, context := range config.Contexts {
		if context.Name == currentContext {
			if context.Context.Namespace == "" {
				return "default", nil
			}
			return context.Context.Namespace, nil
		}
	}
	return "", oops.Errorf("current context not found in kubeconfig")
}

// GetCurrentCluster is a method of the K8sClient that retrieves the current cluster
// from the kubeconfig, allowing you to identify the active cluster.
func (c *K8sClient) GetCurrentCluster() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}
	currentContext := config.CurrentContext
	for _, context := range config.Contexts {
		if context.Name == currentContext {
			return context.Context.Cluster, nil
		}
	}
	return "", oops.Errorf("current context not found in kubeconfig")
}

// GetCurrentUser is a method of the K8sClient that retrieves the current user
// from the kubeconfig, providing information about the active user.
func (c *K8sClient) GetCurrentUser() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}
	currentContext := config.CurrentContext
	for _, context := range config.Contexts {
		if context.Name == currentContext {
			return context.Context.User, nil
		}
	}
	return "", oops.Errorf("current context not found in kubeconfig")
}
