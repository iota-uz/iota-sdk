package main

import (
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
