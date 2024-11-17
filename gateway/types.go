package gateway

// LoadBalancer interface defines the methods that a load balancer should implement.
type LoadBalancer interface {
	NextEndpoint() string
}

// Config represents the configuration for the gateway.
type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig represents the configuration for a service.
type ServiceConfig struct {
	Endpoints    []string `yaml:"endpoints"`
	LoadBalancer string   `yaml:"loadBalancer"`
}

// GatewayServiceConfig represents the configuration for a service in the gateway.
type GatewayServiceConfig struct {
	serviceName      string
	loadBalancerType LoadBalancer
	endpoints        []string
}
