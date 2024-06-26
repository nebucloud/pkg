package meter

import (
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
)

func GetMeter() metric.Meter {
	return otel.Meter("k8sxds")
}

func InstallPromExporter() error {
	promReader, err := otelprom.New()
	if err != nil {
		return err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(promReader))
	otel.SetMeterProvider(provider)
	return nil
}

var MeterModule = fx.Options(
	fx.Provide(GetMeter),
	fx.Invoke(func() error {
		return InstallPromExporter()
	}),
)
