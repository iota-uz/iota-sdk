package testenv

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type adapterStub struct {
	starts, stops atomic.Int32
	readyErr      error
	capability    bool
	started       *Gate
}

func (a *adapterStub) Start(ctx context.Context, s Spec, id string) (Descriptor, error) {
	a.starts.Add(1)
	if a.started != nil {
		if err := a.started.Wait(ctx); err != nil {
			return Descriptor{EnvironmentID: id}, err
		}
	}
	d := Descriptor{EnvironmentID: id, BaseURL: "http://localhost:1234", BuildRevision: "revision", SchemaFingerprint: s.SchemaFingerprint, BaselineFingerprint: s.BaselineFingerprint, ArtifactDirectory: "/owned"}
	if a.capability {
		d.Capabilities = []string{"nats"}
	}
	return d, nil
}
func (a *adapterStub) Ready(context.Context, Descriptor) error { return a.readyErr }
func (a *adapterStub) Stop(context.Context, Descriptor) error  { a.stops.Add(1); return nil }
func testSpec() Spec {
	return Spec{RunID: "run", Slot: "worker-1", Isolation: "worker", SchemaFingerprint: "schema", BaselineFingerprint: "baseline", RequiredCapabilities: []string{"nats"}}
}
func TestCoordinatorConcurrentStartAndOwnedStop(t *testing.T) {
	// Falsely green if a fresh coordinator is used for every concurrent start.
	a := &adapterStub{capability: true, started: NewGate()}
	c := NewCoordinator(a)
	var wg sync.WaitGroup
	results := make(chan Descriptor, 12)
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := c.Start(context.Background(), testSpec())
			results <- d
			errs <- err
		}()
	}
	<-a.started.Entered()
	a.started.Release()
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var id string
	for d := range results {
		if id == "" {
			id = d.EnvironmentID
		}
		require.Equal(t, id, d.EnvironmentID)
	}
	require.EqualValues(t, 1, a.starts.Load())
	s := testSpec()
	s.BaselineFingerprint = "other"
	_, err := c.Start(context.Background(), s)
	var control *Error
	require.ErrorAs(t, err, &control)
	require.Equal(t, "resource_conflict", control.Code)
	require.Error(t, c.Stop(context.Background(), "foreign"))
	require.Zero(t, a.stops.Load())
	require.NoError(t, c.Stop(context.Background(), id))
	require.NoError(t, c.Stop(context.Background(), id))
	require.EqualValues(t, 1, a.stops.Load())
}
func TestCoordinatorCompensatesStartupFailure(t *testing.T) {
	// Falsely green if the adapter is never checked for cleanup after readiness failure.
	for _, a := range []*adapterStub{{readyErr: errors.New("not ready")}, {capability: false}} {
		c := NewCoordinator(a)
		_, err := c.Start(context.Background(), testSpec())
		require.Error(t, err)
		require.EqualValues(t, 1, a.stops.Load())
	}
}
