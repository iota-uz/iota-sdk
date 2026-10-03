package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type failingLifecycle struct {
	adapterStub
	cleanup error
}

func (a *failingLifecycle) Stop(context.Context, Descriptor) error { return a.cleanup }

// Falsely green if successful compensation hides a failing Stop, or no adapter failure is injected.
func TestCoordinatorPreservesPrimaryAndCleanupDiagnostics(t *testing.T) {
	primary, cleanup := errors.New("ready failed"), errors.New("drop failed")
	a := &failingLifecycle{adapterStub: adapterStub{readyErr: primary, capability: true}, cleanup: cleanup}
	d, err := NewCoordinator(a).Start(t.Context(), testSpec())
	require.ErrorIs(t, err, primary)
	require.ErrorIs(t, err, cleanup)
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, "startup_failed", failure.Code)
	require.Equal(t, "start", failure.Operation)
	require.Equal(t, d.EnvironmentID, failure.EnvironmentID)
	require.Equal(t, "/owned", failure.ArtifactDirectory)
	require.Len(t, failure.Causes, 1)
	require.Equal(t, "cleanup_failed", failure.Causes[0].Code)
}

type concurrentLifecycle struct {
	adapterStub
	entered chan string
	release chan struct{}
}

func (a *concurrentLifecycle) Start(ctx context.Context, s Spec, id string) (Descriptor, error) {
	a.entered <- s.Slot
	select {
	case <-a.release:
		return a.adapterStub.Start(ctx, s, id)
	case <-ctx.Done():
		return Descriptor{EnvironmentID: id}, ctx.Err()
	}
}

// Falsely green if separate Serve instances are used, or response bytes are never parsed after concurrent writes.
func TestServeIndependentStartsOverlapAndEOFCleansAll(t *testing.T) {
	a := &concurrentLifecycle{adapterStub: adapterStub{capability: true}, entered: make(chan string, 2), release: make(chan struct{})}
	var input, output bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, slot := range []string{"one", "two"} {
		s := testSpec()
		s.Slot = slot
		require.NoError(t, encoder.Encode(controlRequest{ID: slot, Operation: "start", Spec: s}))
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, NewCoordinator(a), &input, &output) }()
	for range 2 {
		select {
		case <-a.entered:
		case <-ctx.Done():
			close(a.release)
			<-done
			t.Fatal("independent startup serialized")
		}
	}
	close(a.release)
	require.NoError(t, <-done)
	require.EqualValues(t, 2, a.stops.Load())
	decoder := json.NewDecoder(&output)
	ids := map[string]bool{}
	for range 2 {
		var r response
		require.NoError(t, decoder.Decode(&r))
		require.Nil(t, r.Error)
		require.NotNil(t, r.Descriptor)
		ids[r.ID] = true
	}
	require.Len(t, ids, 2)
}

// Falsely green if the wire only preserves code while losing compensation and the partial descriptor.
func TestServeFailureRetainsPartialDescriptorAndCauses(t *testing.T) {
	a := &failingLifecycle{adapterStub: adapterStub{capability: true, readyErr: &Error{Code: "baseline_mismatch", Message: "mismatch"}}, cleanup: errors.New("drop failed")}
	var input, output bytes.Buffer
	require.NoError(t, json.NewEncoder(&input).Encode(controlRequest{ID: "request", Operation: "start", Spec: testSpec()}))
	require.Error(t, Serve(t.Context(), NewCoordinator(a), &input, &output))
	var r response
	require.NoError(t, json.Unmarshal(output.Bytes(), &r))
	require.NotNil(t, r.Descriptor)
	require.Equal(t, "baseline_mismatch", r.Error.Code)
	require.Equal(t, r.Descriptor.EnvironmentID, r.Error.EnvironmentID)
	require.Equal(t, "/owned", r.Error.ArtifactDirectory)
	require.Len(t, r.Error.Causes, 1)
}
