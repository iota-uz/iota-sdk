package main

import (
	"flag"
	"fmt"
	"os"

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

	if err := hub.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running DevHub: %v\n", err)
		os.Exit(1)
	}
}
