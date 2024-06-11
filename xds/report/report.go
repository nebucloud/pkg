package report

import (
	"context"
	"sync"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	loadReportingService "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v3"
	"github.com/nebucloud/pkg/logger"
	"github.com/nebucloud/pkg/xds/meter"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

// MeterServer is a load reporting service server that uses custom logging and metrics.
type MeterServer struct {
	loadReportingService.UnimplementedLoadReportingServiceServer

	lock           sync.Mutex
	nodesConnected map[string]bool

	statsIntervalInSeconds int64
	statsUpdateCounter     metric.Int64Counter
	nodeGauge              metric.Int64UpDownCounter
	logger                 *logger.Klogger

	stopCh chan struct{}
}

// Option is a function type used to configure the MeterServer.
type Option func(s *MeterServer)

// NewMeterServer creates a new MeterServer with the given options.
func NewMeterServer(logger *logger.Klogger, opts ...Option) loadReportingService.LoadReportingServiceServer {
	meter := meter.GetMeter()
	lrsUpdatesCounter, _ := meter.Int64Counter("lrs_updates")
	lrsNodesCounter, _ := meter.Int64UpDownCounter("lrs_nodes")
	s := &MeterServer{
		nodesConnected:         make(map[string]bool),
		statsIntervalInSeconds: 300,
		statsUpdateCounter:     lrsUpdatesCounter,
		nodeGauge:              lrsNodesCounter,
		logger:                 logger,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// StreamLoadStats handles streaming load stats requests.
func (s *MeterServer) StreamLoadStats(stream loadReportingService.LoadReportingService_StreamLoadStatsServer) error {
	var node *corev3.Node
	for {
		req, err := stream.Recv()
		if err != nil {
			if node != nil {
				s.removeNode(stream.Context(), node)
			}
			return err
		}
		if node == nil {
			node = req.Node
		}

		s.HandleRequest(stream, req)
	}
}

// HandleRequest handles a single load stats request.
func (s *MeterServer) HandleRequest(stream loadReportingService.LoadReportingService_StreamLoadStatsServer, request *loadReportingService.LoadStatsRequest) {
	nodeID := request.GetNode().GetId()

	s.statsUpdateCounter.Add(stream.Context(), 1)

	s.lock.Lock()
	defer s.lock.Unlock()

	if _, exist := s.nodesConnected[nodeID]; !exist {
		s.logger.InfoS("New node connected", "node_id", nodeID, "cluster_str", request.Node.Cluster)
		s.nodesConnected[nodeID] = true
		s.nodeGauge.Add(stream.Context(), 1)

		err := stream.Send(&loadReportingService.LoadStatsResponse{
			Clusters:                  []string{"dummy_cluster"},
			LoadReportingInterval:     &durationpb.Duration{Seconds: s.statsIntervalInSeconds},
			ReportEndpointGranularity: true,
		})
		if err != nil {
			s.logger.Errorf("Unable to send response to node %s due to err: %s", nodeID, err)
			delete(s.nodesConnected, nodeID)
			s.logger.InfoS("Node disconnected", "node_id", nodeID, "cluster_str", request.Node.Cluster)
			s.nodeGauge.Add(stream.Context(), -1)
		}
		return
	}

	for _, clusterStats := range request.ClusterStats {
		if len(clusterStats.UpstreamLocalityStats) > 0 {
			s.logger.InfoS("Got stats", "node_id", request.Node.Id, "cluster_str", request.Node.Cluster, "cluster_stats", clusterStats)
		}
	}
}

// removeNode removes a node from the nodesConnected map.
func (s *MeterServer) removeNode(ctx context.Context, node *corev3.Node) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.nodesConnected, node.Id)

	s.logger.InfoS("Node disconnected", "node_id", node.Id, "cluster_str", node.Cluster)

	s.nodeGauge.Add(ctx, -1)
}

// WithStatsIntervalInSeconds returns an option to set the stats interval in seconds.
func WithStatsIntervalInSeconds(statsIntervalInSeconds int64) Option {
	return func(s *MeterServer) {
		s.statsIntervalInSeconds = statsIntervalInSeconds
	}
}

// Run starts the MeterServer.
func (s *MeterServer) Run() {
	<-s.stopCh
}

// Stop stops the MeterServer.
func (s *MeterServer) Stop() {
	close(s.stopCh)
}

// LoadReportingServiceModule is an FX module that provides the load reporting service.
var LoadReportingServiceModule = fx.Options(
	fx.Provide(
		func(logger *logger.Klogger) loadReportingService.LoadReportingServiceServer {
			return NewMeterServer(
				logger,
				WithStatsIntervalInSeconds(300),
			)
		},
	),
	fx.Invoke(RegisterLoadReportingService),
)

// RegisterLoadReportingService registers the load reporting service with the gRPC server.
func RegisterLoadReportingService(lc fx.Lifecycle, grpcServer *grpc.Server, lrsServer loadReportingService.LoadReportingServiceServer) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			loadReportingService.RegisterLoadReportingServiceServer(grpcServer, lrsServer)
			return nil
		},
	})
}
