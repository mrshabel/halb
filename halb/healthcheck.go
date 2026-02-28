package halb

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHealthCheckTimeout = 3 * time.Second
	defaultUnHealthyThreshold = 3
	defaultHealthyThreshold   = 2
)

type Worker struct {
	backend   *Backend
	healthCfg HealthConfig
	doneCh    chan struct{}
	ctxCancel context.CancelFunc
}

// NewWorker starts a health check worker in the background
func NewWorker(parentCtx context.Context, backend *Backend, healthCfg HealthConfig) *Worker {
	ctx, cancel := context.WithCancel(parentCtx)

	worker := &Worker{
		backend:   backend,
		healthCfg: healthCfg,
		doneCh:    make(chan struct{}),
		ctxCancel: cancel,
	}

	go worker.run(ctx)
	return worker
}

// Stop gracefully shuts down the health checker
func (w *Worker) Stop() {
	// prompt worker to stop
	w.ctxCancel()

	// wait for cleanup
	<-w.doneCh
}

func (w *Worker) run(ctx context.Context) {
	// close channels on exit
	defer close(w.doneCh)

	// run initial check
	w.ping(ctx)

	// start periodic health check
	ticker := time.NewTicker(w.healthCfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ping(ctx)
		}
	}
}

func (w *Worker) ping(ctx context.Context) {
	// compose request
	url := strings.TrimRight(w.backend.URL.String(), "/") + w.healthCfg.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		w.trackFailure()
		return
	}

	client := &http.Client{
		Timeout: defaultHealthCheckTimeout,
		// no redirect following
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		w.trackFailure()
		slog.Debug("Health check failed", "backend", w.backend.URL.String(), "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		w.trackSuccess()
		return
	}

	w.trackFailure()
	slog.Debug("Health check detected unhealthy status", "backend", w.backend.URL.String(), "status", resp.StatusCode)
}

func (w *Worker) trackSuccess() {
	// update success count and reset failures
	w.backend.ConsecutiveSuccessCount.Add(1)
	w.backend.ConsecutiveFailureCount.Store(0)

	// mark as healthy if successive successes detected
	if w.backend.ConsecutiveSuccessCount.Load() >= int32(defaultHealthyThreshold) {
		isHealthy := w.backend.IsHealthy.Load()
		w.backend.IsHealthy.Store(true)

		// transition occurred
		if !isHealthy {
			slog.Debug("Backend became healthy", "backend", w.backend.URL.String(), "consecutive_successes", w.backend.ConsecutiveSuccessCount.Load())
		}
	}
}

func (w *Worker) trackFailure() {
	// update failure count and reset success
	w.backend.ConsecutiveFailureCount.Add(1)
	w.backend.ConsecutiveSuccessCount.Store(0)

	// mark as unhealthy if successive failures detected
	if w.backend.ConsecutiveFailureCount.Load() >= int32(defaultUnHealthyThreshold) {
		isHealthy := w.backend.IsHealthy.Load()
		w.backend.IsHealthy.Store(false)

		// transition occurred
		if isHealthy {
			slog.Debug("Backend became unhealthy", "backend", w.backend.URL.String(), "consecutive_failures", w.backend.ConsecutiveFailureCount.Load())
		}
	}
}
