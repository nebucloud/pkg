package snapshot

import (
	"context"
	"fmt"
	"sort"

	"github.com/edgedb/edgedb-go"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	consulApi "github.com/hashicorp/consul/api"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/nebucloud/pkg/logger"
	"github.com/nebucloud/pkg/xds/meter"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type endpointCacheItem struct {
	version   string
	resources []types.Resource
}

func (s *Snapshotter) startEndpoints(ctx context.Context, memdb *memdb.MemDB, edgedbClient *edgedb.Client, consulClient *consulApi.Client, logger *logger.Klogger) error {
	emit := func() {}

	store := k8scache.NewUndeltaStore(func(v []interface{}) {
		emit()
	}, k8scache.DeletionHandlingMetaNamespaceKeyFunc)

	reflector := k8scache.NewReflector(&k8scache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			// Check if endpoints are cached in MemDB
			txn := memdb.Txn(false)
			defer txn.Abort()
			iter, err := txn.Get("endpoints", "id")
			if err != nil {
				return nil, err
			}
			var endpoints []corev1.Endpoints
			for obj := iter.Next(); obj != nil; obj = iter.Next() {
				endpoint := obj.(*corev1.Endpoints)
				endpoints = append(endpoints, *endpoint)
			}
			if len(endpoints) > 0 {
				return &corev1.EndpointsList{Items: endpoints}, nil
			}

			// If endpoints are not cached, fetch them from Kubernetes
			return s.client.CoreV1().Endpoints("").List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return s.client.CoreV1().Endpoints("").Watch(ctx, options)
		},
	}, &corev1.Endpoints{}, store, s.ResyncPeriod)

	var lastSnapshotHash uint64

	emit = func() {
		version := reflector.LastSyncResourceVersion()
		s.kubeEventCounter.Add(ctx, 1, metric.WithAttributes(meter.ResourceAttrKey.String("endpoints")))

		endpoints := sliceToEndpoints(store.List())

		// Persist endpoints in EdgeDB
		for _, ep := range endpoints {
			err := s.persistEndpointInEdgeDB(ctx, edgedbClient, ep)
			if err != nil {
				klog.Errorf("Failed to persist endpoint in EdgeDB: %v", err)
			}
		}

		// Register endpoints with Consul
		for _, ep := range endpoints {
			err := s.registerEndpointWithConsul(consulClient, ep)
			if err != nil {
				klog.Errorf("Failed to register endpoint with Consul: %v", err)
			}
		}

		endpointsResources, err := s.kubeEndpointsToResources(endpoints, memdb, logger)
		if err != nil {
			klog.Errorf("Failed to convert endpoints to resources: %v", err)
			return
		}

		hash, err := resourcesHash(endpointsResources)
		if err == nil {
			if hash == lastSnapshotHash {
				klog.V(4).Info("new snapshot is equivalent to the previous one")
				return
			}
			lastSnapshotHash = hash
		} else {
			klog.Errorf("fail to hash snapshot: %s", err)
		}

		resourcesByType := resourcesToMap(endpointsResources)
		s.setEndpointResourcesByType(resourcesByType)

		snapshot, err := cache.NewSnapshot(version, resourcesByType)
		if err != nil {
			panic(err)
		}

		s.endpointsCache.SetSnapshot(ctx, "", snapshot)

		// Cache endpoints in MemDB
		txn := memdb.Txn(true)
		for _, ep := range endpoints {
			if err := txn.Insert("endpoints", ep); err != nil {
				txn.Abort()
				klog.Errorf("Failed to cache endpoint in MemDB: %v", err)
				return
			}
		}
		txn.Commit()
	}

	reflector.Run(ctx.Done())
	return nil
}

func (s *Snapshotter) persistEndpointInEdgeDB(ctx context.Context, client *edgedb.Client, ep *corev1.Endpoints) error {
	// Implement the logic to persist the endpoint data in EdgeDB using the provided client
	// You can use EdgeDB's query language to store the endpoint data in the appropriate tables/collections
	// Example:
	// err := client.QuerySingle(ctx, `
	//   INSERT Endpoint {
	//     name := <str>$name,
	//     namespace := <str>$namespace,
	//     // Add other endpoint fields as needed
	//   }
	// `, map[string]interface{}{
	//   "name":      ep.Name,
	//   "namespace": ep.Namespace,
	// })
	// return err

	return nil // Replace with your actual implementation
}

