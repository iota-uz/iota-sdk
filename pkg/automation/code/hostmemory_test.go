package code

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/metrics"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// heapPeak samples the live Go heap (as of each collection) while fn runs and returns its peak growth.
func heapPeak(fn func()) uint64 {
	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	live := func() uint64 {
		metrics.Read(sample)
		return sample[0].Value.Uint64()
	}
	runtime.GC()
	base := live()
	var peak atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if v := live(); v > base && v-base > peak.Load() {
				peak.Store(v - base)
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	fn()
	runtime.GC() // what is still referenced at the end counts too
	if v := live(); v > base && v-base > peak.Load() {
		peak.Store(v - base)
	}
	close(stop)
	<-done
	return peak.Load()
}

// Code cannot make the host hold unbounded memory: replies with big bodies wait in Go only up to the
// held-body budget, whatever the code does with its microtasks.
func TestHostMemoryFetchReplies(t *testing.T) {
	if testing.Short() {
		t.Skip("moves several hundred MiB")
	}
	body := []byte(strings.Repeat("<", 9<<20))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	var res *RunResult
	grew := heapPeak(func() {
		res = runCode(t, `export default async (_: any, env: any) => {
  const all = Array.from({ length: 30 }, () => env.fetch("https://example.com/"));
  all[0].then(() => { const end = Date.now() + 2000; while (Date.now() < end) {} });
  const settled = await Promise.allSettled(all);
  return { outcome: "done", output: settled.filter((s) => s.status === "rejected").length };
}`, RunInput{Fetcher: testFetcher(srv), Limits: Limits{Timeout: 20 * time.Second}})
	})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if string(res.Output) == "0" {
		t.Fatal("30 unread 9 MiB bodies were all held")
	}
	t.Logf("live Go heap peak growth %d MiB, %s fetches refused", grew>>20, res.Output)
	if grew > 4*DefaultMemoryPages*wasmPageBytes {
		t.Fatalf("Go heap grew by %d MiB", grew>>20)
	}
}

func TestHostMemoryCallFloods(t *testing.T) {
	srv := slowServer(t, 50*time.Millisecond)
	for name, code := range map[string]string{
		"fetch flood": `export default async (_: any, env: any) => {
  const s = "x".repeat(1 << 20);
  for (;;) env.fetch("https://example.com/", { method: "POST", body: s }).catch(() => {});
}`,
		"log flood":    `export default async (_: any, env: any) => { const s = "x".repeat(30e6); for (;;) env.log(s) }`,
		"digest flood": `export default async (_: any, env: any) => { const s = "x".repeat(1 << 20); for (;;) env.crypto.digest("SHA-256", s) }`,
	} {
		t.Run(name, func(t *testing.T) {
			goroutines := runtime.NumGoroutine()
			var maxG atomic.Int64
			grew := heapPeak(func() {
				stop := make(chan struct{})
				go func() {
					for {
						if n := int64(runtime.NumGoroutine()); n > maxG.Load() {
							maxG.Store(n)
						}
						select {
						case <-stop:
							return
						case <-time.After(5 * time.Millisecond):
						}
					}
				}()
				res := runCode(t, code, RunInput{Fetcher: testFetcher(srv), Limits: Limits{Timeout: 2 * time.Second}})
				close(stop)
				wantReason(t, res, ReasonTimeout)
			})
			t.Logf("live Go heap peak growth %d MiB, %d extra goroutines at most", grew>>20, maxG.Load()-int64(goroutines))
			if grew > 256<<20 {
				t.Fatalf("Go heap grew by %d MiB", grew>>20)
			}
			if extra := maxG.Load() - int64(goroutines); extra > 200 {
				t.Fatalf("%d extra goroutines", extra)
			}
		})
	}
}

func TestCompileWaitKeepsEachDeadline(t *testing.T) {
	eng := testEngine(t)
	// A unique program, large enough to take a while to compile.
	var b strings.Builder
	fmt.Fprintf(&b, "// %d\n", time.Now().UnixNano())
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&b, "globalThis.v%d = [%d, { k: %d > 1 }];\n", i, i, i)
	}
	b.WriteString(`export default async () => ({ outcome: "done" })`)
	c := mustCompile(t, LangTS, b.String())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = eng.Run(context.Background(), c, RunInput{Outcomes: []string{"done"}, Limits: Limits{Timeout: 5 * time.Second}})
	}()
	time.Sleep(2 * time.Millisecond)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			const deadline = 10 * time.Millisecond
			start := time.Now()
			res, err := eng.Run(context.Background(), c, RunInput{Outcomes: []string{"done"}, Limits: Limits{Timeout: deadline}})
			if err != nil {
				t.Error(err)
				return
			}
			if took := time.Since(start); took > deadline+50*time.Millisecond {
				t.Errorf("took %s with a %s deadline (%v)", took, deadline, res.Err)
			}
		}()
	}
	wg.Wait()
}

