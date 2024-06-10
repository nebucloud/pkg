package snapshot

import (
	"context"
	"sync"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/nebucloud/pkg/logger"
	"github.com/nebucloud/pkg/xds/meter"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
)

func mapTypeURL(typeURL string) string {
	switch typeURL {
	case resource.ListenerType, resource.RouteType, resource.ClusterType:
		return "services"
	case resource.EndpointType:
		return "endpoints"
	default:
		return ""
	}
}

type Snapshotter struct {
	ResyncPeriod time.Duration

	client         kubernetes.Interface
	servicesCache  cache.SnapshotCache
	endpointsCache cache.SnapshotCache
	muxCache       cache.MuxCache

	endpointResourceCache   map[string]endpointCacheItem
	resourcesByTypeLock     sync.RWMutex
	serviceResourcesByType  map[string][]types.Resource
	endpointResourcesByType map[string][]types.Resource
	apiGatewayStats         map[string]int
	kubeEventCounter        metric.Int64Counter

	logger *logger.Klogger
}

// NewSnapshotter creates a new Snapshotter instance.
func NewSnapshotter(lc fx.Lifecycle, client kubernetes.Interface, logger *logger.Klogger) *Snapshotter {
	servicesCache := cache.NewSnapshotCache(false, EmptyNodeID{}, logger)
	endpointsCache := cache.NewSnapshotCache(false, EmptyNodeID{}, logger)
	muxCache := cache.MuxCache{
		Classify: func(r *cache.Request) string {
			return mapTypeURL(r.TypeUrl)
		},
		ClassifyDelta: func(r *cache.DeltaRequest) string {
			return mapTypeURL(r.TypeUrl)
		},
		Caches: map[string]cache.Cache{
			"services":  servicesCache,
			"endpoints": endpointsCache,
		},
	}

	ss := &Snapshotter{
		ResyncPeriod: 10 * time.Minute,

		client:         client,
		servicesCache:  servicesCache,
		endpointsCache: endpointsCache,
		muxCache:       muxCache,

		endpointResourceCache: map[string]endpointCacheItem{},
		logger:                logger,
	}

	meter := meter.GetMeter()
	ss.kubeEventCounter, _ = meter.Int64Counter("xds_kube_events")
	meter.Int64ObservableGauge("xds_snapshot_resources", metric.WithInt64Callback(ss.snapshotResourceGaugeCallback))
	meter.Int64ObservableGauge("xds_apigateway_endpoints", metric.WithInt64Callback(ss.apiGatewayEndpointGaugeCallback))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return ss.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			// Add any necessary cleanup logic here
			return nil
		},
	})

	return ss
}

func (s *Snapshotter) MuxCache() *cache.MuxCache {
	return &s.muxCache
}

func (s *Snapshotter) Start(stopCtx context.Context) error {
	group, groupCtx := errgroup.WithContext(stopCtx)
	group.Go(func() error {
		return s.startServices(groupCtx)
	})
	group.Go(func() error {
		return s.startEndpoints(groupCtx)
	})
	return group.Wait()
}

func (s *Snapshotter) snapshotResourceGaugeCallback(_ context.Context, result metric.Int64Observer) error {
	for k, r := range s.getServiceResourcesByType() {
		result.Observe(int64(len(r)), metric.WithAttributes(meter.TypeURLAttrKey.String(k)))
	}
	for k, r := range s.getEndpointResourcesByType() {
		result.Observe(int64(len(r)), metric.WithAttributes(meter.TypeURLAttrKey.String(k)))
	}
	return nil
}

func (s *Snapshotter) apiGatewayEndpointGaugeCallback(_ context.Context, result metric.Int64Observer) error {
	for k, stat := range s.getAPIGatewayStats() {
		result.Observe(int64(stat), metric.WithAttributes(meter.APIGatewayAttrKey.String(k)))
	}
	return nil
}

func (s *Snapshotter) setServiceResourcesByType(serviceResourcesByType map[string][]types.Resource) {
	s.resourcesByTypeLock.Lock()
	defer s.resourcesByTypeLock.Unlock()
	s.serviceResourcesByType = serviceResourcesByType
}

func (s *Snapshotter) getServiceResourcesByType() map[string][]types.Resource {
	s.resourcesByTypeLock.RLock()
	defer s.resourcesByTypeLock.RUnlock()
	return s.serviceResourcesByType
}

func (s *Snapshotter) setEndpointResourcesByType(endpointResourcesByType map[string][]types.Resource) {
	s.resourcesByTypeLock.Lock()
	defer s.resourcesByTypeLock.Unlock()
	s.endpointResourcesByType = endpointResourcesByType
}

func (s *Snapshotter) getEndpointResourcesByType() map[string][]types.Resource {
	s.resourcesByTypeLock.RLock()
	defer s.resourcesByTypeLock.RUnlock()
	return s.endpointResourcesByType
}

func (s *Snapshotter) setAPIGatewayStats(apiGatewayStats map[string]int) {
	s.resourcesByTypeLock.Lock()
	defer s.resourcesByTypeLock.Unlock()
	s.apiGatewayStats = apiGatewayStats
}

func (s *Snapshotter) getAPIGatewayStats() map[string]int {
	s.resourcesByTypeLock.RLock()
	defer s.resourcesByTypeLock.RUnlock()
	return s.apiGatewayStats
}