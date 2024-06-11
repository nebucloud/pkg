package k8s

import (
	"reflect"

	"go.uber.org/fx"
	"go.uber.org/multierr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Option is a functional option for tailoring the Kubernetes client
// configuration prior to creating it. Each option can modify the
// *rest.Config prior to it being passed to the client.
type Option func(*rest.Config) error

var (
	optionType        = reflect.TypeOf(Option(nil))
	noErrorOptionType = reflect.TypeOf((func(*rest.Config))(nil))
)

// OptionFunc represents the types of functions that can be coerced into Options.
type OptionFunc interface {
	~func(*rest.Config) error | ~func(*rest.Config)
}

// AsOption coerces a function into an Option.
func AsOption[OF OptionFunc](of OF) Option {
	// trivial conversions
	switch oft := any(of).(type) {
	case Option:
		return oft

	case func(*rest.Config):
		return func(cfg *rest.Config) error {
			oft(cfg)
			return nil
		}
	}

	// now we convert to the underlying type
	ofv := reflect.ValueOf(of)
	if ofv.CanConvert(optionType) {
		return ofv.Convert(optionType).Interface().(Option)
	}

	// there are only (2) types, so the other type must be it
	f := ofv.Convert(noErrorOptionType).Interface().(func(*rest.Config))
	return func(cfg *rest.Config) error {
		f(cfg)
		return nil
	}
}

// WithQPS configures the Kubernetes client with a custom QPS value.
func WithQPS(qps float32) Option {
	return func(cfg *rest.Config) error {
		cfg.QPS = qps
		return nil
	}
}

// WithBurst configures the Kubernetes client with a custom Burst value.
func WithBurst(burst int) Option {
	return func(cfg *rest.Config) error {
		cfg.Burst = burst
		return nil
	}
}

// Decorate is an uber/fx decorator that returns a new Kubernetes client Config
// that results from applying any number of options to an existing Config.
// If no options are supplied, this function returns a clone of the original.
func Decorate(original *rest.Config, opts ...Option) (*rest.Config, error) {
	cfg := rest.CopyConfig(original)
	var err error
	for _, o := range opts {
		err = multierr.Append(err, o(cfg))
	}

	return cfg, err
}

// NewClientFromConfig is the standard constructor for a Kubernetes client.
// It allows for any number of options to tailor the configuration.
func NewClientFromConfig(cfg *rest.Config, opts ...Option) (*K8sClient, error) {
	cfg, err := Decorate(cfg, opts...)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		RestConfig:        cfg,
		KubeClient:        kubeClient,
		DynamicKubeClient: dynamicClient,
	}, nil
}

// Provide gives a very simple, opinionated way of using NewClientFromConfig within an fx.App.
// It assumes a global, unnamed *rest.Config optional dependency and zero or more Option
// in a value group named 'k8s.options'.
func Provide(external ...Option) fx.Option {
	ctor := NewClientFromConfig
	if len(external) > 0 {
		ctor = func(cfg *rest.Config, injected ...Option) (*K8sClient, error) {
			return NewClientFromConfig(cfg, append(injected, external...)...)
		}
	}

	return fx.Provide(
		fx.Annotate(
			ctor,
			fx.ParamTags(`optional:"true"`, `group:"k8s.options"`),
		),
	)
}
