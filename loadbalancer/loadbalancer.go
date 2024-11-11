package loadbalancer

import (
	"errors"
	"sync"
)

// Config holds the configuration for the services
type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig holds the configuration for a service
type ServiceConfig struct {
	Endpoints    []string `yaml:"endpoints"`
	LoadBalancer string   `yaml:"loadBalancer"`
}

// LoadBalancer interface
type LoadBalancer interface {
	NextEndpoint() string
}

// Service holds the load balancer for the service
type Service struct {
	LoadBalancer LoadBalancer
}

// // NewService creates a new service
// func NewService(lb LoadBalancer) *Service {
// 	return &Service{LoadBalancer: lb}
// }

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
		lb = &RoundRobin{endpoints: svcConfig.Endpoints}
	case "least-connections":
		lb = &LeastConnections{endpoints: svcConfig.Endpoints, connCount: make(map[string]int)}
	default:
		return nil, errors.New("unsupported load balancer type")
	}

	return &Service{LoadBalancer: lb}, nil
}

type RoundRobin struct {
	endpoints []string
	idx       int
	mux       sync.Mutex
}

func (rr *RoundRobin) NextEndpoint() string {
	rr.mux.Lock()
	defer rr.mux.Unlock()
	endpoint := rr.endpoints[rr.idx]
	rr.idx = (rr.idx + 1) % len(rr.endpoints)
	return endpoint
}

// Implementation of least-connections load balancer (for demonstration)
type LeastConnections struct {
	endpoints []string
	connCount map[string]int
	mux       sync.Mutex
}

func (lc *LeastConnections) NextEndpoint() string {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	minConns := int(^uint(0) >> 1)
	selected := ""
	for _, endpoint := range lc.endpoints {
		if lc.connCount[endpoint] < minConns {
			minConns = lc.connCount[endpoint]
			selected = endpoint
		}
	}
	lc.connCount[selected]++
	return selected
}
