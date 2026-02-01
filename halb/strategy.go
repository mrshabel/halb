package halb

import "math"

var (
	inf = int64(math.MaxInt64)
)

// RoundRobinRouter selects the next healthy backend server in a circular order
func RoundRobinRouter(pool *ServerPool) *Backend {
	backends := getHealthyBackends(pool)
	if len(backends) == 0 {
		return nil
	}

	// get next available backend while updating index
	idx := pool.CurrentRRIndex.Add(1) % uint32(len(backends))
	return backends[idx]
}

// LeastConn selects the next healthy backend server with the fewest active connections
func LeastConnRouter(pool *ServerPool) *Backend {
	var candidate *Backend
	minConns := inf

	for _, backend := range pool.Backends {
		if !backend.IsHealthy.Load() {
			continue
		}

		// load connections
		conns := backend.ActiveConns.Load()

		if conns < minConns {
			minConns = conns
			candidate = backend
		}
	}

	return candidate
}

// getHealthyBackends retrieves the healthy backends from the server pool
func getHealthyBackends(pool *ServerPool) []*Backend {
	var healthy []*Backend
	for _, backend := range pool.Backends {
		if !backend.IsHealthy.Load() {
			continue
		}
		healthy = append(healthy, backend)
	}

	return healthy
}
