package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
