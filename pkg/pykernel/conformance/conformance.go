//go:build unix

// Package conformance is the pykernel sandbox conformance suite. A consumer
// runs it against the Manager it uses in production — in CI and on a copy of
// the target environment — before it enables untrusted execution. Every probe
// runs as model-authored code inside a real kernel and tries to break one
// boundary: read /proc/1/environ, $HOME or the application binary, write
// outside the workdir, open TCP/UDP to the internet, to the database host and
// to a host listener, reach abstract and pathname unix sockets, ptrace or
// signal the host, use namespaces/mount/bpf/keyrings/perf/io_uring, fork-bomb,
// exhaust memory, overrun the workdir quota. Host-side listeners confirm that
// nothing got through. The suite also checks the run-time contract: exec
// cancellation and timeouts reach capability calls and free the slot, output
// truncation and a stalled consumer are reported in EventDone.
//
// Run returns a Report; Report.Err is nil only when every probe passed.
package conformance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
)

// Options configures Run.
type Options struct {
	// Manager is the kernel manager under test, configured exactly as the
	// consumer runs it in production. Required.
	Manager pykernel.Manager
	// WorkRoot holds the per-probe workdirs (default: a temporary directory,
	// removed afterwards). It must be writable by the kernel uid.
	WorkRoot string
	// DatabaseAddrs are host:port addresses of the databases the application
	// uses; the kernel must not reach any of them. DB_HOST/DB_PORT from the
	// environment and 127.0.0.1:5432 are always tried.
	DatabaseAddrs []string
	// ExternalAddr is an internet TCP address to try (default 1.1.1.1:443);
	// ExternalUDPAddr likewise for UDP (default 8.8.8.8:53).
	ExternalAddr    string
	ExternalUDPAddr string
	// ProbeTimeout bounds one probe exec (default 60s).
	ProbeTimeout time.Duration
	// QuotaProbeMaxBytes bounds how much the quota probe writes before it
	// declares the quota unenforced (default 4 GiB).
	QuotaProbeMaxBytes int64
	// Skip names probes not to run (see Probes).
	Skip []string
}

// Check is one assertion inside a probe.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Result is the outcome of one probe.
type Result struct {
	Probe  string  `json:"probe"`
	Passed bool    `json:"passed"`
	Checks []Check `json:"checks"`
	Error  string  `json:"error,omitempty"`
}

// Report is the outcome of a conformance run.
type Report struct {
	Results []Result `json:"results"`
}

// Failed lists the probes that did not pass.
func (r Report) Failed() []Result {
	var out []Result
	for _, res := range r.Results {
		if !res.Passed {
			out = append(out, res)
		}
	}
	return out
}

// ErrNonConformant matches the error Report.Err returns.
var ErrNonConformant = errors.New("pykernel/conformance: sandbox is not conformant")

// Err is nil when every probe passed.
func (r Report) Err() error {
	failed := r.Failed()
	if len(failed) == 0 {
		return nil
	}
	parts := make([]string, 0, len(failed))
	for _, f := range failed {
		var bad []string
		for _, c := range f.Checks {
			if !c.OK {
				bad = append(bad, fmt.Sprintf("%s (%s)", c.Name, c.Detail))
			}
		}
		if f.Error != "" {
			bad = append(bad, f.Error)
		}
		parts = append(parts, f.Probe+": "+strings.Join(bad, ", "))
	}
	return fmt.Errorf("%w: %s", ErrNonConformant, strings.Join(parts, "; "))
}

// Probes lists every probe name in run order.
func Probes() []string {
	out := make([]string, 0, len(probes))
	for _, p := range probes {
		out = append(out, p.name)
	}
	return out
}

type probe struct {
	name string
	run  func(ctx context.Context, r *runner) Result
}

