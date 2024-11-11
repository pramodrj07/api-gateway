package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var config *Config
var mux sync.RWMutex

// Load config at startup and periodically reload
func init() {
	var err error
	config, err = LoadConfig("configuration/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Start config watcher for periodic reloads
	go watchConfig()
}

func main() {
	http.HandleFunc("/", routeHandler)
	fmt.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// getService fetches the service configuration and selects the load balancer based on the config.
func getService(serviceName string) (*Service, error) {
	mux.RLock()
	defer mux.RUnlock()

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

	return &Service{loadBalancer: lb}, nil
}

// Route handler
func routeHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Path[1:]
	service, err := getService(serviceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	url := service.loadBalancer.NextEndpoint() + r.URL.Path
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
	}
}

func watchConfig() {
	for {
		time.Sleep(5 * time.Second)
		newConfig, err := LoadConfig("config/config.yaml")
		if err == nil {
			mux.Lock()
			config = newConfig
			mux.Unlock()
			log.Println("Configuration reloaded")
		}
	}
}
