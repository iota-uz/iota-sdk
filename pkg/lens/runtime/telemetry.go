package runtime

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	datasourceSaturationOnce sync.Once
	datasourceSaturation     metric.Int64Histogram
)

func (r *Runtime) changeDatasourceActivity(ctx context.Context, source string, delta int) {
	r.activityMu.Lock()
	r.activeBySource[source] += delta
	active := r.activeBySource[source]
	if active == 0 {
		delete(r.activeBySource, source)
	}
	r.activityMu.Unlock()

	datasourceSaturationOnce.Do(func() {
		datasourceSaturation, _ = otel.Meter("github.com/iota-uz/iota-sdk/lens/runtime").
			Int64Histogram("lens.datasource_saturation", metric.WithUnit("{query}"))
	})
	datasourceSaturation.Record(ctx, int64(active), metric.WithAttributes(attribute.String("source", source)))
	if observer, ok := r.observer.(DatasourceObserver); ok {
		observer.DatasourceSaturation(source, active)
	}
}
