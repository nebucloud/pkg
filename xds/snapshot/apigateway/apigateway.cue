package apigateway

// RouteConfiguration represents the Envoy RouteConfiguration.
RouteConfiguration: {
	name: string
	virtual_hosts: [...VirtualHost]
}

// VirtualHost represents a virtual host within the RouteConfiguration.
VirtualHost: {
	name: string
	domains: [...string]
	routes: [...Route]
}

// Route represents a route within a VirtualHost.
Route: {
	name:  string
	match: RouteMatch
	route: RouteAction
}

// RouteMatch represents the match criteria for a route.
RouteMatch: {
	prefix: string
}

// RouteAction represents the action to take when a route matches.
RouteAction: {
	cluster: string
}

// Listener represents an Envoy Listener.
Listener: {
	name:         string
	api_listener: ApiListener
}

// ApiListener represents the API listener configuration.
ApiListener: {
	api_listener: HttpConnectionManager
}

// HttpConnectionManager represents the HTTP connection manager configuration.
HttpConnectionManager: {
	http_filters: [...HttpFilter]
	route_specifier: RouteSpecifier
}

// HttpFilter represents an HTTP filter configuration.
HttpFilter: {
	name: string
	typed_config: {...}
}

// RouteSpecifier represents the route specifier configuration.
RouteSpecifier: {
	route_config?: RouteConfiguration
}
