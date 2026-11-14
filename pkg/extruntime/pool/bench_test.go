package pool_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
)

const benchArtifact = `
export const echo = async (ctx, input) => input;
export const hostcalls = async (ctx, input) => {
  for (let i = 0; i < input.n; i++) await ctx.query.get("e", { key: "k" });
  return input.n;
};`

func benchPool(tb testing.TB, size int) *pool.Pool {
	tb.Helper()
	p, err := pool.New(pool.Config{
		Size:         size,
		WorkerScript: pooltest.BuildWorker(tb),
		Logger:       slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

func median(d []time.Duration) time.Duration {
	slices.Sort(d)
	return d[len(d)/2]
}

// TestReportIPC logs cold start and warm round-trip numbers (go test -v -run TestReportIPC).
func TestReportIPC(t *testing.T) {
	script := pooltest.BuildWorker(t)
	cfg := pool.Config{Size: 1, WorkerScript: script, Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}

	var cold []time.Duration
	for i := 0; i < 7; i++ {
		start := time.Now()
		p, err := pool.New(cfg)
		require.NoError(t, err)
		cold = append(cold, time.Since(start))
		require.NoError(t, p.Close(context.Background()))
	}

	p := benchPool(t, 1)
	art := pooltest.Artifact(t, benchArtifact)
	first := time.Now()
	_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "echo", `{"a":1}`, nil))
	require.NoError(t, err)
	firstImport := time.Since(first)

	const n = 2000
	warm := make([]time.Duration, n)
	for i := range warm {
		s := time.Now()
		_, err := p.Invoke(context.Background(), invoke(art, pool.KindCommand, "echo", `{"a":1}`, nil))
		require.NoError(t, err)
		warm[i] = time.Since(s)
	}

	calls := pool.HostCallsFunc(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`null`), nil
	})
	const hc = 1000
	s := time.Now()
	_, err = p.Invoke(context.Background(), invoke(art, pool.KindCommand, "hostcalls", `{"n":1000}`, calls))
	require.NoError(t, err)
	perCall := time.Since(s) / hc

	slices.Sort(warm)
	t.Logf("cold start (spawn node -> ready), median of 7: %v (min %v, max %v)", median(cold), cold[0], cold[len(cold)-1])
	t.Logf("first invocation incl. artifact import: %v", firstImport)
	t.Logf("warm invoke round trip (echo), %d runs: p50=%v p95=%v p99=%v", n, warm[n/2], warm[n*95/100], warm[n*99/100])
	t.Logf("host call round trip inside one invocation (Node->Go->Node), %d calls: %v/call", hc, perCall)
}

func BenchmarkWarmInvoke(b *testing.B) {
	p := benchPool(b, 1)
	art := pooltest.Artifact(b, benchArtifact)
	in := invoke(art, pool.KindCommand, "echo", `{"a":1}`, nil)
	_, err := p.Invoke(context.Background(), in)
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Invoke(context.Background(), in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHostCall(b *testing.B) {
	p := benchPool(b, 1)
	art := pooltest.Artifact(b, benchArtifact)
	calls := pool.HostCallsFunc(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`null`), nil
	})
	in := invoke(art, pool.KindCommand, "hostcalls", `{"n":`+itoa(b.N)+`}`, calls)
	b.ResetTimer()
	if _, err := p.Invoke(context.Background(), in); err != nil {
		b.Fatal(err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
