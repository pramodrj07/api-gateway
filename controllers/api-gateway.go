package controllers

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// APIGateway holds the service map and provides request routing
type APIGateway struct {
	ServiceMap map[string]string
	Mutex      *sync.RWMutex
}

// NewAPIGateway initializes an APIGateway with a service map reference
func NewAPIGateway(serviceMap map[string]string, mutex *sync.RWMutex) *APIGateway {
	return &APIGateway{
		ServiceMap: serviceMap,
		Mutex:      mutex,
	}
}

// ServeHTTP handles incoming requests and routes them to the appropriate service
func (g *APIGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract the service name from the URL path (e.g., /service-name)
	serviceName := r.URL.Path[1:]

	// Look up the service address
	g.Mutex.RLock()
	address, exists := g.ServiceMap[serviceName]
	g.Mutex.RUnlock()

	if !exists || address == "" {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Forward the request to the service
	url := fmt.Sprintf("http://%s%s", address, r.URL.Path)
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to reach service", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy the response back to the client
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
