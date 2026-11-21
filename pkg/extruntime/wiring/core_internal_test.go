package wiring

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Options.Events wins; settings it leaves zero come from the runtime
// configuration a Tune produced.
func TestMergeEventsKeepsExplicitSettingsAndFillsTheRest(t *testing.T) {
	got := mergeEvents(
		events.Config{PollInterval: time.Second, MaxAttempts: 9},
		events.Config{PollInterval: time.Minute, BatchSize: 7, MaxAttempts: 2, Retention: time.Hour, Concurrency: 3},
	)
	require.Equal(t, time.Second, got.PollInterval)
	require.Equal(t, 9, got.MaxAttempts)
	require.Equal(t, 7, got.BatchSize)
	require.Equal(t, 3, got.Concurrency)
	require.Equal(t, time.Hour, got.Retention)
}

// Package schedules resolve their owner with the runtime's loader (built over
// Config.PermissionUniverse), not the core's, which an application may replace
// independently; without a runtime the core's loader serves.
func TestPackageScheduleOwnersUseTheRuntimeSubjectLoader(t *testing.T) {
	core := host.SubjectLoaderFunc(func(context.Context, uuid.UUID, int64, spi.SubjectKind) (spi.Subject, error) {
		return spi.Subject{Credential: "core"}, nil
	})
	rtSubjects := &runtime.UserSubjects{Universe: []permission.Permission{}}
	rt := &runtime.Runtime{Subjects: rtSubjects}

	l := lazySubjects{core: core, runtime: func() (*runtime.Runtime, bool) { return rt, true }}
	require.Same(t, rtSubjects, l.loader(), "the runtime's loader, with its permission universe")

	l.runtime = func() (*runtime.Runtime, bool) { return &runtime.Runtime{}, true }
	got, err := l.LoadSubject(context.Background(), uuid.New(), 1, spi.SubjectOwner)
	require.NoError(t, err)
	require.Equal(t, "core", got.Credential, "a runtime without a loader falls back to the core's")

	l.runtime = func() (*runtime.Runtime, bool) { return nil, false }
	got, err = l.LoadSubject(context.Background(), uuid.New(), 1, spi.SubjectOwner)
	require.NoError(t, err)
	require.Equal(t, "core", got.Credential, "no runtime: the core's loader")
}

// The clock a Tune configured on the runtime survives the merge into the
// core's scheduler; Options.Schedules wins when it sets one.
func TestMergeSchedulesKeepsTheRuntimeClock(t *testing.T) {
	at := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	fallback := events.SchedulerConfig{Now: func() time.Time { return at }, Lookback: time.Hour}
	got := mergeSchedules(events.SchedulerConfig{}, fallback)
	require.Equal(t, at, got.Now())
	require.Equal(t, time.Hour, got.Lookback)
	own := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	got = mergeSchedules(events.SchedulerConfig{Now: func() time.Time { return own }}, fallback)
	require.Equal(t, own, got.Now())
}
