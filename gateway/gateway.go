package gateway

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	log "go.uber.org/zap"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

// Gateway represents an API Gateway.
type Gateway struct {
	ctx             context.Context
	watcherChan     chan string
	lock            *sync.Mutex
	configPath      string
	serviceRegistry map[string]*GatewayServiceConfig
	log             *log.Logger
}

// NewGateway creates a new Gateway instance.
func NewGateway(ctx context.Context, lock *sync.Mutex, configPath string, log *log.Logger) *Gateway {
	return &Gateway{
		ctx:             context.Background(),
		watcherChan:     make(chan string),
		lock:            lock,
		configPath:      configPath,
		serviceRegistry: make(map[string]*GatewayServiceConfig),
		log:             log,
	}
}

// NewGatewayServiceConfig creates a new GatewayServiceConfig instance.
func NewGatewayServiceConfig(serviceName string, loadBalancerType LoadBalancer, endpoints []string) *GatewayServiceConfig {
	return &GatewayServiceConfig{
		serviceName:      serviceName,
		loadBalancerType: loadBalancerType,
		endpoints:        endpoints,
	}
}

// Run starts the API Gateway.
func (gateway *Gateway) Run() {
	err := gateway.loadConfig()
	if err != nil {
		gateway.log.Sugar().DPanic("Failed to load config: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		gateway.log.Sugar().DPanic("Failed to create watcher: %v", err)
	}

	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				gateway.log.Sugar().Infof("event:", event)
				if event.Has(fsnotify.Write) {
					go gateway.updateServiceConfig(gateway.watcherChan)
					gateway.log.Sugar().Infof("modified file:", event.Name)
					gateway.watcherChan <- event.String()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				gateway.log.Sugar().Infof("error:", err)
			case <-gateway.ctx.Done():
				gateway.log.Sugar().Infof("Context done, exiting watcher")
				return
			}
		}
	}()

	err = watcher.Add(gateway.configPath)
	if err != nil {
		gateway.log.Sugar().Fatalf("Failed to add watcher: %v", err)
	}

	gateway.log.Sugar().Infof("API Gateway listening on :8080")
	// Initialize a new mux router
	mux := http.NewServeMux()
	// Register the route handler
	mux.HandleFunc("/", http.HandlerFunc(gateway.routeHandler))
	if err = http.ListenAndServe(":8080", mux); err != nil {
		gateway.log.Sugar().Fatalf("Failed to start server: %v", err)
	}
}

// loadConfig loads the configuration from the config file.
func (g *Gateway) loadConfig() error {
	data, err := os.ReadFile(g.configPath)
	if err != nil {
		return err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return err
	}

	// Update the service registry with the new configuration
	g.lock.Lock()
	defer g.lock.Unlock()

	for serviceName, serviceConfig := range config.Services {
		var lb LoadBalancer
		switch serviceConfig.LoadBalancer {
		case "round-robin":
			lb = NewRoundRobin(serviceConfig.Endpoints, g.log)
		case "least-connections":
			lb = NewLeastConnections(serviceConfig.Endpoints, g.log)
		default:
			lb = nil
		}

		g.serviceRegistry[serviceName] = NewGatewayServiceConfig(serviceName, lb, serviceConfig.Endpoints)
	}

	return nil
}

// updateServiceConfig updates the service configuration.
func (g *Gateway) updateServiceConfig(chan string) {
	msg := <-g.watcherChan
	g.log.Sugar().Infof("Received update event for file: %s", msg)

	err := g.loadConfig()
	if err != nil {
		g.log.Sugar().Fatalf("Failed to load config: %v", err)
	}

	g.log.Sugar().Infof("Updated Service Registry: %+v\n", g.serviceRegistry)
}

func (g *Gateway) routeHandler(w http.ResponseWriter, r *http.Request) {
	// Log when the request is received
	g.log.Sugar().Infof("Received request: %s %s", r.Method, r.URL.Path) // Construct the URL and perform the HTTP request
	serviceName := strings.TrimPrefix(r.URL.Path, "/")
	g.log.Sugar().Infof("Service name: %s", serviceName)

	// check if the service exists in the service registry. If exists, fetch the service from the registry.
	// If not, fetch the service from the load balancer.
	service, exists := g.serviceRegistry[serviceName]
	if !exists {
		g.log.Sugar().Infof("Service not found: %s", serviceName)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	if service.loadBalancerType == nil {
		g.log.Sugar().Infof("Load balancer not found for service: %s", serviceName)
		http.Error(w, "Load balancer not found", http.StatusServiceUnavailable)
		return
	}

	g.log.Sugar().Infof("Service found: %s with endpoints %v", serviceName, service.endpoints)
	url := service.loadBalancerType.NextEndpoint() + r.URL.Path
	g.log.Sugar().Infof("Forwarding request to: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		// Log if the service is unavailable
		g.log.Sugar().Infof("Error fetching from service: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Log the response status code
	g.log.Sugar().Infof("Received response: %d", resp.StatusCode)

	// Copy the response body to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		// Log an error if there's a problem copying the response
		g.log.Sugar().Infof("Error writing response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
	}
}