// Run executes the suite against opts.Manager.
func Run(ctx context.Context, opts Options) Report {
	if opts.Manager == nil {
		return Report{Results: []Result{{Probe: "setup", Error: "Options.Manager is required"}}}
	}
	r, err := newRunner(opts)
	if err != nil {
		return Report{Results: []Result{{Probe: "setup", Error: err.Error()}}}
	}
	defer r.close()
	skip := make(map[string]bool, len(opts.Skip))
	for _, s := range opts.Skip {
		skip[s] = true
	}
	var rep Report
	for _, p := range probes {
		if skip[p.name] {
			continue
		}
		if err := ctx.Err(); err != nil {
			rep.Results = append(rep.Results, Result{Probe: p.name, Error: err.Error()})
			continue
		}
		res := p.run(ctx, r)
		res.Probe = p.name
		res.Passed = res.Error == "" && len(res.Checks) > 0
		for _, c := range res.Checks {
			res.Passed = res.Passed && c.OK
		}
		rep.Results = append(rep.Results, res)
	}
	return rep
}

// --- runner ---------------------------------------------------------------

type runner struct {
	opts     Options
	root     string
	ownRoot  bool
	canary   string
	tcp      *tcpTrap
	udp      *udpTrap
	abstract *tcpTrap
	pathUnix *tcpTrap
	sockPath string
	restore  func()
}

const canaryEnv = "PYKERNEL_CONFORMANCE_CANARY"

func newRunner(opts Options) (*runner, error) {
	if opts.ExternalAddr == "" {
		opts.ExternalAddr = "1.1.1.1:443"
	}
	if opts.ExternalUDPAddr == "" {
		opts.ExternalUDPAddr = "8.8.8.8:53"
	}
	if opts.ProbeTimeout == 0 {
		opts.ProbeTimeout = 60 * time.Second
	}
	if opts.QuotaProbeMaxBytes == 0 {
		opts.QuotaProbeMaxBytes = 4 << 30
	}
	r := &runner{opts: opts, root: opts.WorkRoot}
	if r.root == "" {
		dir, err := os.MkdirTemp("", "pykernel-conformance-")
		if err != nil {
			return nil, err
		}
		r.root, r.ownRoot = dir, true
	}
	suffix := randHex()
	r.canary = "secret-" + suffix
	prev, had := os.LookupEnv(canaryEnv)
	_ = os.Setenv(canaryEnv, r.canary)
	r.restore = func() {
		if had {
			_ = os.Setenv(canaryEnv, prev)
		} else {
			_ = os.Unsetenv(canaryEnv)
		}
	}
	var err error
	if r.tcp, err = listenTrap("tcp", "127.0.0.1:0"); err != nil {
		r.close()
		return nil, err
	}
	if r.udp, err = listenUDPTrap(); err != nil {
		r.close()
		return nil, err
	}
	// Abstract unix sockets are Linux-only; elsewhere the probe is skipped.
	r.abstract, _ = listenTrap("unix", "@pykernel-conformance-"+suffix)
	r.sockPath = filepath.Join(os.TempDir(), "pykernel-conformance-"+suffix+".sock")
	r.pathUnix, _ = listenTrap("unix", r.sockPath)
	return r, nil
}

func (r *runner) close() {
	for _, t := range []*tcpTrap{r.tcp, r.abstract, r.pathUnix} {
		if t != nil {
			t.close()
		}
	}
	if r.udp != nil {
		r.udp.close()
	}
	_ = os.Remove(r.sockPath)
	if r.restore != nil {
		r.restore()
	}
	if r.ownRoot {
		_ = os.RemoveAll(r.root)
	}
}

type session struct {
	key     string
	tenant  uuid.UUID
	caps    pykernel.CapabilitySet
	workdir string
}

func (s *session) Key() string                          { return s.key }
func (s *session) TenantID() uuid.UUID                  { return s.tenant }
func (s *session) Capabilities() pykernel.CapabilitySet { return s.caps }
func (s *session) Mode() pykernel.Mode                  { return pykernel.ModePlan }
func (s *session) Workdir() string                      { return s.workdir }

func (r *runner) session(name string, caps ...pykernel.Capability) (*session, error) {
	set, err := pykernel.NewCapabilitySet(caps...)
	if err != nil {
		return nil, err
	}
	wd := filepath.Join(r.root, name+"-"+randHex())
	if err := os.MkdirAll(wd, 0o700); err != nil {
		return nil, err
	}
	return &session{key: "pykernel-conformance/" + name + "/" + randHex(), tenant: uuid.New(), caps: set, workdir: wd}, nil
}

