package spotlight

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Hot-path benchmarks for the spotlight remediation (issue #2810).
// Covers the four perf signals from the local E2E test plan:
//   1. searchCacheKey — ranks every cache lookup
//   2. buildAccessFilter + validateAccessFilter — grows with role/permission count
//   3. backoffWithJitter — runs on every retry attempt
//   4. EventDeduper.Seen — gates every inbound projector event
//
// Run with:  go test -bench=. -benchmem -benchtime=3s ./pkg/spotlight

func makeRoles(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = "role_" + strconv.Itoa(i)
	}
	return out
}

func makePerms(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = "module.entity:action_" + strconv.Itoa(i)
	}
	return out
}

func BenchmarkSearchCacheKey(b *testing.B) {
	tenant := uuid.New()
	req := SearchRequest{
		TenantID:    tenant,
		UserID:      "user-1234",
		Query:       "хочу полис каско на грузовик 2024",
		Language:    "ru",
		Intent:      "auto.search",
		Mode:        "hybrid",
		TopK:        50,
		Roles:       makeRoles(20),
		Permissions: makePerms(40),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = searchCacheKey(req)
	}
}

func BenchmarkBuildAccessFilter(b *testing.B) {
	for _, sz := range []int{1, 5, 20, 100, 500} {
		b.Run(fmt.Sprintf("roles_%d_perms_%d", sz, sz*2), func(b *testing.B) {
			req := SearchRequest{
				UserID:      "user-1234",
				Roles:       makeRoles(sz),
				Permissions: makePerms(sz * 2),
			}
			b.ReportAllocs()
			b.ResetTimer()
			var filter string
			for i := 0; i < b.N; i++ {
				filter = buildAccessFilter(req)
			}
			b.ReportMetric(float64(len(filter)), "bytes/op")
		})
	}
}

func BenchmarkValidateAccessFilter(b *testing.B) {
	req := SearchRequest{
		UserID:      "user-1234",
		Roles:       makeRoles(50),
		Permissions: makePerms(100),
	}
	filter := buildAccessFilter(req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateAccessFilter(nil, filter)
	}
}

func BenchmarkBackoffWithJitter(b *testing.B) {
	p := DefaultRetryPolicy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backoffWithJitter(p, i%5)
	}
}

func BenchmarkEventDeduperSeen(b *testing.B) {
	cfg := EventDedupConfig{Capacity: 10_000, TTL: 30 * time.Minute}
	d := NewEventDeduper(cfg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Seen("crm.contracts", "pk-"+strconv.Itoa(i%5000), "ev-"+strconv.Itoa(i))
	}
}

func BenchmarkEventDeduperSeen_Hot(b *testing.B) {
	// Same (provider,pk,event_id) → exercises the LRU hit path.
	cfg := EventDedupConfig{Capacity: 1000, TTL: 30 * time.Minute}
	d := NewEventDeduper(cfg)
	_ = d.Seen("crm.contracts", "pk-1", "ev-hot")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Seen("crm.contracts", "pk-1", "ev-hot")
	}
}

func BenchmarkEngineBreakerExecute_Closed(b *testing.B) {
	cfg := DefaultEngineBreakerConfig("bench")
	br := NewEngineBreaker(cfg)
	ctx := context.Background()
	noop := func() error { return nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = br.Execute(ctx, noop)
	}
}

func BenchmarkEngineBreakerExecute_Open(b *testing.B) {
	cfg := DefaultEngineBreakerConfig("bench-open")
	cfg.MinRequests = 2
	cfg.FailureRatio = 0.5
	cfg.Timeout = 1 * time.Hour
	br := NewEngineBreaker(cfg)
	ctx := context.Background()
	boom := errors.New("boom")
	for i := 0; i < 6; i++ {
		_ = br.Execute(ctx, func() error { return boom })
	}
	if br.State() != BreakerStateOpen {
		b.Fatalf("breaker did not open: %v", br.State())
	}
	noop := func() error { return nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = br.Execute(ctx, noop)
	}
}
