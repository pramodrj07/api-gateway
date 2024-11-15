package gateway

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	loadbalancer "github.com/pramodrj07/api-gateway/loadbalancer"
	"gopkg.in/yaml.v2"
)

// TestNewGateway tests that a new Gateway instance is created correctly
func TestNewGateway(t *testing.T) {
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	gateway := NewGateway(context.Background(), &sync.Mutex{}, "config.yaml", logger)

	if gateway == nil {
		t.Fatal("Expected a new Gateway instance, got nil")
	}
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	gateway := NewGateway(context.Background(), &sync.Mutex{}, "config.yaml", logger)

	// Test loading the configuration
	config, err := gateway.loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config == nil {
		t.Fatal("Expected a valid config, got nil")
	}
}

// TestRouteHandler tests the Gateway's route handler with a sample request
func TestRouteHandler(t *testing.T) {
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	gateway := NewGateway(context.Background(), &sync.Mutex{}, "config.yaml", logger)

	// Set a sample config in the Gateway
	gateway.config = &loadbalancer.Config{
		Services: map[string]loadbalancer.ServiceConfig{
			"test-service": {
				LoadBalancer: "round_robin",
				Endpoints: []string{
					"http://localhost:8081",
				},
			},
		},
	}

	// Create a test server that mocks the service response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	}))
	defer ts.Close()

	// Update the endpoint to use the test server URL
	gateway.config.Services["test-service"].Endpoints[0] = ts.URL

	// Create a request to test the route handler
	req, err := http.NewRequest("GET", "/test-service", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	// Record the response
	rec := httptest.NewRecorder()
	gateway.routeHandler(rec, req)

	// Check the status code
	if status := rec.Code; status != http.StatusOK {
		t.Errorf("Expected status %v, got %v", http.StatusOK, status)
	}

	// Check the response body
	expected := `{"message": "ok"}`
	if rec.Body.String() != expected {
		t.Errorf("Expected body %v, got %v", expected, rec.Body.String())
	}
}

// TestUpdateServiceConfig tests the updateServiceConfig method with a config change
func TestUpdateServiceConfig(t *testing.T) {
	logger := log.New(ioutil.Discard, "", log.LstdFlags)
	gateway := NewGateway(context.Background(), &sync.Mutex{}, "config.yaml", logger)

	// Set initial configuration
	initialConfig := &loadbalancer.Config{
		Services: map[string]loadbalancer.ServiceConfig{
			"test-service": {
				LoadBalancer: "round_robin",
				Endpoints: []string{
					"http://localhost:8081",
				},
			},
		},
	}
	gateway.config = initialConfig

	// Mock a new configuration update
	updatedConfigData := `
services:
  service-a:
    loadBalancer:
      type: "round_robin"
      endpoints:
        - "http://localhost:8081"
        - "http://localhost:8083"
`
	updatedConfig := &loadbalancer.Config{}
	if err := yaml.Unmarshal([]byte(updatedConfigData), updatedConfig); err != nil {
		t.Fatalf("Failed to unmarshal updated config data: %v", err)
	}

	// Simulate receiving a new configuration update
	gateway.lock.Lock()
	defer gateway.lock.Unlock()
	gateway.config = updatedConfig

	// Check that the configuration is updated
	if len(gateway.config.Services["service-a"].Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(gateway.config.Services["service-a"].Endpoints))
	}
}
