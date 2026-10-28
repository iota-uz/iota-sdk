package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"runtime/metrics"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
)

// MemoryExitCode is the child exit status for an exhausted memory budget.
const MemoryExitCode = 3

// ChildConfig is fixed by the supervisor through child flags, never by the guest.
type ChildConfig struct {
	MemoryBytes int64
	Budgets     Budgets
}

// ServeChild runs exactly one request read from stdin and writes the result to
// stdout. Hard limits are applied before the request is read, so no guest code
// runs without them.
func ServeChild(stdin io.Reader, stdout, stderr io.Writer, cfg ChildConfig) int {
	if err := applyLimits(); err != nil {
		_, _ = fmt.Fprintln(stderr, "apply limits:", err)
		return 1
	}
	debug.SetMemoryLimit(cfg.MemoryBytes)
	go watchMemory(cfg.MemoryBytes)

	var req protocol.Request
	if err := json.NewDecoder(io.LimitReader(stdin, protocol.MaxRequestBytes)).Decode(&req); err != nil {
		_, _ = fmt.Fprintln(stderr, "decode request:", err)
		return 1
	}
	if verr := req.Validate(); verr != nil {
		_, _ = fmt.Fprintln(stderr, "invalid request:", verr)
		return 1
	}
	res, err := Run(context.Background(), &req, cfg.Budgets)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "run:", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(res); err != nil {
		_, _ = fmt.Fprintln(stderr, "encode result:", err)
		return 1
	}
	return 0
}

// watchMemory terminates the child once the Go heap outgrows the budget. It
// complements the container memory limit, which ends the process even when an
// allocation outruns this check.
func watchMemory(limit int64) {
	samples := []metrics.Sample{
		{Name: "/memory/classes/total:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
	}
	for range time.Tick(10 * time.Millisecond) {
		metrics.Read(samples)
		if int64(samples[0].Value.Uint64()-samples[1].Value.Uint64()) > limit {
			os.Exit(MemoryExitCode)
		}
	}
}
