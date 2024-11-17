package gateway

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

// Mock LoadBalancer implementation
type MockLoadBalancer struct {
	endpoints []string
	index     int
}

func (m *MockLoadBalancer) NextEndpoint() string {
	if len(m.endpoints) == 0 {
		return ""
	}
	endpoint := m.endpoints[m.index]
	m.index = (m.index + 1) % len(m.endpoints)
	return endpoint
}

// Helper function to create a test gateway
func createTestGateway(configPath string) *Gateway {
	logger := log.New(os.Stdout, "test: ", log.Lshortfile)
	ctx := context.Background()
	lock := &sync.Mutex{}
	return NewGateway(ctx, lock, configPath, logger)
}

// Test NewGateway initialization
func TestNewGateway(t *testing.T) {
	g := createTestGateway("test_config.yaml")
	if g == nil {
		t.Fatalf("Failed to create Gateway")
	}
	if g.configPath != "test_config.yaml" {
		t.Errorf("Expected configPath to be 'test_config.yaml', got %s", g.configPath)
	}
}

// Test loadConfig with valid YAML
func TestLoadConfig_ValidYAML(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "config.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
services:
  service1:
    endpoints:
      - http://localhost:8081
      - http://localhost:8082
    loadBalancer: round-robin
`
	_, err = tmpFile.WriteString(configContent)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	g := createTestGateway(tmpFile.Name())

	err = g.loadConfig()
	if err != nil {
		t.Errorf("Unexpected error during loadConfig: %v", err)
	}

	if len(g.serviceRegistry) != 1 {
		t.Errorf("Expected 1 service in serviceRegistry, got %d", len(g.serviceRegistry))
	}

	service, exists := g.serviceRegistry["service1"]
	if !exists {
		t.Errorf("Expected service1 in serviceRegistry")
	}

	if len(service.endpoints) != 2 {
		t.Errorf("Expected 2 endpoints for service1, got %d", len(service.endpoints))
	}
}

// Test loadConfig with invalid YAML
func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "config.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("invalid_yaml")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	g := createTestGateway(tmpFile.Name())

	err = g.loadConfig()
	if err == nil {
		t.Errorf("Expected error during loadConfig with invalid YAML, got nil")
	}
}

// Test routeHandler for valid service
func TestRouteHandler_ValidService(t *testing.T) {
	mockLoadBalancer := &MockLoadBalancer{endpoints: []string{"http://localhost:8081"}}
	g := createTestGateway("")

	g.serviceRegistry["service1"] = &GatewayServiceConfig{
		serviceName:      "service1",
		loadBalancerType: mockLoadBalancer,
		endpoints:        []string{"http://localhost:8081"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"success"}`))
	}))
	defer server.Close()

	g.serviceRegistry["service1"].endpoints = []string{server.URL}
	mockLoadBalancer.endpoints = []string{server.URL}

	req := httptest.NewRequest("GET", "/service1", nil)
	w := httptest.NewRecorder()

	g.routeHandler(w, req)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %d", resp.StatusCode)
	}

	if string(body) != `{"message":"success"}` {
		t.Errorf("Expected response body to be 'success', got %s", string(body))
	}
}

// Test routeHandler for non-existent service
func TestRouteHandler_NonExistentService(t *testing.T) {
	g := createTestGateway("")

	req := httptest.NewRequest("GET", "/unknown-service", nil)
	w := httptest.NewRecorder()

	g.routeHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status NotFound, got %d", resp.StatusCode)
	}
}
