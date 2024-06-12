package snapshot

import (
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/edgedb/edgedb-go"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	consulApi "github.com/hashicorp/consul/api"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/nebucloud/pkg/logger"
	"github.com/nebucloud/pkg/xds/meter"
	"go.opentelemetry.io/otel/metric"
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

	logger    *logger.Klogger
	dbContext context.Context
	dbCancel  context.CancelFunc
}

// NewSnapshotter creates a new Snapshotter instance.
func NewSnapshotter(client kubernetes.Interface, logger *logger.Klogger, dbProvider DatabaseProvider, rcache *ristretto.Cache, consulClient *consulApi.Client) *Snapshotter {
	dbContext, dbCancel := context.WithCancel(context.Background())

	ss := &Snapshotter{
		ResyncPeriod: 10 * time.Minute,
		client:       client,
	}

	ss.servicesCache = cache.NewSnapshotCache(false, EmptyNodeID{}, logger)
	ss.endpointsCache = cache.NewSnapshotCache(false, EmptyNodeID{}, logger)
	ss.muxCache = cache.MuxCache{
		Classify: func(r *cache.Request) string {
			return mapTypeURL(r.TypeUrl)
		},
		ClassifyDelta: func(r *cache.DeltaRequest) string {
			return mapTypeURL(r.TypeUrl)
		},
		Caches: map[string]cache.Cache{
			"services":  ss.servicesCache,
			"endpoints": ss.endpointsCache,
		},
	}

	ss.endpointResourceCache = map[string]endpointCacheItem{}
	ss.logger = logger
	ss.dbContext = dbContext
	ss.dbCancel = dbCancel

	meter := meter.GetMeter()
	ss.kubeEventCounter, _ = meter.Int64Counter("xds_kube_events")
	meter.Int64ObservableGauge("xds_snapshot_resources", metric.WithInt64Callback(ss.snapshotResourceGaugeCallback))
	meter.Int64ObservableGauge("xds_apigateway_endpoints", metric.WithInt64Callback(ss.apiGatewayEndpointGaugeCallback))

	go ss.startWithDatabase(dbProvider, rcache, consulClient)

	return ss
}

// startWithDatabase starts the Snapshotter with the provided database, cache, and Consul client.
func (s *Snapshotter) startWithDatabase(dbProvider DatabaseProvider, cache *ristretto.Cache, consulClient *consulApi.Client) {
	_, err := dbProvider.GetDatabase(s.dbContext)
	if err != nil {
		s.logger.Errorf("Failed to get database: %v", err)
		s.dbCancel()
		return
	}
	defer s.dbCancel()

	memdb, err := s.createMemDB()
	if err != nil {
		s.logger.Errorf("Failed to create MemDB: %v", err)
		return
	}

	edgedbClient, err := s.createEdgeDBClient()
	if err != nil {
		s.logger.Errorf("Failed to create EdgeDB client: %v", err)
		return
	}
	defer edgedbClient.Close()

	group, groupCtx := errgroup.WithContext(s.dbContext)
	group.Go(func() error {
		return s.startServices(groupCtx, memdb, edgedbClient, consulClient)
	})
	group.Go(func() error {
		return s.startEndpoints(groupCtx, memdb, edgedbClient, consulClient, s.logger)
	})
	err = group.Wait()
	if err != nil {
		s.logger.Errorf("Error in reconciliation loops: %v", err)
	}
}

// createMemDB creates a new instance of MemDB.
func (s *Snapshotter) createMemDB() (*memdb.MemDB, error) {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"services": {
				Name: "services",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "ID"},
					},
				},
			},
			// Add other tables as needed
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// createEdgeDBClient creates a new instance of EdgeDB client.
func (s *Snapshotter) createEdgeDBClient() (*edgedb.Client, error) {
	client, err := edgedb.CreateClient(s.dbContext, edgedb.Options{
		Host:            "",
		Port:            0,
		Credentials:     []byte{},
		CredentialsFile: "",
		User:            "",
		Database:        "",
		Branch:          "",
		// Password:           edgedbtypes.OptionalStr{},
		ConnectTimeout:     10 * time.Second,
		WaitUntilAvailable: 30 * time.Second,
		Concurrency:        4,
		TLSOptions:         edgedb.TLSOptions{},
		TLSCAFile:          "",
		TLSSecurity:        "",
		ServerSettings:     map[string][]byte{},
		SecretKey:          "",
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// MuxCache returns the MuxCache.
func (s *Snapshotter) MuxCache() *cache.MuxCache {
	return &s.muxCache
}

func (s *Snapshotter) Start(stopCtx context.Context, memdb *memdb.MemDB, edgedbClient *edgedb.Client, consulClient *consulApi.Client, logger *logger.Klogger) error {
	group, groupCtx := errgroup.WithContext(stopCtx)
	group.Go(func() error {
		return s.startServices(groupCtx, memdb, edgedbClient, consulClient)
	})
	group.Go(func() error {
		return s.startEndpoints(groupCtx, memdb, edgedbClient, consulClient, logger)
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
