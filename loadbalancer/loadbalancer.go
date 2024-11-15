package loadbalancer

import (
	"errors"
	"sync"
)

// LoadBalancer interface
type LoadBalancer interface {
	NextEndpoint() string
}

// Config holds the configuration for the services
type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig holds the configuration for a service
type ServiceConfig struct {
	Endpoints    []string `yaml:"endpoints"`
	LoadBalancer string   `yaml:"loadBalancer"`
}

// Service holds the load balancer for the service
type Service struct {
	LoadBalancer LoadBalancer
}

func NewService(loadBalancer *LoadBalancer) *Service {
	return &Service{
		LoadBalancer: *loadBalancer,
	}
}

// GetService fetches the service configuration and selects the load balancer based on the config.
func GetService(lock *sync.Mutex, config Config, serviceName string) (*Service, error) {
	lock.Lock()
	defer lock.Unlock()

	// Retrieve the service config
	svcConfig, exists := config.Services[serviceName]
	if !exists {
		return nil, errors.New("service not found")
	}

	// Select the load balancer based on the configuration
	var lb LoadBalancer
	switch svcConfig.LoadBalancer {
	case "round-robin":
		lb = NewRoundRobin(svcConfig.Endpoints)
	case "least-connections":
		lb = NewLeastConnections(svcConfig.Endpoints)
	default:
		return nil, errors.New("unsupported load balancer type")
	}

	return NewService(&lb), nil
}
