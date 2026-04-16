package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/commands"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/telemetryconfig"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	src, err := config.Build(envprov.New(".env", ".env.local"))
	if err != nil {
		log.Fatalf("failed to build config source: %v", err)
	}

	reg := config.NewRegistry(src)
	cfg, err := config.Register[telemetryconfig.Config](reg, "telemetry")
	if err != nil {
		log.Fatalf("failed to load telemetryconfig: %v", err)
	}

	options := []func(*commands.LogCollector){
		commands.WithBatchSize(100),
		commands.WithTimeout(5 * time.Second),
	}

	log.Println("Starting log collector...")
	if err := commands.CollectLogs(ctx, cfg, options...); err != nil {
		if err != context.Canceled {
			log.Fatalf("Log collector error: %v", err)
		}
		log.Println("Log collector stopped")
	}
}
