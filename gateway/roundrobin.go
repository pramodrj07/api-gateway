package gateway

import (
	"log"
	"sync"
)

type RoundRobin struct {
	endpoints []string
	idx       int
	mux       sync.Mutex
	log       *log.Logger
}

// NewRoundRobin initializes a RoundRobin instance with given endpoints.
func NewRoundRobin(endpoints []string, log *log.Logger) *RoundRobin {
	return &RoundRobin{
		endpoints: endpoints,
		idx:       0,
		log:       log,
	}
}

// NextEndpoint returns the next endpoint in a round-robin fashion.
// If there are no endpoints, it returns an empty string.
func (rr *RoundRobin) NextEndpoint() string {
	rr.mux.Lock()
	defer rr.mux.Unlock()

	rr.log.Println("RoundRobin: Total endpoints are", len(rr.endpoints))
	if len(rr.endpoints) == 0 {
		return "" // No endpoints available
	}

	rr.log.Println("RoundRobin: Current index is", rr.idx)

	endpoint := rr.endpoints[rr.idx]
	rr.idx = (rr.idx + 1) % len(rr.endpoints)
	rr.log.Printf("RoundRobin: Next endpoint is: %s", endpoint)
	return endpoint
}

// SetEndpoints allows updating the list of endpoints in a thread-safe manner.
func (rr *RoundRobin) SetEndpoints(endpoints []string) {
	rr.mux.Lock()
	defer rr.mux.Unlock()

	rr.endpoints = endpoints
	rr.idx = 0 // Reset index when endpoints are updated
}
