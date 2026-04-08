package applet

import (
	"time"

	"github.com/iota-uz/applets"
)

type noopMetricsRecorder struct{}

func (noopMetricsRecorder) RecordDuration(string, time.Duration, map[string]string) {}

func (noopMetricsRecorder) IncrementCounter(string, map[string]string) {}

func NewNoopMetricsRecorder() applets.MetricsRecorder {
	return noopMetricsRecorder{}
}
