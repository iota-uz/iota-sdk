package code

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Every scenario below must end within its deadline + 50 ms with the process alive.

func TestIsolationDeadline(t *testing.T) {
	const deadline = 300 * time.Millisecond
	testEngine(t) // created outside the measured runs
	for name, code := range map[string]string{
		"busy loop":           `export default async () => { while (true) {} }`,
		"allocating loop":     `export default async () => { for (;;) new Array(1e6).fill(1) }`,
		"string repeat loop":  `export default async () => { let s = ""; for (;;) { s = "x".repeat(1 << 24); s = "" } }`,
		"redos":               `export default async () => { /^(a+)+$/.test("a".repeat(40) + "b"); return { outcome: "done" } }`,
		"await loop":          `export default async () => { for (;;) await null }`,
		"top level loop":      "for (;;) {}\nexport default async () => ({ outcome: 'done' })",
		"swallowed interrupt": `export default async () => { for (;;) { try { for (;;) {} } catch (e) {} } }`,
		"bigint":              `export default async () => { const x = 7n ** 9999999n; return { outcome: "done", output: String(x).length } }`,
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			res := runCode(t, code, RunInput{Limits: Limits{Timeout: deadline}})
			within(t, time.Since(start), deadline)
			wantReason(t, res, ReasonTimeout)
		})
	}
}

// slowServer answers after hold, ignoring the client going away.
func slowServer(t *testing.T, hold time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(hold)
		_, _ = w.Write([]byte("late"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testFetcher sends every request for a public-looking name to srv.
func testFetcher(srv *httptest.Server, opts ...FetchOption) Fetcher {
	addr := srv.Listener.Addr().String()
	base := []FetchOption{
		WithResolver(staticResolver{"example.com": {"93.184.215.14"}}),
		WithDialer(func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}),
		withTLSConfig(srv.Client().Transport.(*http.Transport).TLSClientConfig),
	}
	return NewGuardedFetcher(append(base, opts...)...)
}

func TestIsolationFetchAtDeadlineNeverHangs(t *testing.T) {
	srv := slowServer(t, 2*time.Second)
	fetcher := testFetcher(srv)
	c := mustCompile(t, LangTS, `export default async (_: any, env: any) => {
  const r = await env.fetch("https://example.com/slow");
  return { outcome: "done", output: await r.text() };
}`)
	const deadline = 100 * time.Millisecond
	runs := 100
	if testing.Short() {
		runs = 10
	}
	var wg sync.WaitGroup
	var late atomic.Int32
	sem := make(chan struct{}, 10)
	for i := 0; i < runs; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			start := time.Now()
			res, err := testEngine(t).Run(context.Background(), c, RunInput{
				Outcomes: []string{"done"}, Fetcher: fetcher, Limits: Limits{Timeout: deadline},
			})
			if time.Since(start) > deadline+50*time.Millisecond {
				late.Add(1)
			}
			if err != nil || res.Err == nil || res.Err.Reason != ReasonTimeout {
				t.Errorf("run: %v %+v", err, res)
			}
		}()
	}
	wg.Wait()
	if n := late.Load(); n > 0 {
		t.Fatalf("%d of %d runs overshot the deadline", n, runs)
	}
}

