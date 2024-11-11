package gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"

	loadbalancer "github.com/pramodrj07/api-gateway/loadbalancer"
)

type Gateway struct {
	ctx         context.Context
	watcherChan chan string
	config      *loadbalancer.Config
	lock        *sync.Mutex
	log         *log.Logger
}

func NewGateway(ctx context.Context, lock *sync.Mutex, log *log.Logger) *Gateway {
	return &Gateway{
		ctx:         context.Background(),
		watcherChan: make(chan string),
		lock:        lock,
		config:      nil,
		log:         log,
	}
}

func (g *Gateway) loadConfig(path string) (*loadbalancer.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config loadbalancer.Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (gateway *Gateway) Run() {
	var err error

	config, err := gateway.loadConfig("/workspaces/api-gateway/configuration/config.yaml")
	if err != nil {
		gateway.log.Fatalf("Failed to load config: %v", err)
	}

	gateway.log.Println("Configuration loaded")

	// Initialize the service registry
	gateway.config = config

	gateway.log.Printf("Configuration: %+v\n", gateway.config)
	// Start config watcher for periodic reloads
	// go gateway.watchConfig(gateway.watcherChan)

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

	err = watcher.Add("/workspaces/api-gateway/configuration/config.yaml")
	if err != nil {
		gateway.log.Fatalf("Failed to add watcher: %v", err)
	}

	fmt.Println("API Gateway listening on :8080")
	// Initialize a new mux router
	mux := http.NewServeMux()
	// Register the route handler
	mux.HandleFunc("/", http.HandlerFunc(gateway.routeHandler))
	if err = http.ListenAndServe(":8088", mux); err != nil {
		gateway.log.Fatalf("Failed to start server: %v", err)
	}
}

func (g *Gateway) updateServiceConfig(chan string) {
	msg := <-g.watcherChan
	g.log.Printf("Received update event for file: %s", msg)

	updatedConfig, err := g.loadConfig("/workspaces/api-gateway/configuration/config.yaml")
	if err != nil {
		g.log.Fatalf("Failed to load config: %v", err)
	}

	if !reflect.DeepEqual(updatedConfig, g.config) {
		g.lock.Lock()
		defer g.lock.Unlock()
		g.config = updatedConfig
		g.log.Printf("New Configuration detected. Updated Configuration is: %+v", g.config)
		return
	}

	g.log.Printf("Updated configuration: %+v", g.config)
}

// Route handler
func (g *Gateway) routeHandler(w http.ResponseWriter, r *http.Request) {
	// Log when the request is received
	g.log.Printf("Received request: %s %s", r.Method, r.URL.Path) // Construct the URL and perform the HTTP request
	serviceName := strings.TrimPrefix(r.URL.Path, "/")
	g.log.Printf("Service name: %s", serviceName)
	service, err := loadbalancer.GetService(g.lock, *g.config, serviceName)
	if err != nil {
		// Log if the service is not found
		// g.log.Printf("Service not found: %v", err)
		// http.Error(w, "Service not found or unsupported load balancer type", http.StatusNotFound)
		// return
		g.log.Printf("Error fetching service: %v", err.Error())
		switch err.Error() {
		case "service not found":
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		case "unsupported load balancer type":
			http.Error(w, "Unsupported load balancer type", http.StatusNotImplemented)
			return
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
	url := service.LoadBalancer.NextEndpoint() + r.URL.Path
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
