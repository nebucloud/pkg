package k8s

import (
	"fmt"
	"net/http"

	"github.com/nebucloud/pkg/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewHttpClientWithConfig(logger *logger.Klogger) (*http.Client, *rest.Config, error) {
	logger.Info("Loading Kubernetes client configuration...")

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		logger.Error("Failed to load Kubernetes client configuration", err)
		return nil, nil, err
	}

	transport, err := rest.TransportFor(config)
	if err != nil {
		logger.Error("Failed to create HTTP transport for Kubernetes client", err)
		return nil, nil, err
	}

	httpTransport, ok := transport.(*http.Transport)
	if !ok {
		err := fmt.Errorf("unexpected transport type: %T", transport)
		logger.Error(err.Error())
		return nil, nil, err
	}

	logger.Info("Kubernetes client configuration loaded successfully")
	return &http.Client{
		Transport: otelhttp.NewTransport(httpTransport),
		Timeout:   config.Timeout,
	}, config, nil
}
