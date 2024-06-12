package k8s

import (
	"net/http"

	"github.com/nebucloud/pkg/logger"
	"github.com/samber/oops"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewHttpClientWithConfig(logger *logger.Klogger) (*http.Client, *rest.Config, error) {
	logger.Info("Loading Kubernetes client configuration...")
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		return nil, nil, oops.
			In("k8s").
			With("function", "NewHttpClientWithConfig").
			With("operation", "load_client_config").
			Wrapf(err, "failed to load Kubernetes client configuration")
	}

	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, nil, oops.
			In("k8s").
			With("function", "NewHttpClientWithConfig").
			With("operation", "create_transport").
			Wrapf(err, "failed to create HTTP transport for Kubernetes client")
	}

	httpTransport, ok := transport.(*http.Transport)
	if !ok {
		return nil, nil, oops.
			In("k8s").
			With("function", "NewHttpClientWithConfig").
			With("operation", "assert_transport_type").
			Errorf("unexpected transport type: %T", transport)
	}

	logger.Info("Kubernetes client configuration loaded successfully")
	return &http.Client{
		Transport: otelhttp.NewTransport(httpTransport),
		Timeout:   config.Timeout,
	}, config, nil
}