func (s *Snapshotter) registerEndpointWithConsul(client *consulApi.Client, ep *corev1.Endpoints) error {
	// Implement the logic to register the endpoint with Consul using the provided client
	// You can use Consul's API to register the endpoint as a service with the appropriate metadata
	// Example:
	// registration := &consulApi.AgentServiceRegistration{
	//   ID:      fmt.Sprintf("%s-%s", ep.Name, ep.Namespace),
	//   Name:    ep.Name,
	//   Address: ep.Subsets[0].Addresses[0].IP,
	//   // Add other endpoint metadata as needed
	// }
	// err := client.Agent().ServiceRegister(registration)
	// return err

	return nil // Replace with your actual implementation
}

func sliceToEndpoints(s []interface{}) []*corev1.Endpoints {
	out := make([]*corev1.Endpoints, len(s))
	for i, v := range s {
		out[i] = v.(*corev1.Endpoints)
	}
	return out
}

// kubeServicesToResources convert list of Kubernetes endpoints to Endpoint
func (s *Snapshotter) kubeEndpointsToResources(endpoints []*corev1.Endpoints, memdb *memdb.MemDB, logger *logger.Klogger) ([]types.Resource, error) {
	var out []types.Resource

	for _, ep := range endpoints {
		resources, err := s.kubeEndpointToResources(ep, memdb, logger)
		if err != nil {
			logger.Errorf("Failed to convert endpoint to resources: %v", err)
			continue
		}
		out = append(out, resources...)
	}

	return out, nil
}

func (s *Snapshotter) kubeEndpointToResources(ep *corev1.Endpoints, memdb *memdb.MemDB, logger *logger.Klogger) ([]types.Resource, error) {
	name, err := k8scache.MetaNamespaceKeyFunc(ep)
	if err != nil {
		logger.Errorf("fail to get object key: %s", err)
		return nil, err
	}

	// Check if the endpoint is cached in MemDB
	txn := memdb.Txn(false)
	defer txn.Abort()

	cached, err := txn.First("endpoints", "id", name)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		item := cached.(endpointCacheItem)
		if item.version == ep.ResourceVersion {
			return item.resources, nil
		}
	}

	var out []types.Resource

	for _, subset := range ep.Subsets {
		for _, port := range subset.Ports {
			var portName string
			if port.Name == "" {
				portName = fmt.Sprintf("%s.%s:%d", ep.Name, ep.Namespace, port.Port)
			} else {
				portName = fmt.Sprintf("%s.%s:%s", ep.Name, ep.Namespace, port.Name)
			}

			cla := &endpointv3.ClusterLoadAssignment{
				ClusterName: portName,
				Endpoints: []*endpointv3.LocalityLbEndpoints{
					{
						LoadBalancingWeight: wrapperspb.UInt32(1),
						Locality:            &corev3.Locality{},
						LbEndpoints:         []*endpointv3.LbEndpoint{},
					},
				},
			}
			out = append(out, cla)

			sortedAddresses := subset.Addresses
			sort.SliceStable(sortedAddresses, func(i, j int) bool {
				l := sortedAddresses[i].IP
				r := sortedAddresses[j].IP
				return l < r
			})

			for _, addr := range sortedAddresses {
				hostname := addr.Hostname
				if hostname == "" && addr.TargetRef != nil {
					hostname = fmt.Sprintf("%s.%s", addr.TargetRef.Name, addr.TargetRef.Namespace)
				}
				if hostname == "" && addr.NodeName != nil {
					hostname = *addr.NodeName
				}

				cla.Endpoints[0].LbEndpoints = append(cla.Endpoints[0].LbEndpoints, &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Protocol: corev3.SocketAddress_TCP,
										Address:  addr.IP,
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: uint32(port.Port),
										},
									},
								},
							},
							Hostname: hostname,
						},
					},
				})
			}
		}
	}

	// Cache the endpoint resources in MemDB
	txn = memdb.Txn(true)
	if err := txn.Insert("endpoints", endpointCacheItem{
		version:   ep.ResourceVersion,
		resources: out,
	}); err != nil {
		txn.Abort()
		return nil, err
	}
	txn.Commit()

	return out, nil
}

