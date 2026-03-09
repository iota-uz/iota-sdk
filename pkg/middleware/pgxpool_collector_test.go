package middleware

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mockPool is a minimal wrapper to verify the collector registers and describes
// the expected metrics. We can't easily unit-test Collect() without a real
// pgxpool.Pool, but we can verify Describe() emits the correct descriptors.

func TestPgxPoolCollector_DescribeEmitsAllMetrics(t *testing.T) {
	// We can't construct a real pgxpool.Pool in a unit test without a database,
	// but we can verify that a nil-pool collector at least describes all expected
	// metric descriptors. Collect() would panic on nil, but Describe() does not
	// access the pool.
	c := &pgxPoolCollector{
		pool: nil, // only Describe is called
		acquiredConns: prometheus.NewDesc(
			"pgxpool_acquired_conns", "", nil, nil,
		),
		idleConns: prometheus.NewDesc(
			"pgxpool_idle_conns", "", nil, nil,
		),
		totalConns: prometheus.NewDesc(
			"pgxpool_total_conns", "", nil, nil,
		),
		maxConns: prometheus.NewDesc(
			"pgxpool_max_conns", "", nil, nil,
		),
		constructingConns: prometheus.NewDesc(
			"pgxpool_constructing_conns", "", nil, nil,
		),
		acquireCount: prometheus.NewDesc(
			"pgxpool_acquire_count_total", "", nil, nil,
		),
		acquireDuration: prometheus.NewDesc(
			"pgxpool_acquire_duration_seconds_total", "", nil, nil,
		),
		emptyAcquireCount: prometheus.NewDesc(
			"pgxpool_empty_acquire_count_total", "", nil, nil,
		),
		canceledAcquireCount: prometheus.NewDesc(
			"pgxpool_canceled_acquire_count_total", "", nil, nil,
		),
		newConns: prometheus.NewDesc(
			"pgxpool_new_conns_total", "", nil, nil,
		),
		maxLifetimeDestroyCount: prometheus.NewDesc(
			"pgxpool_max_lifetime_destroy_count_total", "", nil, nil,
		),
		maxIdleDestroyCount: prometheus.NewDesc(
			"pgxpool_max_idle_destroy_count_total", "", nil, nil,
		),
	}

	ch := make(chan *prometheus.Desc, 20)
	c.Describe(ch)
	close(ch)

	var count int
	for range ch {
		count++
	}

	const expectedMetrics = 12
	if count != expectedMetrics {
		t.Fatalf("expected %d metric descriptors, got %d", expectedMetrics, count)
	}
}

func TestPgxPoolCollector_RegistersWithoutPanic(t *testing.T) {
	// Verify the collector can be registered (Describe doesn't panic).
	// We use testutil.ToFloat64 indirectly by just registering.
	reg := prometheus.NewPedanticRegistry()

	c := &pgxPoolCollector{
		pool: nil,
		acquiredConns: prometheus.NewDesc(
			"pgxpool_acquired_conns", "test", nil, nil,
		),
		idleConns: prometheus.NewDesc(
			"pgxpool_idle_conns", "test", nil, nil,
		),
		totalConns: prometheus.NewDesc(
			"pgxpool_total_conns", "test", nil, nil,
		),
		maxConns: prometheus.NewDesc(
			"pgxpool_max_conns", "test", nil, nil,
		),
		constructingConns: prometheus.NewDesc(
			"pgxpool_constructing_conns", "test", nil, nil,
		),
		acquireCount: prometheus.NewDesc(
			"pgxpool_acquire_count_total", "test", nil, nil,
		),
		acquireDuration: prometheus.NewDesc(
			"pgxpool_acquire_duration_seconds_total", "test", nil, nil,
		),
		emptyAcquireCount: prometheus.NewDesc(
			"pgxpool_empty_acquire_count_total", "test", nil, nil,
		),
		canceledAcquireCount: prometheus.NewDesc(
			"pgxpool_canceled_acquire_count_total", "test", nil, nil,
		),
		newConns: prometheus.NewDesc(
			"pgxpool_new_conns_total", "test", nil, nil,
		),
		maxLifetimeDestroyCount: prometheus.NewDesc(
			"pgxpool_max_lifetime_destroy_count_total", "test", nil, nil,
		),
		maxIdleDestroyCount: prometheus.NewDesc(
			"pgxpool_max_idle_destroy_count_total", "test", nil, nil,
		),
	}

	if err := reg.Register(c); err != nil {
		t.Fatalf("failed to register pgxpool collector: %v", err)
	}

	// Suppress unused import warning for testutil.
	_ = testutil.ToFloat64
}
