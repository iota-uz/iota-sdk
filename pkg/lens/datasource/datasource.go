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

func (c CapabilitySet) Has(cap Capability) bool {
	if c == nil {
		return false
	}
	return c[cap]
}

type TimeRange struct {
	Start *time.Time
	End   *time.Time
	Mode  string
}

type QueryRequest struct {
	Source    string
	Text      string
	Params    map[string]any
	TimeRange TimeRange
	Timezone  string
	MaxRows   int
	Kind      string
	Labels    map[string]string
}

type DataSource interface {
	Run(ctx context.Context, req QueryRequest) (*frame.FrameSet, error)
	Capabilities() CapabilitySet
}

type Plugin interface {
	Name() string
	New(config map[string]any) (DataSource, error)
}
