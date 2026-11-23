//go:build unix

// Command pykernel-conformance runs the pykernel sandbox conformance suite on
// the current host, with the same confinement a consumer's Manager uses. Build
// it into the application image (or run it with `go run`) on a copy of the
// target environment before enabling untrusted Python execution:
//
//	pykernel-conformance -python python3 -db "$DB_HOST:$DB_PORT"
//
// It prints the isolation report and every probe as JSON and exits non-zero
// when the sandbox is unavailable or any probe fails.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel/conformance"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/lifecycle"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/manager"
)

type output struct {
	Isolation   isolation.Report    `json:"isolation"`
	Conformance *conformance.Report `json:"conformance,omitempty"`
	Error       string              `json:"error,omitempty"`
}

func main() {
	python := flag.String("python", "python3", "Python interpreter the kernels run")
	workRoot := flag.String("workdir", "", "directory for probe workdirs (default: a temporary directory)")
	dbs := flag.String("db", "", "comma-separated host:port database addresses the kernel must not reach")
	external := flag.String("external", "1.1.1.1:443", "internet TCP address to try")
	runAs := flag.Int("run-as-uid", -1, "uid the kernels run as (required when running as root)")
	runAsGID := flag.Int("run-as-gid", -1, "gid the kernels run as (default: the uid)")
	readPaths := flag.String("read-paths", "", "comma-separated extra read-only paths, as configured in production")
	memory := flag.Uint64("memory-bytes", 0, "RLIMIT_AS (default isolation.DefaultLimits)")
	quota := flag.Int64("workdir-bytes", 256<<20, "workdir quota the quota probe overruns")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall timeout")
	flag.Parse()

	profile := isolation.Profile{}
	if *readPaths != "" {
		profile.ReadPaths = strings.Split(*readPaths, ",")
	}
	if *runAs >= 0 {
		gid := *runAsGID
		if gid < 0 {
			gid = *runAs
		}
		profile.RunAs = &isolation.Credential{UID: uint32(*runAs), GID: uint32(gid)}
	}
	var out output
	m, err := manager.New(manager.Config{
		PythonPath:   *python,
		Policy:       lifecycle.WarmPool(time.Minute, 64),
		Sandbox:      profile,
		EnvAllowlist: []string{"PATH", "LANG"},
		Limits:       isolation.ResourceLimits{AddressSpaceBytes: *memory, WorkdirBytes: *quota},
	})
	if err != nil {
		out.Isolation = isolation.Check(context.Background(), isolation.CheckOptions{PythonPath: *python, Profile: profile})
		out.Error = err.Error()
		emit(out)
		os.Exit(2)
	}
	defer func() { _ = m.Shutdown(context.Background()) }()
	out.Isolation = m.Isolation()

	var addrs []string
	if *dbs != "" {
		addrs = strings.Split(*dbs, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	rep := conformance.Run(ctx, conformance.Options{
		Manager: m, WorkRoot: *workRoot, DatabaseAddrs: addrs, ExternalAddr: *external,
	})
	out.Conformance = &rep
	if err := rep.Err(); err != nil {
		out.Error = err.Error()
	}
	emit(out)
	if out.Error != "" {
		os.Exit(1)
	}
}

func emit(out output) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
