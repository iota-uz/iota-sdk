package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/iota-uz/iota-sdk/pkg/devhub"
)

func main() {
	configPath := flag.String("config", "devhub.yml", "Path to the devhub.yml config file")
	flag.Parse()

	hub, err := devhub.NewDevHub(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DevHub: %v\n", err)
		os.Exit(1)
	}

	// Create a context that cancels on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	if err := hub.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running DevHub: %v\n", err)
		os.Exit(1)
	}
}
