// Command api menjalankan HTTP server go-chi-api.
package main

import (
	"apps/go-chi-api/internal/platform/config"
	"apps/go-chi-api/internal/platform/logger"
	"apps/go-chi-api/internal/platform/postgres"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	// load config (fail fast jika ada error)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// setup logger dengan config
	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	// setup graceful shutdown context
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// connect to database
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// setup router
	r := chi.NewRouter()

	// healthcheck end point
	r.Get("/healthz", healthcheckHandler)

	//...

	// setup databasehttp server dengan timeout
	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	// start server in goroutine
	go func() {
		log.Info("listening", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// wait for shutdown signal
	<-ctx.Done()
	log.Info("shutting down")

	// graceful shutdown dengan timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}

	log.Info("server stopped")

}

func healthcheckHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"oke"}`)); err != nil {
		log.Printf("write healthz response: %v", err)
	}
}
