package main

import (
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/devhub"
)

func main() {
	hub := devhub.NewDevHub()
	
	if err := hub.Run(); err != nil {
		fmt.Printf("Error running DevHub: %v\n", err)
		os.Exit(1)
	}
}