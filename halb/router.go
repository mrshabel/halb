package halb

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultIdleConns        = 200
	defaultIdleConnsPerHost = 20
	defaultIdleConnTimeout  = 60 * time.Minute
)

type Router struct {
	// lock-free routing table of hostname mapping to server pools
	table atomic.Value

	workers []*Worker
	// lock for coordinating health-check workers
	mu     sync.RWMutex
	ctx    context.Context
	doneCh chan struct{}
}

// ServerPool holds backend servers and selection strategy
type ServerPool struct {
	Backends []*Backend
	Strategy LoadBalancingStrategy
	// round robin index
	CurrentRRIndex atomic.Uint32
	HealthCheck    HealthConfig
}

// Backend is a live backend server
type Backend struct {
	URL                     *url.URL
	Proxy                   *httputil.ReverseProxy
	IsHealthy               atomic.Bool
	Weight                  int
	ActiveConns             atomic.Int64
	ConsecutiveFailureCount atomic.Int32
	ConsecutiveSuccessCount atomic.Int32
}

type routingTable struct {
	routes map[string]*ServerPool
}

func NewRouter(ctx context.Context) *Router {
	router := &Router{ctx: ctx}
	table := &routingTable{routes: make(map[string]*ServerPool)}
	router.table.Store(table)
	return router
}

// Reload atomically swaps the routing configuration
func (r *Router) Reload(cfg *Config) error {
	// lock router to prevent other reloads for now
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Debug("Reloading configuration", slog.Int("services", len(cfg.Services)))

	// stop all existing health check workers
	for _, worker := range r.workers {
		worker.Stop()
	}

	// build new routing table
	table := &routingTable{routes: make(map[string]*ServerPool)}
	workers := []*Worker{}

	for name, service := range cfg.Services {
		pool := &ServerPool{
			Strategy:    service.Strategy,
			HealthCheck: service.Health,
		}

		// create backend servers
		for _, server := range service.Servers {
			backendURL, _ := url.Parse(server)

			// TODO: add dynamic weights
			backend := &Backend{
				URL:    backendURL,
				Weight: 1,
			}
			backend.IsHealthy.Store(true)

			// setup reverse proxy with custom transport
			backend.Proxy = &httputil.ReverseProxy{
				Director: func(r *http.Request) {
					// replace request url details with that of the upstream server
					r.URL.Scheme = backendURL.Scheme
					r.URL.Host = backendURL.Host
					r.Host = backendURL.Host

					// add forwarding headers
					if ip := getClientIP(r); ip != "" {
						r.Header.Set("X-Forwarded-For", ip)
					}
					r.Header.Set("X-Forwarded-Proto", backendURL.Scheme)
					r.Header.Set("X-Forwarded-Host", r.Host)
				},

				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					slog.Error("proxy error", "backend", backendURL.String(), "path", r.URL.Path, "error", err.Error())

					// mark as failed
					backend.ConsecutiveFailureCount.Add(1)
					w.WriteHeader(http.StatusBadGateway)
					fmt.Fprintf(w, "Bad Gateway: %v", err)
				},

				// setup transport with keepalive enabled
				Transport: &http.Transport{
					MaxIdleConns:        defaultIdleConns,
					MaxIdleConnsPerHost: defaultIdleConnsPerHost,
					IdleConnTimeout:     defaultIdleConnTimeout,
					DisableKeepAlives:   false,
					DisableCompression:  false,
				},
			}

			pool.Backends = append(pool.Backends, backend)

			// start health check if enabled
			if service.Health.Enabled() {
				worker := NewWorker(r.ctx, backend, service.Health)
				workers = append(workers, worker)
			}
		}

		host := extractHost(service.Host)
		table.routes[host] = pool

		slog.Info("service configured", "name", name, "host", host, "strategy", service.Strategy, "backends", len(pool.Backends))
	}

	// swap routing table atomically
	r.table.Store(table)
	r.workers = workers

	slog.Info("configuration reload complete", "services", len(table.routes))

	return nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// load configuration
	start := time.Now()

	// get current routing table
	table := r.table.Load().(*routingTable)

	// extract host
	host := extractHost(req.Host)

	// find matching service
	pool, ok := table.routes[host]
	if !ok {
		slog.Warn("No route found", "host", host, "original_host", req.Host, "remote_addr", req.RemoteAddr)

		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "No service configured for host: %q", host)
		return
	}

	// select backend
	var backend *Backend
	switch pool.Strategy {
	case RoundRobin:
		backend = RoundRobinRouter(pool)
	case LeastConn:
		backend = LeastConnRouter(pool)
	default:
		backend = RoundRobinRouter(pool)
	}

	if backend == nil {
		slog.Error("no healthy backend servers available",
			slog.String("host", host),
			slog.Int("total_backends", len(pool.Backends)),
		)

		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "No healthy backend servers available for: %s", host)
		return
	}

	// track active connections
	backend.ActiveConns.Add(1)
	defer backend.ActiveConns.Add(-1)

	// capture response status and proxy request to upstream server
	res := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	backend.Proxy.ServeHTTP(res, req)

	slog.Info("request",
		slog.String("ip", getClientIP(req)),
		slog.String("host", host),
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.String("backend", backend.URL.String()),
		slog.Int("status", res.status),
		slog.Int64("latency", time.Since(start).Milliseconds()),
	)
}

// Shutdown gracefully stops all health checkers
func (r *Router) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("shutting down health checkers", "count", len(r.workers))
	for _, worker := range r.workers {
		worker.Stop()
	}
}
