// testenv-scenario prepares, disposes, exports and replays scenarios of an
// isolated test environment over its HTTP control routes.
//
//	go run ./cmd/testenv-scenario --base-url http://127.0.0.1:PORT \
//	  --environment-id ENV --control-token-file TOKEN prepare --scope ENV-a < input.json
//
// Consumers that need in-process preparation call cli.Run with testenv.Local.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/iota-uz/iota-sdk/pkg/testenv/cli"
)

func main() {
	if err := run(); err != nil {
		var typed *testenv.Error
		if errors.As(err, &typed) {
			encoded, _ := json.Marshal(typed)
			fmt.Fprintln(os.Stderr, string(encoded))
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func run() error {
	baseURL := flag.String("base-url", os.Getenv("TESTENV_BASE_URL"), "environment base URL")
	environmentID := flag.String("environment-id", os.Getenv("TESTENV_ENVIRONMENT_ID"), "expected environment ID")
	tokenFile := flag.String("control-token-file", "", "local file containing the control token")
	flag.Parse()
	if *tokenFile == "" {
		return errors.New("--control-token-file is required")
	}
	token, err := os.ReadFile(*tokenFile)
	if err != nil {
		return err
	}
	client, err := testenv.NewClient(*baseURL, strings.TrimSpace(string(token)), *environmentID)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return cli.Run(ctx, client, flag.Args(), os.Stdin, os.Stdout)
}
