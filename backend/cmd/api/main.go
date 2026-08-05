// Package api wires all dependencies and starts the HTTP server.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amirhosein/weather-fusion/internal/cache/redis"
	"github.com/amirhosein/weather-fusion/internal/config"
	"github.com/amirhosein/weather-fusion/internal/handlers"
	"github.com/amirhosein/weather-fusion/internal/repositories/postgres"
	"github.com/amirhosein/weather-fusion/pkg/logger"
)

// Run is the application entry point called from main.
func Run() {
	// Load configuration from environment / .env file
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Initialize application logger
	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)
	log.Info("starting weather-fusion", "version", cfg.AppVersion, "env", cfg.AppEnv)

	// Connect to database
	db, err := postgres.Connect(cfg)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database connected")

	// Connect to cache store
	cache, err := redis.NewClient(cfg)
	if err != nil {
		log.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer cache.Close()
	log.Info("redis connected")

	// Setup HTTP router and routes
	router := handlers.NewRouter(cfg, log)

	// Configure HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.AppPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a non-blocking goroutine
	go func() {
		log.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received")

	// Give outstanding requests up to 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", "error", err)
	}
	log.Info("server stopped gracefully")
}
