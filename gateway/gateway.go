package gateway

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Gateway struct {
	ctx             context.Context
	watcherChan     chan string
	lock            *sync.Mutex
	configPath      string
	serviceRegistry map[string]*GatewayServiceConfig
	clientset       *kubernetes.Clientset
	servicePods     map[string][]string // Maps service name to pod endpoints
	servicePodsMu   sync.Mutex
	roundRobinIndex map[string]int // Round-robin index per service
	log             *log.Logger
}

type Config struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	Endpoints    []string `yaml:"endpoints"`
	LoadBalancer string   `yaml:"loadBalancer"`
}

type GatewayServiceConfig struct {
	serviceName      string
	loadBalancerType LoadBalancer
	endpoints        []string
}

func NewGatewayServiceConfig(serviceName string, loadBalancerType LoadBalancer, endpoints []string) *GatewayServiceConfig {
	return &GatewayServiceConfig{
		serviceName:      serviceName,
		loadBalancerType: loadBalancerType,
		endpoints:        endpoints,
	}
}

type LoadBalancer interface {
	NextEndpoint() string
}

func NewGateway(ctx context.Context, lock *sync.Mutex, configPath string, log *log.Logger) *Gateway {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to load in-cluster config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create Kubernetes client")
	}

	return &Gateway{
		ctx:             context.Background(),
		watcherChan:     make(chan string),
		lock:            lock,
		configPath:      configPath,
		serviceRegistry: make(map[string]*GatewayServiceConfig),
		clientset:       clientset,
		roundRobinIndex: make(map[string]int),
		servicePods:     make(map[string][]string),
		log:             log,
	}
}

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

func (gateway *Gateway) Run() {
	err := gateway.loadConfig()
	if err != nil {
		gateway.log.Fatalf("Failed to load config: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		gateway.log.Fatalf("Failed to create watcher: %v", err)
	}

	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Has(fsnotify.Write) {
					go gateway.updateServiceConfig(gateway.watcherChan)
					log.Println("modified file:", event.Name)
					gateway.watcherChan <- event.String()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			case <-gateway.ctx.Done():
				gateway.log.Println("Context done, exiting watcher")
				return
			}
		}
	}()

	err = watcher.Add(gateway.configPath)
	if err != nil {
		gateway.log.Fatalf("Failed to add watcher: %v", err)
	}

	gateway.log.Printf("API Gateway listening on :8080")
	// Initialize a new mux router
	mux := http.NewServeMux()
	// Register the route handler
	mux.HandleFunc("/", http.HandlerFunc(gateway.routeHandler))
	if err = http.ListenAndServe(":8080", mux); err != nil {
		gateway.log.Fatalf("Failed to start server: %v", err)
	}
}

func (g *Gateway) updateServiceConfig(chan string) {
	msg := <-g.watcherChan
	g.log.Printf("Received update event for file: %s", msg)

	err := g.loadConfig()
	if err != nil {
		g.log.Fatalf("Failed to load config: %v", err)
	}

	g.log.Printf("Updated Service Registry: %+v\n", g.serviceRegistry)
}

func (g *Gateway) routeHandler(w http.ResponseWriter, r *http.Request) {
	// Log when the request is received
	g.log.Printf("Received request: %s %s", r.Method, r.URL.Path) // Construct the URL and perform the HTTP request
	serviceName := strings.TrimPrefix(r.URL.Path, "/")
	g.log.Printf("Service name: %s", serviceName)

	// check if the service exists in the service registry. If exists, fetch the service from the registry.
	// If not, fetch the service from the load balancer.
	service, exists := g.serviceRegistry[serviceName]
	if !exists {
		g.log.Printf("Service not found: %s", serviceName)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	if service.loadBalancerType == nil {
		g.log.Printf("Load balancer not found for service: %s", serviceName)
		http.Error(w, "Load balancer not found", http.StatusServiceUnavailable)
		return
	}

	g.log.Printf("Service found: %s with endpoints %v", serviceName, service.endpoints)
	url := service.loadBalancerType.NextEndpoint() + r.URL.Path
	g.log.Printf("Forwarding request to: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		// Log if the service is unavailable
		g.log.Printf("Error fetching from service: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Log the response status code
	g.log.Printf("Received response: %d", resp.StatusCode)

	// Copy the response body to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		// Log an error if there's a problem copying the response
		g.log.Printf("Error writing response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
	}
}
