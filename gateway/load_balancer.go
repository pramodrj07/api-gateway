package main

import "sync"

type LoadBalancer interface {
	NextEndpoint() string
}

// Service holds the load balancer for the service
type Service struct {
	loadBalancer LoadBalancer
}

type RoundRobin struct {
	endpoints []string
	idx       int
	mux       sync.Mutex
}

func (rr *RoundRobin) NextEndpoint() string {
	rr.mux.Lock()
	defer rr.mux.Unlock()
	endpoint := rr.endpoints[rr.idx]
	rr.idx = (rr.idx + 1) % len(rr.endpoints)
	return endpoint
}

// Implementation of least-connections load balancer (for demonstration)
type LeastConnections struct {
	endpoints []string
	connCount map[string]int
	mux       sync.Mutex
}

func (lc *LeastConnections) NextEndpoint() string {
	lc.mux.Lock()
	defer lc.mux.Unlock()

	minConns := int(^uint(0) >> 1)
	selected := ""
	for _, endpoint := range lc.endpoints {
		if lc.connCount[endpoint] < minConns {
			minConns = lc.connCount[endpoint]
			selected = endpoint
		}
	}
	lc.connCount[selected]++
	return selected
}
