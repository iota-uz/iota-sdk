package middleware

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// pgxPoolCollector implements prometheus.Collector by snapshotting
// pgxpool.Pool.Stat() on each scrape.
type pgxPoolCollector struct {
	pool   *pgxpool.Pool
	labels prometheus.Labels

	// Gauges (point-in-time snapshot)
	acquiredConns *prometheus.Desc
	idleConns     *prometheus.Desc
	totalConns    *prometheus.Desc
	maxConns      *prometheus.Desc

	constructingConns *prometheus.Desc

	// Counters (monotonically increasing)
	acquireCount            *prometheus.Desc
	acquireDuration         *prometheus.Desc
	emptyAcquireCount       *prometheus.Desc
	canceledAcquireCount    *prometheus.Desc
	newConns                *prometheus.Desc
	maxLifetimeDestroyCount *prometheus.Desc
	maxIdleDestroyCount     *prometheus.Desc
}

// NewPgxPoolCollector returns a prometheus.Collector that exports pgxpool
// connection-pool statistics. Register it with prometheus.DefaultRegisterer
// (or any custom registry).
func NewPgxPoolCollector(pool *pgxpool.Pool, constLabels prometheus.Labels) prometheus.Collector {
	if pool == nil {
		panic("NewPgxPoolCollector: pool must not be nil")
	}

	labels := cloneLabels(constLabels)
	return &pgxPoolCollector{
		pool:   pool,
		labels: labels,
		acquiredConns: prometheus.NewDesc(
			"pgxpool_acquired_conns",
			"Number of currently acquired connections in the pool.",
			nil, labels,
		),
		idleConns: prometheus.NewDesc(
			"pgxpool_idle_conns",
			"Number of currently idle connections in the pool.",
			nil, labels,
		),
		totalConns: prometheus.NewDesc(
			"pgxpool_total_conns",
			"Total number of connections currently in the pool.",
			nil, labels,
		),
		maxConns: prometheus.NewDesc(
			"pgxpool_max_conns",
			"Maximum number of connections allowed in the pool.",
			nil, labels,
		),
		constructingConns: prometheus.NewDesc(
			"pgxpool_constructing_conns",
			"Number of connections currently being established.",
			nil, labels,
		),
		acquireCount: prometheus.NewDesc(
			"pgxpool_acquire_count_total",
			"Cumulative count of successful connection acquisitions from the pool.",
			nil, labels,
		),
		acquireDuration: prometheus.NewDesc(
			"pgxpool_acquire_duration_seconds_total",
			"Total time spent acquiring connections from the pool, in seconds.",
			nil, labels,
		),
		emptyAcquireCount: prometheus.NewDesc(
			"pgxpool_empty_acquire_count_total",
			"Cumulative count of acquires that had to create a new connection because the pool was empty.",
			nil, labels,
		),
		canceledAcquireCount: prometheus.NewDesc(
			"pgxpool_canceled_acquire_count_total",
			"Cumulative count of acquires that were canceled by the caller.",
			nil, labels,
		),
		newConns: prometheus.NewDesc(
			"pgxpool_new_conns_total",
			"Cumulative count of new connections opened by the pool.",
			nil, labels,
		),
		maxLifetimeDestroyCount: prometheus.NewDesc(
			"pgxpool_max_lifetime_destroy_count_total",
			"Cumulative count of connections destroyed because they exceeded max lifetime.",
			nil, labels,
		),
		maxIdleDestroyCount: prometheus.NewDesc(
			"pgxpool_max_idle_destroy_count_total",
			"Cumulative count of connections destroyed because they exceeded max idle time.",
			nil, labels,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *pgxPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquiredConns
	ch <- c.idleConns
	ch <- c.totalConns
	ch <- c.maxConns
	ch <- c.constructingConns
	ch <- c.acquireCount
	ch <- c.acquireDuration
	ch <- c.emptyAcquireCount
	ch <- c.canceledAcquireCount
	ch <- c.newConns
	ch <- c.maxLifetimeDestroyCount
	ch <- c.maxIdleDestroyCount
}

// Collect implements prometheus.Collector.
func (c *pgxPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(c.acquiredConns, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(stat.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.constructingConns, prometheus.GaugeValue, float64(stat.ConstructingConns()))
	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(stat.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireDuration, prometheus.CounterValue, stat.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(c.emptyAcquireCount, prometheus.CounterValue, float64(stat.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.canceledAcquireCount, prometheus.CounterValue, float64(stat.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.newConns, prometheus.CounterValue, float64(stat.NewConnsCount()))
	ch <- prometheus.MustNewConstMetric(c.maxLifetimeDestroyCount, prometheus.CounterValue, float64(stat.MaxLifetimeDestroyCount()))
	ch <- prometheus.MustNewConstMetric(c.maxIdleDestroyCount, prometheus.CounterValue, float64(stat.MaxIdleDestroyCount()))
}
