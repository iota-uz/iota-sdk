package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultShutdownTimeout bounds how long in-flight requests may run after
	// a shutdown signal. It stays below the common 30s container grace period.
	DefaultShutdownTimeout = 25 * time.Second
	// DefaultStopTimeout bounds the stop hooks that run after the HTTP server
	// has drained.
	DefaultStopTimeout = 10 * time.Second
)

// Lifecycle is a dependency that Run tears down after the HTTP server has
// drained. *bootstrap.Runtime implements it: its Shutdown runs composition stop
// hooks in reverse registration order and then closes the realtime bus and the
// database pool.
type Lifecycle interface {
	Shutdown(ctx context.Context) error
}

// StopFunc is a named teardown step registered with WithStopHook.
type StopFunc func(ctx context.Context) error

type stopHook struct {
	name string
	fn   StopFunc
}

// RunOption configures Run.
type RunOption func(*runOptions)

type runOptions struct {
	signals         []os.Signal
	shutdownTimeout time.Duration
	stopTimeout     time.Duration
	drainDelay      time.Duration
	readinessPaths  []string
	listener        net.Listener
	runtime         Lifecycle
	stopHooks       []stopHook
	onReady         func(addr net.Addr)
}

// WithShutdownTimeout sets how long in-flight requests may take to finish
// after a shutdown signal. When it expires the remaining connections are
// closed. Defaults to DefaultShutdownTimeout.
func WithShutdownTimeout(d time.Duration) RunOption {
	return func(o *runOptions) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

// WithStopTimeout sets the budget for stop hooks and the runtime teardown that
// run after the HTTP server has drained. Defaults to DefaultStopTimeout.
func WithStopTimeout(d time.Duration) RunOption {
	return func(o *runOptions) {
		if d > 0 {
			o.stopTimeout = d
		}
	}
}

// WithDrainDelay keeps accepting requests for d after the shutdown signal while
// readiness paths already answer 503, so a load balancer can stop routing
// traffic before the listener closes.
func WithDrainDelay(d time.Duration) RunOption {
	return func(o *runOptions) {
		if d >= 0 {
			o.drainDelay = d
		}
	}
}

// WithSignals replaces the signals that start a graceful shutdown. Defaults to
// SIGTERM and SIGINT. Passing none leaves shutdown to ctx cancellation only.
func WithSignals(signals ...os.Signal) RunOption {
	return func(o *runOptions) {
		o.signals = append([]os.Signal(nil), signals...)
	}
}

// WithReadinessPaths replaces the paths that answer 503 once shutdown begins.
// Defaults to "/health".
func WithReadinessPaths(paths ...string) RunOption {
	return func(o *runOptions) {
		o.readinessPaths = append([]string(nil), paths...)
	}
}

// WithListener serves on ln instead of listening on the address passed to Run.
func WithListener(ln net.Listener) RunOption {
	return func(o *runOptions) {
		o.listener = ln
	}
}

// WithRuntime registers the runtime whose Shutdown runs after every stop hook
// registered with WithStopHook. server.New sets it to the bootstrap runtime.
func WithRuntime(rt Lifecycle) RunOption {
	return func(o *runOptions) {
		o.runtime = rt
	}
}

// WithStopHook registers a teardown step that runs after the HTTP server has
// drained. Stop hooks run in reverse registration order, before the runtime
// teardown (WithRuntime), with a shared WithStopTimeout budget.
func WithStopHook(name string, fn StopFunc) RunOption {
	return func(o *runOptions) {
		if fn != nil {
			o.stopHooks = append(o.stopHooks, stopHook{name: name, fn: fn})
		}
	}
}

// WithOnReady is called once the listener is open, with its address.
func WithOnReady(fn func(addr net.Addr)) RunOption {
	return func(o *runOptions) {
		o.onReady = fn
	}
}

// Run serves HTTP on addr until ctx is cancelled or a shutdown signal (SIGTERM,
// SIGINT by default) arrives, then shuts the process down gracefully:
//
//  1. readiness paths start answering 503 and, after the optional drain delay,
//     the listener closes;
//  2. in-flight requests finish within the shutdown timeout; connections still
//     open after it are closed;
//  3. stop hooks registered with WithStopHook run in reverse registration order;
//  4. the runtime (WithRuntime) shuts down: its lifecycle context is cancelled,
//     composition stop hooks run in reverse order, then the realtime bus and
//     the database pool close.
//
// A second signal during shutdown terminates the process immediately. Run
// returns nil after a clean shutdown and the joined errors otherwise.
func (s *HTTPServer) Run(ctx context.Context, addr string, opts ...RunOption) error {
	cfg := runOptions{
		signals:         []os.Signal{syscall.SIGTERM, syscall.SIGINT},
		shutdownTimeout: DefaultShutdownTimeout,
		stopTimeout:     DefaultStopTimeout,
		readinessPaths:  []string{"/health"},
	}
	for _, opt := range s.runDefaults {
		opt(&cfg)
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	logger := s.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	runCtx, stopSignals := ctx, context.CancelFunc(func() {})
	if len(cfg.signals) > 0 {
		runCtx, stopSignals = signal.NotifyContext(ctx, cfg.signals...)
	}
	defer stopSignals()

	// Build the router before binding, so the port only opens once every
	// controller has registered.
	s.draining.Store(false)
	srv := s.newHTTPServer(addr, cfg.readinessPaths)

	ln := cfg.listener
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return errors.Join(fmt.Errorf("server: listen on %s: %w", addr, err), s.teardown(cfg, logger))
		}
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()
	logger.WithField("addr", ln.Addr().String()).Info("http server listening")
	if cfg.onReady != nil {
		cfg.onReady(ln.Addr())
	}

	var runErr error
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("server: serve: %w", err)
		}
		return errors.Join(runErr, s.teardown(cfg, logger))
	case <-runCtx.Done():
	}
	// Restore default signal handling: a second signal kills the process.
	stopSignals()

	logger.WithFields(logrus.Fields{
		"cause":            context.Cause(runCtx),
		"shutdown_timeout": cfg.shutdownTimeout,
		"drain_delay":      cfg.drainDelay,
	}).Info("graceful shutdown started")
	s.draining.Store(true)

	if cfg.drainDelay > 0 {
		timer := time.NewTimer(cfg.drainDelay)
		select {
		case <-timer.C:
		case err := <-serveErr:
			timer.Stop()
			serveErr <- err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.shutdownTimeout)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Warn("in-flight requests did not finish before the shutdown timeout; closing connections")
		runErr = errors.Join(runErr, fmt.Errorf("server: shutdown: %w", err), srv.Close())
	}
	cancel()
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		runErr = errors.Join(runErr, fmt.Errorf("server: serve: %w", err))
	}
	logger.Info("http server stopped")

	runErr = errors.Join(runErr, s.teardown(cfg, logger))
	if runErr == nil {
		logger.Info("graceful shutdown complete")
	}
	return runErr
}

// teardown runs stop hooks in reverse registration order, then the runtime.
func (s *HTTPServer) teardown(cfg runOptions, logger *logrus.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.stopTimeout)
	defer cancel()

	var err error
	for i := len(cfg.stopHooks) - 1; i >= 0; i-- {
		hook := cfg.stopHooks[i]
		if hookErr := hook.fn(ctx); hookErr != nil {
			logger.WithError(hookErr).WithField("hook", hook.name).Warn("stop hook failed")
			err = errors.Join(err, fmt.Errorf("stop hook %q: %w", hook.name, hookErr))
		}
	}
	if cfg.runtime != nil {
		if rtErr := cfg.runtime.Shutdown(ctx); rtErr != nil {
			logger.WithError(rtErr).Warn("runtime shutdown failed")
			err = errors.Join(err, fmt.Errorf("runtime shutdown: %w", rtErr))
		}
	}
	return err
}
