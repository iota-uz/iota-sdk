package spotlight

import (
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics implements the Metrics interface using promauto so
// instances register automatically with prometheus.DefaultRegisterer.
// Call NewPrometheusMetrics() once at process start and inject via
// WithMetrics(...) into the Spotlight service.
//
// Naming follows the issue #2810 §5 spec; cardinality is kept low by
// labelling on provider only (not on tenant or query). Histograms use
// the default DefBuckets except for sizes where we pick byte buckets
// that match observed document payloads.
type PrometheusMetrics struct {
	searchLatency        *prometheus.HistogramVec
	searchAuthorizedHits *prometheus.HistogramVec
	reindexLatency       *prometheus.HistogramVec
	outboxPollLatency    *prometheus.HistogramVec
	queueDepth           *prometheus.GaugeVec
	providerSync         *prometheus.CounterVec
	providerLatency      *prometheus.HistogramVec
	engineErrors         *prometheus.CounterVec
	docSize              *prometheus.HistogramVec
	dedupDropped         *prometheus.CounterVec
	breakerState         *prometheus.GaugeVec
	accessFilterSize     prometheus.Histogram
	eventToIndexed       *prometheus.HistogramVec
}

// docSizeBuckets covers small CRM rows (~1 KiB) up through worst-case
// long-body insurance documents (~1 MiB) on a log scale.
var docSizeBuckets = []float64{
	512, 1024, 2048, 4096, 8192, 16384, 32768, 65536,
	131072, 262144, 524288, 1048576, 2097152, 4194304,
}

// filterSizeBuckets brackets the 4 KiB warn / 16 KiB cap line.
var filterSizeBuckets = []float64{256, 512, 1024, 2048, 4096, 8192, 16384}

// NewPrometheusMetrics constructs and registers all Spotlight metrics.
// Calling more than once on the same registerer will panic via
// promauto; suitable for singleton wiring.
func NewPrometheusMetrics() *PrometheusMetrics {
	return NewPrometheusMetricsWith(prometheus.DefaultRegisterer)
}

// NewPrometheusMetricsWith allows tests / multi-process setups to use a
// dedicated registry instead of the default global one.
func NewPrometheusMetricsWith(reg prometheus.Registerer) *PrometheusMetrics {
	factory := promauto.With(reg)
	return &PrometheusMetrics{
		searchLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_search_latency_seconds",
			Help:    "End-to-end search latency, including ACL + ranking + agent stages.",
			Buckets: prometheus.DefBuckets,
		}, []string{"cache_hit", "over_budget", "intent"}),
		searchAuthorizedHits: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_search_authorized_hits",
			Help:    "Authorized hits returned per search (after ACL post-filter).",
			Buckets: []float64{0, 1, 5, 10, 20, 50, 100},
		}, []string{"intent"}),
		reindexLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_reindex_latency_seconds",
			Help:    "Tenant reindex latency.",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12),
		}, []string{"result"}),
		outboxPollLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_outbox_poll_latency_seconds",
			Help:    "Outbox poll iteration latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"result"}),
		queueDepth: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "spotlight_queue_depth",
			Help: "Current spotlight enqueue depth per language.",
		}, []string{"language"}),
		providerSync: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "spotlight_provider_sync_total",
			Help: "Provider sync attempts, labelled by outcome.",
		}, []string{"provider", "result"}),
		providerLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_provider_sync_seconds",
			Help:    "Per-provider sync wall time.",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12),
		}, []string{"provider", "result"}),
		engineErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "spotlight_engine_errors_total",
			Help: "Engine errors classified by type (413, 429, 5xx, 4xx_drop, null_scan, filter_too_long, other).",
		}, []string{"provider", "type"}),
		docSize: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_doc_size_bytes",
			Help:    "Approximate JSON-marshalled payload size per upserted document.",
			Buckets: docSizeBuckets,
		}, []string{"provider"}),
		dedupDropped: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "spotlight_dedup_dropped_total",
			Help: "Events suppressed as duplicates by EventDeduper.",
		}, []string{"provider"}),
		breakerState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "spotlight_breaker_state",
			Help: "Circuit breaker state per engine (0=closed, 1=half_open, 2=open).",
		}, []string{"engine"}),
		accessFilterSize: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "spotlight_access_filter_bytes",
			Help:    "Size of the access-control filter string emitted for the Meili query.",
			Buckets: filterSizeBuckets,
		}),
		eventToIndexed: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "spotlight_event_to_indexed_seconds",
			Help:    "Wall time from inbound event to Meili task completion, per provider.",
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 14),
		}, []string{"provider"}),
	}
}

var _ Metrics = (*PrometheusMetrics)(nil)

func (m *PrometheusMetrics) OnSearch(req SearchRequest, telemetry SearchTelemetry) {
	cacheHit := "miss"
	if telemetry.CacheHit {
		cacheHit = "hit"
		if telemetry.CacheStale {
			cacheHit = "stale"
		}
	}
	overBudget := "no"
	if telemetry.OverBudget {
		overBudget = "yes"
	}
	intent := string(req.Intent)
	if intent == "" {
		intent = "unknown"
	}
	m.searchLatency.WithLabelValues(cacheHit, overBudget, intent).Observe(telemetry.TotalTook.Seconds())
	m.searchAuthorizedHits.WithLabelValues(intent).Observe(float64(telemetry.AuthorizedHits))
}

func (m *PrometheusMetrics) OnQueue(_ uuid.UUID, language string, _ bool, queueSize int) {
	if language == "" {
		language = "unknown"
	}
	m.queueDepth.WithLabelValues(language).Set(float64(queueSize))
}

func (m *PrometheusMetrics) OnReindex(_ uuid.UUID, _ string, took time.Duration, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.reindexLatency.WithLabelValues(result).Observe(took.Seconds())
}

func (m *PrometheusMetrics) OnOutboxPoll(took time.Duration, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.outboxPollLatency.WithLabelValues(result).Observe(took.Seconds())
}

func (m *PrometheusMetrics) OnProviderSync(providerID string, result ProviderSyncResult, _ int, took time.Duration) {
	if providerID == "" {
		providerID = "unknown"
	}
	m.providerSync.WithLabelValues(providerID, string(result)).Inc()
	m.providerLatency.WithLabelValues(providerID, string(result)).Observe(took.Seconds())
}

func (m *PrometheusMetrics) OnEngineError(providerID string, class EngineErrorClass) {
	if providerID == "" {
		providerID = "unknown"
	}
	m.engineErrors.WithLabelValues(providerID, string(class)).Inc()
}

func (m *PrometheusMetrics) OnDocumentObserved(providerID string, sizeBytes int) {
	if providerID == "" {
		providerID = "unknown"
	}
	m.docSize.WithLabelValues(providerID).Observe(float64(sizeBytes))
}

func (m *PrometheusMetrics) OnEventDedupDropped(providerID string) {
	if providerID == "" {
		providerID = "unknown"
	}
	m.dedupDropped.WithLabelValues(providerID).Inc()
}

func (m *PrometheusMetrics) OnBreakerState(engineName string, state BreakerState) {
	if engineName == "" {
		engineName = "default"
	}
	m.breakerState.WithLabelValues(engineName).Set(float64(state))
}

func (m *PrometheusMetrics) OnAccessFilterSize(sizeBytes int) {
	m.accessFilterSize.Observe(float64(sizeBytes))
}