func TestIsolationOutOfMemory(t *testing.T) {
	baseline := rssBytes()
	var peak atomic.Int64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if r := rssBytes(); r > peak.Load() {
				peak.Store(r)
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	res := runCode(t, `export default async () => {
  const keep: string[] = [];
  for (let i = 0; ; i++) keep.push("x".repeat(1 << 20) + i);
}`, RunInput{Limits: Limits{Timeout: 20 * time.Second}})
	close(stop)
	<-done
	wantReason(t, res, ReasonOutOfMemory)
	if baseline > 0 {
		const capBytes = DefaultMemoryPages * wasmPageBytes
		if grew := peak.Load() - baseline; grew > 4*capBytes {
			t.Fatalf("RSS grew by %d MiB, more than 4× the %d MiB cap", grew>>20, capBytes>>20)
		}
	}
	// The engine is still usable.
	res = runCode(t, `export default async () => ({ outcome: "done" })`, RunInput{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
}

// rssBytes is the resident set of the process on Linux, 0 elsewhere.
func rssBytes() int64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if rest, ok := strings.CutPrefix(s.Text(), "VmRSS:"); ok {
			kb, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB")), 10, 64)
			return kb << 10
		}
	}
	return 0
}

func TestIsolationStackOverflow(t *testing.T) {
	for name, code := range map[string]string{
		"recursion":    `const r = (n: number): number => r(n + 1) + 1; export default async () => ({ outcome: "done", output: r(0) })`,
		"top level":    "function f(): number { return f() + 1 }\nf();\nexport default async () => ({ outcome: 'done' })",
		"json nesting": `export default async () => ({ outcome: "done", output: JSON.parse("[".repeat(1e6)) })`,
	} {
		t.Run(name, func(t *testing.T) {
			res := runCode(t, code, RunInput{})
			wantReason(t, res, ReasonStackOverflow)
		})
	}
}

func TestIsolationImports(t *testing.T) {
	for _, code := range []string{
		`import "node:fs"; export default async () => ({ outcome: "done" })`,
		`import { readFileSync } from "fs"; export default async () => ({ outcome: "done", output: readFileSync("/etc/passwd") })`,
		`const fs = require("fs"); export default async () => ({ outcome: "done" })`,
		`export default async () => { await import("std"); return { outcome: "done" } }`,
		`export * from "./x.js"`,
	} {
		_, err := NewCompiler().Compile(context.Background(), Source{Code: code, Lang: LangJS})
		var ce *CompileError
		if !errors.As(err, &ce) {
			t.Errorf("%s: want CompileError, got %v", code, err)
			continue
		}
		if ce.Line != 1 || ce.Column < 1 {
			t.Errorf("%s: no position: %+v", code, ce)
		}
	}
	for _, spec := range []string{`["qjs","std"].join(":")`, `String.fromCharCode(111, 115)`, `["", "etc", "hosts"].join("/")`, `["./x", "js"].join(".")`, `["data:text/javascript", "export default 1"].join(",")`} {
		res := runCode(t, `export default async () => { await import(`+spec+`); return { outcome: "done" } }`, RunInput{})
		wantReason(t, res, ReasonException)
	}
}

func TestIsolationNoAmbientAuthority(t *testing.T) {
	res := runCode(t, `declare const globalThis: any;
export default async () => ({ outcome: "done", output: ["std", "os", "print", "console", "require", "process", "fetch", "WebAssembly", "Deno", "Bun", "setTimeout"]
  .filter((k) => typeof globalThis[k] !== "undefined") })`, RunInput{})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if string(res.Output) != `[]` {
		t.Fatalf("globals reachable: %s", res.Output)
	}
}

func TestIsolationFreshInstancePerRun(t *testing.T) {
	code := `declare const globalThis: any;
export default async () => { const seen = globalThis.__leak === 1; globalThis.__leak = 1; Object.prototype.polluted = 1; return { outcome: "done", output: seen } }`
	for i := 0; i < 3; i++ {
		res := runCode(t, code, RunInput{})
		if res.Err != nil || string(res.Output) != "false" {
			t.Fatalf("run %d saw state of an earlier run: %s %v", i, res.Output, res.Err)
		}
	}
}

func TestIsolationClockAndRandom(t *testing.T) {
	code := `export default async () => ({ outcome: "done", output: { now: Date.now(), r: Math.random() } })`
	var rs []float64
	for i := 0; i < 2; i++ {
		res := runCode(t, code, RunInput{})
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		var out struct {
			Now int64   `json:"now"`
			R   float64 `json:"r"`
		}
		if err := json.Unmarshal(res.Output, &out); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(time.UnixMilli(out.Now)); d < -time.Second || d > time.Second {
			t.Fatalf("Date.now() is %s off the host clock", d)
		}
		rs = append(rs, out.R)
	}
	if rs[0] == rs[1] {
		t.Fatalf("Math.random() repeats across instances: %v", rs)
	}
}

func TestFreshInstanceLatency(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("latency is measured without -short and -race")
	}
	eng := testEngine(t)
	c := mustCompile(t, LangJS, `export default async () => ({ outcome: "done" })`)
	in := RunInput{Outcomes: []string{"done"}}
	for i := 0; i < 50; i++ { // warm up
		if _, err := eng.Run(context.Background(), c, in); err != nil {
			t.Fatal(err)
		}
	}
	const n = 1000
	ds := make([]time.Duration, n)
	for i := range ds {
		res, err := eng.Run(context.Background(), c, in)
		if err != nil || res.Err != nil {
			t.Fatal(err, res.Err)
		}
		ds[i] = res.Duration
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	p50 := ds[n/2]
	t.Logf("fresh instance per run: p50 %s, p90 %s, p99 %s", p50, ds[n*9/10], ds[n*99/100])
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" && p50 > time.Millisecond {
		t.Fatalf("p50 %s exceeds 1 ms", p50)
	}
}
