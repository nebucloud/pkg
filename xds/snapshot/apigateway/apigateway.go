package apigateway

import (
	"fmt"
	"regexp"
	"strings"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/nebucloud/pkg/logger"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
)

const (
	NameAnnotation    = "xds.nebucloud.com/api-gateway"
	ServiceAnnotation = "xds.nebucloud.com/grpc-service"
	PortName          = "grpc"
)

var nameRegex = regexp.MustCompile("^[a-z0-9][a-z0-9-]{0,63}$")

type Params struct {
	fx.In

	Services []*v1.Service `name:"kubeServices"`
	Logger   *logger.Klogger
}

func NewApiGateway(params Params) ([]types.Resource, map[string]int) {
	return FromKubeServices(params.Services, params.Logger)
}

func FromKubeServices(services []*v1.Service, logger *logger.Klogger) ([]types.Resource, map[string]int) {
	routerConfigs := map[string]*routev3.RouteConfiguration{}
	gateways := map[string]*listenerv3.Listener{}

	router, _ := anypb.New(&routerv3.Router{})

outer:
	for _, svc := range services {
		apiGatewayRaw, ok := svc.Annotations[NameAnnotation]
		if !ok {
			continue
		}
		apiGateways := strings.Split(apiGatewayRaw, ",")
		for _, name := range apiGateways {
			if !nameRegex.MatchString(name) {
				logger.Warnf("Service %s/%s API Gateway %s does not match regex %s", svc.Namespace, svc.Name, name, nameRegex.String())
				continue outer
			}
		}

		grpcServiceRaw, ok := svc.Annotations[ServiceAnnotation]
		if !ok {
			continue
		}
		rpcs := strings.Split(grpcServiceRaw, ",")

		hasGrpcPort := false
		for _, port := range svc.Spec.Ports {
			if port.Name == PortName {
				hasGrpcPort = true
				break
			}
		}
		if !hasGrpcPort {
			logger.Warnf("Service %s/%s has API Gateway annotation but no grpc named port", svc.Namespace, svc.Name)
			continue
		}

		for _, gateway := range apiGateways {
			if _, ok = gateways[gateway]; !ok {
				gateways[gateway] = &listenerv3.Listener{
					Name: gateway,
				}
			}

			routeConfig, ok := routerConfigs[gateway]
			if !ok {
				routeConfig = &routev3.RouteConfiguration{
					Name: gateway,
					VirtualHosts: []*routev3.VirtualHost{
						{
							Name:    gateway,
							Domains: []string{gateway},
						},
					},
				}
				routerConfigs[gateway] = routeConfig
			}

			for _, rpc := range rpcs {
				routeConfig.VirtualHosts[0].Routes = append(routeConfig.VirtualHosts[0].Routes, &routev3.Route{
					Name: rpc,
					Match: &routev3.RouteMatch{
						PathSpecifier: &routev3.RouteMatch_Prefix{
							Prefix: "/" + rpc + "/",
						},
					},
					Action: &routev3.Route_Route{
						Route: &routev3.RouteAction{
							ClusterSpecifier: &routev3.RouteAction_Cluster{
								Cluster: fmt.Sprintf("%s.%s:%s", svc.Name, svc.Namespace, PortName),
							},
						},
					},
				})
			}
		}
	}

	var out []types.Resource
	stats := make(map[string]int)

	for name, gateway := range gateways {
		manager, _ := anypb.New(&managerv3.HttpConnectionManager{
			HttpFilters: []*managerv3.HttpFilter{
				{
					Name: wellknown.Router,
					ConfigType: &managerv3.HttpFilter_TypedConfig{
						TypedConfig: router,
					},
				},
			},
			RouteSpecifier: &managerv3.HttpConnectionManager_RouteConfig{
				RouteConfig: routerConfigs[name],
			},
		})

		gateway.ApiListener = &listenerv3.ApiListener{
			ApiListener: manager,
		}

		out = append(out, gateway)
		stats[gateway.Name] = len(routerConfigs[name].VirtualHosts[0].Routes)
	}

	for _, route := range routerConfigs {
		out = append(out, route)
	}

	return out, stats
}

var ApiGatewayModule = fx.Options(
	fx.Provide(NewApiGateway),
)
