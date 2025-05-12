package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/commands"
)

func main() {
	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Optional: customize log collector with options
	options := []func(*commands.LogCollector){
		commands.WithBatchSize(100),
		commands.WithTimeout(5 * time.Second),
	}

	// Start log collection
	log.Println("Starting log collector...")
	if err := commands.CollectLogs(ctx, options...); err != nil {
		if err != context.Canceled {
			log.Fatalf("Log collector error: %v", err)
		}
		log.Println("Log collector stopped")
	}
}
