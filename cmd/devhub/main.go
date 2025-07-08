package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/iota-uz/iota-sdk/pkg/devhub"
	"github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "devhub.yml", "Path to the devhub.yml config file")
	showVersion := flag.Bool("version", false, "Show version information")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	if *showVersion {
		_, _ = fmt.Fprintf(os.Stdout, "DevHub CLI\nVersion: %s\nCommit: %s\nBuilt: %s\n", version, commit, date)
		os.Exit(0)
	}

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Warn("Invalid log level, defaulting to info")
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	hub, err := devhub.NewDevHub(*configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create DevHub")
	}

	// Create a context that cancels on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	if err := hub.Run(ctx); err != nil {
		logrus.WithError(err).Fatal("DevHub exited with error")
	}

	logrus.Info("DevHub shut down successfully")
}
