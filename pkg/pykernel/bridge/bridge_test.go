package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// funcDispatcher adapts a function to CallDispatcher.
type funcDispatcher func(context.Context, Call) Reply

func (f funcDispatcher) Dispatch(ctx context.Context, c Call) Reply { return f(ctx, c) }

type sinkItem struct {
	execID string
	ev     RawEvent
}

type chanSink struct{ ch chan sinkItem }

func (s chanSink) Emit(execID string, ev RawEvent) { s.ch <- sinkItem{execID, ev} }

type nopSink struct{}

func (nopSink) Emit(string, RawEvent) {}

// fakeKernel speaks the wire protocol from the kernel side over a net.Pipe.
type fakeKernel struct{ conn net.Conn }

func (k *fakeKernel) read(t *testing.T) *message {
	t.Helper()
	m, err := readFrame(k.conn)
	require.NoError(t, err)
	return m
}

func (k *fakeKernel) write(t *testing.T, m *message) {
	t.Helper()
	require.NoError(t, writeFrame(k.conn, m))
}

func (k *fakeKernel) notify(t *testing.T, method string, params any) {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	k.write(t, &message{JSONRPC: jsonrpcVersion, Method: method, Params: raw})
}

func TestFrameRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	in := &message{
		JSONRPC: jsonrpcVersion,
		Method:  MethodOutLog,
		Params:  json.RawMessage(`{"exec_id":"e1","level":"info","message":"hi"}`),
	}
	require.NoError(t, writeFrame(&buf, in))
	out, err := readFrame(&buf)
	require.NoError(t, err)
	assert.Equal(t, in.Method, out.Method)
	assert.JSONEq(t, string(in.Params), string(out.Params))
}

func TestBridge_CapCallAndStream(t *testing.T) {
	t.Parallel()

	hostConn, kernConn := net.Pipe()
	k := &fakeKernel{conn: kernConn}

	var gotCall Call
	disp := funcDispatcher(func(_ context.Context, c Call) Reply {
		gotCall = c
		res, _ := json.Marshal([]map[string]any{{"n": 1}})
		return Reply{Result: res}
	})
	sink := chanSink{ch: make(chan sinkItem, 8)}

	b := New(hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- b.Serve(ctx, disp, sink) }()

	kernDone := make(chan struct{})
	go func() {
		defer close(kernDone)

		// 1. receive exec.submit.
		m := k.read(t)
		assert.Equal(t, MethodExecSubmit, m.Method)
		var sp execSubmitParams
		require.NoError(t, json.Unmarshal(m.Params, &sp))
		assert.Equal(t, "e1", sp.ExecID)

		// 2. issue a blocking cap.call and read the host's response.
		args, _ := json.Marshal(map[string]any{"query": "select 1"})
		cp, _ := json.Marshal(capCallParams{ExecID: "e1", Name: "sql", Args: args})
		k.write(t, &message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(`1`), Method: MethodCapCall, Params: cp})
		resp := k.read(t)
		require.Nil(t, resp.Error)
		assert.JSONEq(t, `[{"n":1}]`, string(resp.Result))
		assert.JSONEq(t, `1`, string(resp.ID))

		// 3. stream output then terminate.
		k.notify(t, MethodOutStdout, map[string]any{"exec_id": "e1", "chunk": "hi"})
		k.notify(t, MethodExecResult, map[string]any{"exec_id": "e1", "text": "None"})
		k.notify(t, MethodExecDone, map[string]any{"exec_id": "e1"})
	}()

	require.NoError(t, b.Submit(ctx, "e1", "print(sql('select 1'))", Limits{}))

	kinds := map[string]bool{}
	for i := 0; i < 3; i++ {
		select {
		case it := <-sink.ch:
			assert.Equal(t, "e1", it.execID)
			kinds[it.ev.Kind] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
	assert.True(t, kinds[KindStdout], "expected a stdout event")
	assert.True(t, kinds[KindResult], "expected a result event")
	assert.True(t, kinds[KindDone], "expected a done event")

	assert.Equal(t, "sql", gotCall.Name)
	assert.Equal(t, "e1", gotCall.ExecID)
	assert.JSONEq(t, `{"query":"select 1"}`, string(gotCall.Args))

	<-kernDone
	cancel()
	require.NoError(t, <-serveErr)
}

func TestBridge_CapCallRefusalSurfacesTypedError(t *testing.T) {
	t.Parallel()

	hostConn, kernConn := net.Pipe()
	k := &fakeKernel{conn: kernConn}

	disp := funcDispatcher(func(_ context.Context, _ Call) Reply {
		return Reply{Err: &CallError{Type: "PlanModeViolation", Message: "write refused in plan mode"}}
	})

	b := New(hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = b.Serve(ctx, disp, nopSink{}) }()

	kernDone := make(chan struct{})
	go func() {
		defer close(kernDone)
		cp, _ := json.Marshal(capCallParams{ExecID: "e1", Name: "pg_upsert", Args: json.RawMessage(`{}`)})
		k.write(t, &message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(`7`), Method: MethodCapCall, Params: cp})

		resp := k.read(t)
		require.NotNil(t, resp.Error)
		assert.Equal(t, codeCapabilityError, resp.Error.Code)
		assert.Equal(t, "write refused in plan mode", resp.Error.Message)
		var d capErrorData
		require.NoError(t, json.Unmarshal(resp.Error.Data, &d))
		assert.Equal(t, "PlanModeViolation", d.Type)
		assert.JSONEq(t, `7`, string(resp.ID))
	}()

	<-kernDone
}

func TestBridge_Cancel(t *testing.T) {
	t.Parallel()

	hostConn, kernConn := net.Pipe()
	k := &fakeKernel{conn: kernConn}

	b := New(hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = b.Serve(ctx, funcDispatcher(func(context.Context, Call) Reply { return Reply{} }), nopSink{})
	}()

	kernDone := make(chan struct{})
	go func() {
		defer close(kernDone)
		m := k.read(t)
		assert.Equal(t, MethodExecCancel, m.Method)
		var cp execCancelParams
		require.NoError(t, json.Unmarshal(m.Params, &cp))
		assert.Equal(t, "e1", cp.ExecID)
	}()

	require.NoError(t, b.Cancel(ctx, "e1"))
	<-kernDone
}
