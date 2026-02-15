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

type Metrics interface {
	OnSearch(req SearchRequest, telemetry SearchTelemetry)
	OnQueue(tenantID uuid.UUID, language string, enqueued bool, queueSize int)
	OnReindex(tenantID uuid.UUID, language string, took time.Duration, err error)
	OnOutboxPoll(took time.Duration, err error)
	OnWatch(provider string, tenantID uuid.UUID, eventType string, err error)
}

type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (m *NoopMetrics) OnSearch(SearchRequest, SearchTelemetry) {}

func (m *NoopMetrics) OnQueue(uuid.UUID, string, bool, int) {}

func (m *NoopMetrics) OnReindex(uuid.UUID, string, time.Duration, error) {}

func (m *NoopMetrics) OnOutboxPoll(time.Duration, error) {}

func (m *NoopMetrics) OnWatch(string, uuid.UUID, string, error) {}
