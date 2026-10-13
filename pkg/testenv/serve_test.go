package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type deadlineAdapter struct {
	adapterStub
	remaining chan time.Duration
}

func (a *deadlineAdapter) Start(ctx context.Context, spec Spec, id string) (Descriptor, error) {
	deadline, _ := ctx.Deadline()
	a.remaining <- time.Until(deadline)
	return a.adapterStub.Start(ctx, spec, id)
}

func TestServeOperationDeadline(t *testing.T) {
	t.Parallel()
	// False green if parent deadlines match requested limits, hiding an ignored operation timeout.
	for _, tc := range []struct {
		name    string
		options ServeOptions
		parent  time.Duration
		want    time.Duration
	}{
		{"default", ServeOptions{}, time.Hour, time.Minute},
		{"development", ServeOptions{OperationTimeout: 150 * time.Second}, time.Hour, 150 * time.Second},
		{"parent cancellation", ServeOptions{OperationTimeout: 150 * time.Second}, 10 * time.Second, 10 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &deadlineAdapter{adapterStub: adapterStub{capability: true}, remaining: make(chan time.Duration, 1)}
			input, err := json.Marshal(controlRequest{ID: "start", Operation: "start", Spec: testSpec()})
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), tc.parent)
			defer cancel()
			var output bytes.Buffer
			require.NoError(t, ServeWithOptions(ctx, NewCoordinator(a), bytes.NewReader(append(input, '\n')), &output, tc.options))
			var reply response
			require.NoError(t, json.Unmarshal(output.Bytes(), &reply))
			require.Nil(t, reply.Error)
			require.InDelta(t, tc.want.Seconds(), (<-a.remaining).Seconds(), 1)
			require.EqualValues(t, 1, a.stops.Load())
		})
	}
	a := &adapterStub{}
	err := ServeWithOptions(t.Context(), NewCoordinator(a), strings.NewReader(""), &bytes.Buffer{}, ServeOptions{OperationTimeout: -time.Second})
	var control *Error
	require.ErrorAs(t, err, &control)
	require.Equal(t, "invalid_input", control.Code)
	require.Zero(t, a.starts.Load())
}

func TestServeEOFCleansResourcesAndReportsInputError(t *testing.T) {
	// Falsely green if EOF returns before the owned adapter's Stop is observed.
	a := &adapterStub{capability: true}
	c := NewCoordinator(a)
	input, err := json.Marshal(controlRequest{ID: "one", Operation: "start", Spec: testSpec()})
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, Serve(context.Background(), c, bytes.NewReader(append(input, '\n')), &output))
	require.EqualValues(t, 1, a.starts.Load())
	require.EqualValues(t, 1, a.stops.Load())
	var reply response
	require.NoError(t, json.Unmarshal(output.Bytes(), &reply))
	require.Nil(t, reply.Error)
	require.NotNil(t, reply.Descriptor)
	require.Error(t, Serve(context.Background(), NewCoordinator(a), strings.NewReader(strings.Repeat("x", 2<<20)), &bytes.Buffer{}))
}
