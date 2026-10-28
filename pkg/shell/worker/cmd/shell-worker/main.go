// Command shell-worker serves the shell capability for SDK applications.
//
// Without arguments it runs the supervisor (authenticated HTTP API). The
// supervisor re-executes this binary with the "child" argument for every run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/sandbox"
	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/supervisor"
)

const childArg = "child"

func main() {
	if len(os.Args) > 1 && os.Args[1] == childArg {
		os.Exit(runChild(os.Args[2:]))
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := serve(logger); err != nil {
		logger.Error("shell worker stopped", "error", err)
		os.Exit(1)
	}
}

func runChild(args []string) int {
	fs := flag.NewFlagSet(childArg, flag.ContinueOnError)
	memory := fs.Int64("memory-bytes", 0, "Go heap budget of the run")
	if err := fs.Parse(args); err != nil || *memory <= 0 {
		_, _ = fmt.Fprintln(os.Stderr, "child: -memory-bytes is required")
		return 2
	}
	return sandbox.ServeChild(os.Stdin, os.Stdout, os.Stderr, sandbox.ChildConfig{
		MemoryBytes: *memory,
		Budgets:     sandbox.DefaultBudgets,
	})
}

func serve(logger *slog.Logger) error {
	token := os.Getenv("SHELL_WORKER_TOKEN")
	if err := os.Unsetenv("SHELL_WORKER_TOKEN"); err != nil {
		return err
	}
	queueSize, err := envInt("SHELL_WORKER_QUEUE_SIZE", 4)
	if err != nil {
		return err
	}
	retryAfterMS, err := envInt("SHELL_WORKER_RETRY_AFTER_MS", 1000)
	if err != nil {
		return err
	}
	memory, err := envInt("SHELL_WORKER_CHILD_MEMORY_BYTES", 256<<20)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	sup, err := supervisor.New(supervisor.Config{
		Token: token,
		ChildCommand: []string{
			exe, childArg,
			"-memory-bytes", strconv.Itoa(memory),
		},
		QueueSize:  queueSize,
		RetryAfter: time.Duration(retryAfterMS) * time.Millisecond,
		KillGrace:  time.Second,
	}, logger)
	if err != nil {
		return err
	}

	addr := os.Getenv("SHELL_WORKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           sup.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		IdleTimeout:       time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()
	logger.Info("shell worker listening", "addr", addr, "queue_size", queueSize)

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envInt(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return v, nil
}
