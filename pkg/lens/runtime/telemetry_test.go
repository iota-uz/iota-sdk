package runtime

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type saturationObserver struct {
	mu     sync.Mutex
	values []int
}

func (*saturationObserver) SnapshotHit(string)                    {}
func (*saturationObserver) SnapshotMiss(string)                   {}
func (*saturationObserver) DatasetExecuted(string, time.Duration) {}
func (o *saturationObserver) DatasourceSaturation(_ string, active int) {
	o.mu.Lock()
	o.values = append(o.values, active)
	o.mu.Unlock()
}

func TestRuntimeReportsDatasourceSaturation(t *testing.T) {
	observer := &saturationObserver{}
	runtime := New(Options{Observer: observer})
	runtime.changeDatasourceActivity(t.Context(), "analytics", 1)
	runtime.changeDatasourceActivity(t.Context(), "analytics", 1)
	runtime.changeDatasourceActivity(t.Context(), "analytics", -1)
	runtime.changeDatasourceActivity(t.Context(), "analytics", -1)

	require.Equal(t, []int{1, 2, 1, 0}, observer.values)
}
