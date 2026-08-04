// Package spotlight provides this package.
package spotlight

import (
	"time"

	"github.com/google/uuid"
)

type SearchTelemetry struct {
	TotalHits      int
	AuthorizedHits int
	CacheHit       bool
	CacheStale     bool
	EngineTook     time.Duration
	ACLTook        time.Duration
	RankTook       time.Duration
	GroupTook      time.Duration
	AgentTook      time.Duration
	TotalTook      time.Duration
	Budget         time.Duration
	OverBudget     bool
	Err            error
}

// ProviderSyncResult is the outcome an indexer reports per provider so a
// concrete Metrics impl can bucket counters by result type. Issue #2810
// §3.4 / §5.
type ProviderSyncResult string

const (
	ProviderSyncOK     ProviderSyncResult = "ok"
	ProviderSyncFailed ProviderSyncResult = "failed"
	ProviderSyncCapped ProviderSyncResult = "capped"
)

// EngineErrorClass categorizes the failures the Prometheus counters
// `spotlight_engine_errors_total{type=...}` discriminate on. Issue #2810
// §5.
type EngineErrorClass string

const (
	EngineErrorPayloadTooLarge EngineErrorClass = "413"
	EngineErrorRateLimit       EngineErrorClass = "429"
	EngineErrorServer          EngineErrorClass = "5xx"
	EngineErrorClientDrop      EngineErrorClass = "4xx_drop"
	EngineErrorNullScan        EngineErrorClass = "null_scan"
	EngineErrorFilterTooLong   EngineErrorClass = "filter_too_long"
	EngineErrorOther           EngineErrorClass = "other"
)

type Metrics interface {
	OnSearch(req SearchRequest, telemetry SearchTelemetry)
	OnQueue(tenantID uuid.UUID, language string, enqueued bool, queueSize int)
	OnReindex(tenantID uuid.UUID, language string, took time.Duration, err error)
	OnOutboxPoll(took time.Duration, err error)

	// OnProviderSync records the outcome of a single provider sync run.
	// Issue #2810 §3.4 surface for "N of M providers failed".
	OnProviderSync(providerID string, result ProviderSyncResult, docCount int, took time.Duration)

	// OnEngineError increments the engine error counter, bucketed by
	// EngineErrorClass. Wire this from any classifier-aware code path.
	OnEngineError(providerID string, class EngineErrorClass)

	// OnDocumentObserved is invoked per upserted document with its
	// approximate JSON-marshalled payload size. Powers the
	// spotlight_doc_size_bytes histogram (§5) and feeds the early-warning
	// path for 413 risk.
	OnDocumentObserved(providerID string, sizeBytes int)

	// OnEventDedupDropped counts events the projector layer suppressed
	// as duplicates via the EventDeduper.
	OnEventDedupDropped(providerID string)

	// OnBreakerState reports the current state of the engine circuit
	// breaker for the named engine. Used to drive a gauge in Prometheus.
	OnBreakerState(engineName string, state BreakerState)

	// OnAccessFilterSize feeds the spotlight_access_filter_bytes
	// histogram (§2.7). Useful for spotting admins approaching the
	// 16 KiB hard cap.
	OnAccessFilterSize(sizeBytes int)
}

type NoopMetrics struct{}

var _ Metrics = (*NoopMetrics)(nil)

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (m *NoopMetrics) OnSearch(SearchRequest, SearchTelemetry) {}

func (m *NoopMetrics) OnQueue(uuid.UUID, string, bool, int) {}

func (m *NoopMetrics) OnReindex(uuid.UUID, string, time.Duration, error) {}

func (m *NoopMetrics) OnOutboxPoll(time.Duration, error) {}

func (m *NoopMetrics) OnProviderSync(string, ProviderSyncResult, int, time.Duration) {}

func (m *NoopMetrics) OnEngineError(string, EngineErrorClass) {}

func (m *NoopMetrics) OnDocumentObserved(string, int) {}

func (m *NoopMetrics) OnEventDedupDropped(string) {}

func (m *NoopMetrics) OnBreakerState(string, BreakerState) {}

func (m *NoopMetrics) OnAccessFilterSize(int) {}
