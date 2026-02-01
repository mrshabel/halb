package main

import (
	"context"
	"flag"
	"fmt"
	"halb/halb"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to config file")
	debug      = flag.Bool("debug", false, "Run in debug mode")
	version    = "1.0.0"
)

func main() {
	flag.Parse()

	// setup logger
	logger := halb.NewLogger(*debug)
	logger.Info("HALB starting", "version", version, "config", *configPath)

	// create config provider
	cfgProvider, err := halb.NewConfig(*configPath)
	if err != nil {
		logger.Error("Failed to create config provider", "error", err.Error())
		os.Exit(1)
	}

	// load initial configuration
	cfg, err := cfgProvider.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err.Error())
		os.Exit(1)
	}

	// context for shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create router
	router := halb.NewRouter(ctx)

	// load routing table
	if err := router.Reload(cfg); err != nil {
		logger.Error("Failed to initialize router", "error", err.Error())
		os.Exit(1)
	}

	// monitor config file for changes
	cfgProvider.Watch(func(newCfg *halb.Config) {
		logger.Info("Config file changed, attempting reload...")

		if err := router.Reload(newCfg); err != nil {
			logger.Error("Failed to reload configuration. Keeping current configuration", "error", err.Error())
		}
	})

	// create http server
	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:     router,
		IdleTimeout: cfg.Server.Timeout,
	}

	doneCh := make(chan struct{})
	go func() {
		// close done channel on exit
		defer close(doneCh)

		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		<-sigCtx.Done()

		stop()
		logger.Info("Shutdown signal received")

		// shutdown server
		serverCtx, serverCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer serverCancel()
		if err := server.Shutdown(serverCtx); err != nil {
			logger.Error("Server forced to shutdown", "error", err.Error())
		}

		// shutdown router
		router.Shutdown()

		// cancel root context
		cancel()
	}()

	logger.Info("Server listening", "port", cfg.Server.Port)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("Server error", "error", err.Error())
		os.Exit(1)
	}

	// detect signal and gracefully shutdown
	<-doneCh
	logger.Info("HALB shutdown complete")
}
