package loadbalancer

import (
	"math"
	"sync"
)

type LeastConnections struct {
	endpoints []string
	connCount map[string]int
	mux       sync.Mutex
}

// NewLeastConnections initializes a LeastConnections instance with given endpoints.
func NewLeastConnections(endpoints []string) *LeastConnections {
	connCount := make(map[string]int)
	for _, endpoint := range endpoints {
		connCount[endpoint] = 0
	}
	return &LeastConnections{
		endpoints: endpoints,
		connCount: connCount,
	}
}

// NextEndpoint returns the endpoint with the fewest active connections.
// If no endpoints are available, it returns an empty string.
func (lc *LeastConnections) NextEndpoint() string {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	if len(lc.endpoints) == 0 {
		return ""
	}

	minConns := math.MaxInt32
	var selected string
	for _, endpoint := range lc.endpoints {
		if lc.connCount[endpoint] < minConns {
			minConns = lc.connCount[endpoint]
			selected = endpoint
		}
	}
	lc.connCount[selected]++
	return selected
}

// ReleaseEndpoint decreases the connection count for a given endpoint.
// If the endpoint doesn't exist in the map, it does nothing.
func (lc *LeastConnections) ReleaseEndpoint(endpoint string) {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	if count, exists := lc.connCount[endpoint]; exists && count > 0 {
		lc.connCount[endpoint]--
	}
}

// SetEndpoints allows updating the list of endpoints dynamically in a thread-safe way.
// It resets connection counts for any new endpoints and removes counts for any removed ones.
func (lc *LeastConnections) SetEndpoints(endpoints []string) {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	// Reset connection count for any new endpoints and retain counts for existing ones
	newConnCount := make(map[string]int)
	for _, endpoint := range endpoints {
		if count, exists := lc.connCount[endpoint]; exists {
			newConnCount[endpoint] = count
		} else {
			newConnCount[endpoint] = 0
		}
	}
	lc.endpoints = endpoints
	lc.connCount = newConnCount
}
