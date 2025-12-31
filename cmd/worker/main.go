// Package main provides the Temporal worker entry point
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fraudinvestigation/rag-framework/pkg/workflows"
)

func main() {
	log.Println("Starting Fraud Investigation Worker...")

	// Load configuration from environment
	cfg := loadConfig()

	// Create worker
	worker, err := workflows.NewWorker(cfg)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutdown signal received, stopping worker...")
		cancel()
	}()

	// Run worker
	log.Println("Worker started, listening for tasks...")
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("Worker error: %v", err)
	}

	log.Println("Worker stopped")
}

func loadConfig() *workflows.WorkerConfig {
	cfg := workflows.DefaultWorkerConfig()

	// Override from environment
	if v := os.Getenv("TEMPORAL_HOST"); v != "" {
		cfg.TemporalHost = v
	}
	if v := os.Getenv("TEMPORAL_NAMESPACE"); v != "" {
		cfg.TemporalNamespace = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		cfg.LLMBaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLMModel = v
	}

	return cfg
}
