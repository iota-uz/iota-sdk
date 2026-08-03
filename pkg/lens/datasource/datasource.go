// Package datasource defines the Lens datasource contracts and request types.
package datasource

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
)

type Capability string

const (
	CapabilityParameterizedQueries Capability = "parameterized_queries"
	CapabilityTransactions         Capability = "transactions"
	CapabilityTimeouts             Capability = "timeouts"
	CapabilityQueryKinds           Capability = "query_kinds"
)

type CapabilitySet map[Capability]bool

func (c CapabilitySet) Has(capability Capability) bool {
	if c == nil {
		return false
	}
	return c[capability]
}

type TimeRangeMode string

const (
	TimeRangeModeDefault  TimeRangeMode = "default"
	TimeRangeModeBounded  TimeRangeMode = "bounded"
	TimeRangeModeAll      TimeRangeMode = "all"
	TimeRangeModeRelative TimeRangeMode = "relative"
)

type TimeRange struct {
	Start *time.Time    `json:"start,omitempty"`
	End   *time.Time    `json:"end,omitempty"`
	Mode  TimeRangeMode `json:"mode,omitempty"`
}

type QueryKind string

const (
	QueryKindRaw   QueryKind = "raw"
	QueryKindNamed QueryKind = "named"
)

// ExecutionClass describes why Lens is executing a datasource query. It is
// derived by the SDK scheduler from the active surface and layout; dashboard
// authors never declare it. Datasources may use it to select an equivalent
// lower-latency implementation for critical work, but it is not part of query
// result identity.
type ExecutionClass string

const (
	ExecutionClassInteractiveCritical ExecutionClass = "interactive_critical"
	ExecutionClassRootCritical        ExecutionClass = "root_critical"
	ExecutionClassActiveBelowFold     ExecutionClass = "active_below_fold"
	ExecutionClassIntentPrefetch      ExecutionClass = "intent_prefetch"
	ExecutionClassIdlePrefetch        ExecutionClass = "idle_prefetch"
	ExecutionClassExport              ExecutionClass = "export"
)

type QueryRequest struct {
	Source    string            `json:"source"`
	Text      string            `json:"text"`
	Params    map[string]any    `json:"params,omitempty"`
	TimeRange TimeRange         `json:"time_range,omitempty"`
	Timezone  string            `json:"timezone,omitempty"`
	Locale    string            `json:"locale,omitempty"`
	MaxRows   int               `json:"max_rows,omitempty"`
	Kind      QueryKind         `json:"kind,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	// ExecutionClass is scheduling metadata, not a semantic query parameter.
	ExecutionClass ExecutionClass `json:"execution_class,omitempty"`
}

type DataSource interface {
	Run(ctx context.Context, req QueryRequest) (*frame.FrameSet, error)
	Capabilities() CapabilitySet
}
