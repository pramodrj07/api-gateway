package gateway

import (
	"testing"

	"go.uber.org/zap"
)

func TestSingleEndpointForLeastConn(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	lc := NewLeastConnections([]string{"http://example.com"}, logger)
	for i := 0; i < 10; i++ {
		endpoint := lc.NextEndpoint()
		if endpoint != "http://example.com" {
			t.Errorf("Expected http://example.com, but got %s", endpoint)
		}
	}
}

func TestMultipleEndpointsWithLeastConnections(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	lc := NewLeastConnections([]string{"http://example1.com", "http://example2.com"}, logger)

	// Initial selection should be balanced
	endpoint1 := lc.NextEndpoint()
	endpoint2 := lc.NextEndpoint()

	if endpoint1 != "http://example1.com" && endpoint2 != "http://example2.com" {
		t.Errorf("Expected one of each endpoint, got %s and %s", endpoint1, endpoint2)
	}

	// Simulate connections by calling NextEndpoint, which increases the count
	lc.NextEndpoint() // Assigns to "http://example1.com" (increased load)
	selected := lc.NextEndpoint()
	if selected != "http://example2.com" {
		t.Errorf("Expected http://example2.com as it has fewer connections, but got %s", selected)
	}
}

func TestConnectionRelease(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	lc := NewLeastConnections([]string{"http://example1.com", "http://example2.com"}, logger)

	// Simulate connections
	lc.NextEndpoint() // endpoint1
	lc.NextEndpoint() // endpoint2
	lc.NextEndpoint() // endpoint1

	// Now release a connection from endpoint1
	lc.ReleaseEndpoint("http://example1.com")

	// Ensure the connection count is decremented
	if lc.connCount["http://example1.com"] != 1 {
		t.Errorf("Expected 1 connection for http://example1.com, but got %d", lc.connCount["http://example1.com"])
	}

	// endpoint1 should have fewer connections again
	if lc.connCount["http://example1.com"] != 1 {
		t.Errorf("Expected 1 connection for http://example1.com, but got %d", lc.connCount["http://example1.com"])
	}
	if lc.connCount["http://example2.com"] != 1 {
		t.Errorf("Expected 1 connection for http://example2.com, but got %d", lc.connCount["http://example2.com"])
	}
}

func TestNoEndpointsForLeastConn(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	lc := NewLeastConnections([]string{}, logger)
	endpoint := lc.NextEndpoint()
	if endpoint != "" {
		t.Errorf("Expected empty string, but got %s", endpoint)
	}
}

func TestUpdateEndpointsForLeastConn(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	lc := NewLeastConnections([]string{"http://example1.com", "http://example2.com"}, logger)

	// Simulate some connections
	lc.NextEndpoint() // example1
	lc.NextEndpoint() // example2

	// Update endpoints to a new list
	lc.SetEndpoints([]string{"http://new1.com", "http://new2.com", "http://new3.com"})

	// Check that the connection count for the new endpoints is reset
	for _, endpoint := range lc.endpoints {
		if lc.connCount[endpoint] != 0 {
			t.Errorf("Expected 0 connections for new endpoint %s, but got %d", endpoint, lc.connCount[endpoint])
		}
	}
}
