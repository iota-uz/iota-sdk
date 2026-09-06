package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/iota-uz/iota-sdk/sdk-tools/cmd/sdk"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := sdk.NewCommand()
	command.Use = "sdkctl"
	if err := command.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