/*
func (s *Snapshotter) kubeEndpointsToResources(endpoints []*corev1.Endpoints) []types.Resource {
	var out []types.Resource

	for _, ep := range endpoints {
		out = append(out, s.kubeEndpointToResources(ep)...)
	}

	return out
}

func (s *Snapshotter) startEndpoints(ctx context.Context) error {
	emit := func() {}

	store := k8scache.NewUndeltaStore(func(v []interface{}) {
		emit()
	}, k8scache.DeletionHandlingMetaNamespaceKeyFunc)

	reflector := k8scache.NewReflector(&k8scache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return s.client.CoreV1().Endpoints("").List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return s.client.CoreV1().Endpoints("").Watch(ctx, options)
		},
	}, &corev1.Endpoints{}, store, s.ResyncPeriod)

	var lastSnapshotHash uint64

	emit = func() {
		version := reflector.LastSyncResourceVersion()
		s.kubeEventCounter.Add(ctx, 1, metric.WithAttributes(meter.ResourceAttrKey.String("endpoints")))

		endpoints := sliceToEndpoints(store.List())
		endpointsResources := s.kubeEndpointsToResources(endpoints)
		hash, err := resourcesHash(endpointsResources)
		if err == nil {
			if hash == lastSnapshotHash {
				klog.V(4).Info("new snapshot is equivalent to the previous one")
				return
			}
			lastSnapshotHash = hash
		} else {
			klog.Errorf("fail to hash snapshot: %s", err)
		}

		resourcesByType := resourcesToMap(endpointsResources)
		s.setEndpointResourcesByType(resourcesByType)

		snapshot, err := cache.NewSnapshot(version, resourcesByType)
		if err != nil {
			panic(err)
		}

		s.endpointsCache.SetSnapshot(ctx, "", snapshot)
	}

	reflector.Run(ctx.Done())
	return nil
}
*/

/*
func (s *Snapshotter) kubeEndpointToResources(ep *corev1.Endpoints) []types.Resource {
	name, err := k8scache.MetaNamespaceKeyFunc(ep)
	if err != nil {
		klog.Errorf("fail to get object key: %s", err)
		return nil
	}
	if val, ok := s.endpointResourceCache[name]; ok && val.version == ep.ResourceVersion {
		return val.resources
	}

	var out []types.Resource

	for _, subset := range ep.Subsets {
		for _, port := range subset.Ports {
			var portName string
			if port.Name == "" {
				portName = fmt.Sprintf("%s.%s:%d", ep.Name, ep.Namespace, port.Port)
			} else {
				portName = fmt.Sprintf("%s.%s:%s", ep.Name, ep.Namespace, port.Name)
			}

			cla := &endpointv3.ClusterLoadAssignment{
				ClusterName: portName,
				Endpoints: []*endpointv3.LocalityLbEndpoints{
					{
						LoadBalancingWeight: wrapperspb.UInt32(1),
						Locality:            &corev3.Locality{},
						LbEndpoints:         []*endpointv3.LbEndpoint{},
					},
				},
			}
			out = append(out, cla)

			sortedAddresses := subset.Addresses
			sort.SliceStable(sortedAddresses, func(i, j int) bool {
				l := sortedAddresses[i].IP
				r := sortedAddresses[j].IP
				return l < r
			})

			for _, addr := range sortedAddresses {
				hostname := addr.Hostname
				if hostname == "" && addr.TargetRef != nil {
					hostname = fmt.Sprintf("%s.%s", addr.TargetRef.Name, addr.TargetRef.Namespace)
				}
				if hostname == "" && addr.NodeName != nil {
					hostname = *addr.NodeName
				}

				cla.Endpoints[0].LbEndpoints = append(cla.Endpoints[0].LbEndpoints, &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Protocol: corev3.SocketAddress_TCP,
										Address:  addr.IP,
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: uint32(port.Port),
										},
									},
								},
							},
							Hostname: hostname,
						},
					},
				})
			}
		}
	}

	s.endpointResourceCache[name] = endpointCacheItem{
		version:   ep.ResourceVersion,
		resources: out,
	}

	return out
}
*/
