// Package viewmodels provides this package.
package viewmodels

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SystemInfoBuilderOptions carries the dependencies DefaultBuildViewModel
// needs. Pool and Capabilities are sufficient for a useful page; the rest
// refine it with environment / build metadata when available.
type SystemInfoBuilderOptions struct {
	// Pool powers the database connection stats. nil → pool fields stay zero.
	Pool *pgxpool.Pool
	// Capabilities is consulted on every request so probes reflect live state.
	// nil → Capabilities slice is empty.
	Capabilities health.CapabilityService
	// App supplies Environment. nil → Environment stays empty.
	App *appconfig.Config
	// Version, BranchName, GitCommit, GitCommitURL, BuildTime, BuildEnvironment,
	// BuildGoVersion are typically injected via ld flags by the downstream
	// binary and passed through here. Empty strings render as "unknown" in the
	// template.
	Version          string
	BranchName       string
	GitCommit        string
	GitCommitURL     string
	BuildTime        string
	BuildEnvironment string
	BuildGoVersion   string
}

// processStart captures boot time so Uptime survives across requests. It is
// set at package init so the zero value of a subsequent Runtime restart
// reflects actual process age, not container lifetime.
var processStart = time.Now()

// DefaultBuildViewModel returns a BuildViewModel function suitable for
// HealthUIControllerOptions. It fills in the metrics derivable from the Go
// runtime + process environment and populates the Capabilities slice from
// the attached CapabilityService.
//
// Host-level stats (CPU %, total memory, disk) require a platform-specific
// library (gopsutil). This default intentionally leaves those fields at
// zero to avoid adding a CGO dependency that breaks musl builds. Downstream
// binaries that want those numbers supply their own BuildViewModel.
func DefaultBuildViewModel(opts SystemInfoBuilderOptions) func(context.Context, *http.Request) (*SystemInfoViewModel, error) {
	return func(ctx context.Context, r *http.Request) (*SystemInfoViewModel, error) {
		metrics := buildRuntimeMetrics(opts)
		caps := fetchCapabilities(ctx, opts.Capabilities)
		return NewSystemInfoViewModel(metrics, caps, r), nil
	}
}

// buildRuntimeMetrics snapshots the go runtime + pgxpool + host basics.
func buildRuntimeMetrics(opts SystemInfoBuilderOptions) *SystemInfoMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	hostname, _ := os.Hostname()

	metrics := &SystemInfoMetrics{
		GoVersion:        runtime.Version(),
		GOOS:             runtime.GOOS,
		GOARCH:           runtime.GOARCH,
		NumCPU:           runtime.NumCPU(),
		NumGoroutines:    runtime.NumGoroutine(),
		HeapAlloc:        mem.HeapAlloc,
		HeapSys:          mem.HeapSys,
		NumGC:            mem.NumGC,
		LastPauseNs:      mem.PauseNs[(mem.NumGC+255)%256],
		GCCPUFraction:    mem.GCCPUFraction,
		Uptime:           time.Since(processStart),
		Hostname:         hostname,
		CurrentTime:      time.Now(),
		Version:          opts.Version,
		BranchName:       opts.BranchName,
		GitCommit:        opts.GitCommit,
		GitCommitURL:     opts.GitCommitURL,
		BuildTime:        opts.BuildTime,
		BuildEnvironment: opts.BuildEnvironment,
		BuildGoVersion:   opts.BuildGoVersion,
	}

	if tz, offset := time.Now().Zone(); tz != "" {
		metrics.Timezone = tz
		metrics.TimezoneOffset = formatTimezoneOffset(offset)
	}

	if opts.App != nil {
		metrics.Environment = opts.App.Environment
	}

	if opts.Pool != nil {
		stat := opts.Pool.Stat()
		metrics.PoolActiveConns = stat.AcquiredConns()
		metrics.PoolIdleConns = stat.IdleConns()
		metrics.PoolTotalConns = stat.TotalConns()
		metrics.PoolMaxConns = stat.MaxConns()
		metrics.PoolAcquireCount = stat.AcquireCount()
		metrics.PoolNewConnsCount = stat.NewConnsCount()
		metrics.PoolAcquireDuration = stat.AcquireDuration()
	}

	return metrics
}

// fetchCapabilities returns the probe snapshot for svc, or nil if svc is nil.
// Probes self-heal via the capability service's safe-probe wrapper so a
// panicking probe yields a down-status entry instead of killing the request.
func fetchCapabilities(ctx context.Context, svc health.CapabilityService) []health.Capability {
	if svc == nil {
		return nil
	}
	return svc.GetCapabilities(ctx)
}

// formatTimezoneOffset renders a seconds-from-UTC offset as ±HH:MM.
func formatTimezoneOffset(offsetSeconds int) string {
	sign := '+'
	secs := offsetSeconds
	if secs < 0 {
		sign = '-'
		secs = -secs
	}
	hours := secs / 3600
	minutes := (secs % 3600) / 60
	return string(sign) + formatInt(hours) + ":" + pad2(minutes)
}

// pad2 returns a zero-padded two-digit string (used for minutes).
func pad2(n int) string {
	if n < 10 {
		return "0" + formatInt(n)
	}
	return formatInt(n)
}
