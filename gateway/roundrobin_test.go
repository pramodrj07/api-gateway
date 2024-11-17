package gateway

import (
	"testing"

	"go.uber.org/zap"
)

func TestSingleEndpoint(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	rr := NewRoundRobin([]string{"http://example.com"}, logger)
	for i := 0; i < 10; i++ {
		endpoint := rr.NextEndpoint()
		if endpoint != "http://example.com" {
			t.Errorf("Expected http://example.com, but got %s", endpoint)
		}
	}
}

func TestMultipleEndpoints(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	rr := NewRoundRobin([]string{"http://example1.com", "http://example2.com", "http://example3.com"}, logger)
	expectedEndpoints := []string{
		"http://example1.com", "http://example2.com", "http://example3.com",
		"http://example1.com", "http://example2.com", "http://example3.com",
	}

	for i, expected := range expectedEndpoints {
		endpoint := rr.NextEndpoint()
		if endpoint != expected {
			t.Errorf("Test case %d: Expected %s, but got %s", i, expected, endpoint)
		}
	}
}

func TestNoEndpoints(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	rr := NewRoundRobin([]string{}, logger)
	endpoint := rr.NextEndpoint()
	if endpoint != "" {
		t.Errorf("Expected empty string, but got %s", endpoint)
	}
}

func TestUpdateEndpoints(t *testing.T) {
	loggerConfig := zap.NewProductionConfig()
	logger, _ := loggerConfig.Build()
	rr := NewRoundRobin([]string{"http://example1.com", "http://example2.com"}, logger)
	rr.NextEndpoint() // Move index forward once

	// Update endpoints
	rr.SetEndpoints([]string{"http://new1.com", "http://new2.com", "http://new3.com"})

	expectedEndpoints := []string{"http://new1.com", "http://new2.com", "http://new3.com", "http://new1.com"}
	for i, expected := range expectedEndpoints {
		endpoint := rr.NextEndpoint()
		if endpoint != expected {
			t.Errorf("Test case %d: Expected %s, but got %s", i, expected, endpoint)
		}
	}
}
