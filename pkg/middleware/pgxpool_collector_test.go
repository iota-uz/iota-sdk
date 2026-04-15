package middleware

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgxPoolCollector_DescribeEmitsAllMetrics(t *testing.T) {
	labels := prometheus.Labels{"service": "test-service"}

	c := &pgxPoolCollector{
		acquiredConns: prometheus.NewDesc("pgxpool_acquired_conns", "", nil, labels),
		idleConns:     prometheus.NewDesc("pgxpool_idle_conns", "", nil, labels),
		totalConns:    prometheus.NewDesc("pgxpool_total_conns", "", nil, labels),
		maxConns:      prometheus.NewDesc("pgxpool_max_conns", "", nil, labels),
		constructingConns: prometheus.NewDesc(
			"pgxpool_constructing_conns", "", nil, labels,
		),
		acquireCount:            prometheus.NewDesc("pgxpool_acquire_count_total", "", nil, labels),
		acquireDuration:         prometheus.NewDesc("pgxpool_acquire_duration_seconds_total", "", nil, labels),
		emptyAcquireCount:       prometheus.NewDesc("pgxpool_empty_acquire_count_total", "", nil, labels),
		canceledAcquireCount:    prometheus.NewDesc("pgxpool_canceled_acquire_count_total", "", nil, labels),
		newConns:                prometheus.NewDesc("pgxpool_new_conns_total", "", nil, labels),
		maxLifetimeDestroyCount: prometheus.NewDesc("pgxpool_max_lifetime_destroy_count_total", "", nil, labels),
		maxIdleDestroyCount:     prometheus.NewDesc("pgxpool_max_idle_destroy_count_total", "", nil, labels),
	}

	ch := make(chan *prometheus.Desc, 20)
	c.Describe(ch)
	close(ch)

	var count int
	for range ch {
		count++
	}

	const expectedMetrics = 12
	require.Equal(t, expectedMetrics, count)
}

func TestNewPgxPoolCollector_PanicsOnNilPool(t *testing.T) {
	require.PanicsWithValue(t, "NewPgxPoolCollector: pool must not be nil", func() {
		NewPgxPoolCollector(nil, nil)
	})
}

func TestPgxPoolCollector_RegistersWithoutPanic(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	c := &pgxPoolCollector{
		acquiredConns:           prometheus.NewDesc("pgxpool_acquired_conns", "test", nil, prometheus.Labels{"service": "test-service"}),
		idleConns:               prometheus.NewDesc("pgxpool_idle_conns", "test", nil, prometheus.Labels{"service": "test-service"}),
		totalConns:              prometheus.NewDesc("pgxpool_total_conns", "test", nil, prometheus.Labels{"service": "test-service"}),
		maxConns:                prometheus.NewDesc("pgxpool_max_conns", "test", nil, prometheus.Labels{"service": "test-service"}),
		constructingConns:       prometheus.NewDesc("pgxpool_constructing_conns", "test", nil, prometheus.Labels{"service": "test-service"}),
		acquireCount:            prometheus.NewDesc("pgxpool_acquire_count_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		acquireDuration:         prometheus.NewDesc("pgxpool_acquire_duration_seconds_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		emptyAcquireCount:       prometheus.NewDesc("pgxpool_empty_acquire_count_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		canceledAcquireCount:    prometheus.NewDesc("pgxpool_canceled_acquire_count_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		newConns:                prometheus.NewDesc("pgxpool_new_conns_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		maxLifetimeDestroyCount: prometheus.NewDesc("pgxpool_max_lifetime_destroy_count_total", "test", nil, prometheus.Labels{"service": "test-service"}),
		maxIdleDestroyCount:     prometheus.NewDesc("pgxpool_max_idle_destroy_count_total", "test", nil, prometheus.Labels{"service": "test-service"}),
	}

	require.NoError(t, reg.Register(c))
}

func TestPgxPoolCollector_CollectsFromRealPool(t *testing.T) {
	pool := requireTestPool(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	conn.Release()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(NewPgxPoolCollector(pool, prometheus.Labels{"service": "test-service"}))

	families, err := reg.Gather()
	require.NoError(t, err)

	familiesByName := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		familiesByName[family.GetName()] = family
	}

	required := []string{
		"pgxpool_total_conns",
		"pgxpool_acquired_conns",
		"pgxpool_max_conns",
		"pgxpool_acquire_count_total",
	}
	for _, name := range required {
		family, ok := familiesByName[name]
		require.Truef(t, ok, "expected metric family %s to be present", name)
		require.NotEmptyf(t, family.GetMetric(), "expected metric family %s to have samples", name)
		assertMetricHasLabel(t, family, "service", "test-service")
	}
}

func requireTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Hardcoded defaults that match the local dev compose.
	// Full env-driven config (W4 scope) will replace this with
	// itf.SuiteBuilder.WithSource(staticprov.New(...)).
	cfg := &dbconfig.Config{
		Host:     "localhost",
		Port:     "5432",
		Name:     "iota",
		User:     "postgres",
		Password: "",
	}
	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Skipf("postgres not available at %s: %v", addr, err)
	}
	_ = conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

func assertMetricHasLabel(t *testing.T, family *dto.MetricFamily, key, value string) {
	t.Helper()

	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == key && label.GetValue() == value {
				return
			}
		}
	}

	assert.Failf(t, "missing label", "expected %s=%s on metric family %s", key, value, family.GetName())
}