func TestQueueTimeoutIsErrBusy(t *testing.T) {
	eng, err := NewEngine(context.Background(), WithMaxConcurrent(1), WithQueueTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eng.Close(context.Background()) }()
	slow := mustCompile(t, LangJS, `export default async () => { const end = Date.now() + 400; while (Date.now() < end) {} return { outcome: "done" } }`)
	go func() { _, _ = eng.Run(context.Background(), slow, RunInput{Outcomes: []string{"done"}}) }()
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	_, err = eng.Run(context.Background(), slow, RunInput{Outcomes: []string{"done"}})
	if !errors.Is(err, ErrBusy) || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("got %v after %s", err, time.Since(start))
	}
}

func TestCloseDuringARunIsAHostError(t *testing.T) {
	eng, err := NewEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c := mustCompile(t, LangJS, `export default async () => { for (;;) {} }`)
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = eng.Close(context.Background())
	}()
	res, err := eng.Run(context.Background(), c, RunInput{Outcomes: []string{"done"}, Limits: Limits{Timeout: 5 * time.Second}})
	if err == nil {
		t.Fatalf("no host error, result %+v", res.Err)
	}
}

// Header-heavy requests are bounded by their serialized size before they cross into Go.
func TestHostMemoryHeaderHeavyFetches(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("serializes ~50 MiB of headers in the sandbox; measured without -short and -race")
	}
	srv := slowServer(t, time.Second)
	var res *RunResult
	grew := heapPeak(func() {
		res = runCode(t, `export default async (_: any, env: any) => {
  const headers: Record<string, string> = {};
  for (let i = 0; i < 100; i++) headers["X-H" + i] = "é".repeat(200); // 400 UTF-8 bytes, 200 UTF-16 units
  let refused = 0;
  for (let i = 0; i < 800; i++) env.fetch("https://example.com/", { headers }).catch(() => { refused++ });
  await null;
  return { outcome: "done", output: refused };
}`, RunInput{Fetcher: testFetcher(srv), Limits: Limits{MaxFetches: 5000, Timeout: 10 * time.Second}})
	})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	t.Logf("live Go heap peak growth %d MiB, %s fetches refused before the host", grew>>20, res.Output)
	if string(res.Output) == "0" {
		t.Fatal("no request was refused by the queue budget")
	}
	if grew > 64<<20 {
		t.Fatalf("Go heap grew by %d MiB", grew>>20)
	}

	// One request whose headers are over the limit never reaches the host.
	res = runCode(t, `export default async (_: any, env: any) => {
  await env.fetch("https://example.com/", { headers: { "X-Big": "x".repeat(100000) } });
  return { outcome: "done" };
}`, RunInput{Fetcher: testFetcher(srv)})
	wantReason(t, res, ReasonException)
	if !strings.Contains(res.Err.Message, "too long") {
		t.Fatalf("message %q", res.Err.Message)
	}
}

// A compile left unfinished by the run that started it keeps that run's slot until it ends.
func TestUnfinishedCompileKeepsItsSlot(t *testing.T) {
	eng, err := NewEngine(context.Background(), WithMaxConcurrent(1), WithQueueTimeout(5*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eng.Close(context.Background()) }()
	trivial := mustCompile(t, LangJS, `export default async () => ({ outcome: "done" })`)
	in := RunInput{Outcomes: []string{"done"}}
	if _, err := eng.Run(context.Background(), trivial, in); err != nil { // its bytecode is cached now
		t.Fatal(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// %d\n", time.Now().UnixNano())
	for i := 0; b.Len() < MaxSourceChars-200; i++ {
		fmt.Fprintf(&b, "globalThis.v%d = [%d, { k: %d > 1 }];\n", i, i, i)
	}
	b.WriteString(`export default async () => ({ outcome: "done" })`)
	big := mustCompile(t, LangJS, b.String())
	res, err := eng.Run(context.Background(), big, RunInput{Outcomes: []string{"done"}, Limits: Limits{Timeout: time.Millisecond}})
	if err != nil || res.Err == nil {
		t.Fatalf("big run: %v %+v", err, res)
	}
	if _, err := eng.Run(context.Background(), trivial, in); !errors.Is(err, ErrBusy) {
		t.Fatalf("a run took the slot of an unfinished compile: %v", err)
	}
	deadline := time.Now().Add(compileTimeout)
	for {
		if _, err := eng.Run(context.Background(), trivial, in); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the slot never came back")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