// outcome is what one exec produced.
type outcome struct {
	checks []Check
	stdout string
	err    *pykernel.ExecError
	done   *pykernel.DonePayload
}

func collect(ch <-chan pykernel.ExecEvent) outcome {
	var o outcome
	var sb strings.Builder
	for ev := range ch {
		switch ev.Kind {
		case pykernel.EventStdout:
			sb.Write(ev.Stdout)
		case pykernel.EventError:
			o.err = ev.Err
		case pykernel.EventDone:
			o.done = ev.Done
		case pykernel.EventResult, pykernel.EventMetric, pykernel.EventLog, pykernel.EventTruncated:
		}
	}
	o.stdout = sb.String()
	for _, line := range strings.Split(o.stdout, "\n") {
		raw, ok := strings.CutPrefix(line, "PYKERNEL_PROBE ")
		if !ok {
			continue
		}
		var c Check
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			o.checks = append(o.checks, c)
		}
	}
	return o
}

// withKernel acquires a kernel for a fresh session, runs fn and disposes it.
func (r *runner) withKernel(ctx context.Context, name string, caps []pykernel.Capability, fn func(k pykernel.Kernel, s *session) Result) Result {
	s, err := r.session(name, caps...)
	if err != nil {
		return Result{Error: err.Error()}
	}
	k, err := r.opts.Manager.Acquire(ctx, s)
	if err != nil {
		return Result{Error: "acquire: " + err.Error()}
	}
	defer func() {
		_ = r.opts.Manager.Release(k)
		_ = r.opts.Manager.Evict(s.key)
	}()
	return fn(k, s)
}

// pyProbe runs code (prefixed with the helper prelude) and turns its
// PYKERNEL_PROBE lines into checks.
func (r *runner) pyProbe(ctx context.Context, name string, cfg map[string]any, code string) Result {
	return r.withKernel(ctx, name, nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, cfg, code, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		if o.err != nil {
			res.Error = fmt.Sprintf("probe raised %s", o.err.Error())
		}
		return res
	})
}

func (r *runner) exec(ctx context.Context, k pykernel.Kernel, s *session, cfg map[string]any, code string, timeout time.Duration) (outcome, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["host_pid"] = os.Getpid()
	cfg["workdir"] = s.workdir
	raw, err := json.Marshal(cfg)
	if err != nil {
		return outcome{}, err
	}
	src := fmt.Sprintf("CFG = __import__('json').loads(%q)\n%s\n%s", string(raw), prelude, code)
	ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: src, Timeout: timeout, OutputCap: 1 << 20, SeparateStderr: true})
	if err != nil {
		return outcome{}, fmt.Errorf("exec: %w", err)
	}
	return collect(ch), nil
}

// --- host traps -------------------------------------------------------------

// tcpTrap is a stream listener that counts every connection it accepts.
type tcpTrap struct {
	l    net.Listener
	hits atomic.Int64
	wg   sync.WaitGroup
}

func listenTrap(network, addr string) (*tcpTrap, error) {
	l, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	t := &tcpTrap{l: l}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.hits.Add(1)
			_ = c.Close()
		}
	}()
	return t, nil
}

func (t *tcpTrap) addr() string { return t.l.Addr().String() }

func (t *tcpTrap) close() {
	_ = t.l.Close()
	t.wg.Wait()
}

// udpTrap counts datagrams.
type udpTrap struct {
	c    net.PacketConn
	hits atomic.Int64
	wg   sync.WaitGroup
}

func listenUDPTrap() (*udpTrap, error) {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	t := &udpTrap{c: c}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		buf := make([]byte, 64)
		for {
			if _, _, err := c.ReadFrom(buf); err != nil {
				return
			}
			t.hits.Add(1)
		}
	}()
	return t, nil
}

func (t *udpTrap) close() {
	_ = t.c.Close()
	t.wg.Wait()
}

func randHex() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// groupGone reports whether no process is left in process group pgid.
func groupGone(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return errors.Is(err, syscall.ESRCH)
}
