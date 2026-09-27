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
	k.write(t, &message{JSONRPC: jsonrpcVersion, Method: method, Params: mustMarshal(t, params)})
}

// mustMarshal JSON-encodes v and fails the test on error. It must be called on
// the test goroutine (it uses require).
func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
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

// shortWriter accepts at most one byte per Write to exercise writeFrame's
// short-write loop; everything is appended to buf so the frame can be replayed.
type shortWriter struct{ buf bytes.Buffer }

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return w.buf.Write(p[:1])
}

func TestWriteFrame_ToleratesShortWrites(t *testing.T) {
	t.Parallel()

	in := &message{
		JSONRPC: jsonrpcVersion,
		Method:  MethodOutStdout,
		Params:  json.RawMessage(`{"exec_id":"e1","chunk":"a longer chunk so the frame spans many bytes"}`),
	}
	w := &shortWriter{}
	require.NoError(t, writeFrame(w, in))

	// The full frame must have been delivered intact and round-trip cleanly.
	out, err := readFrame(&w.buf)
	require.NoError(t, err)
	assert.Equal(t, in.Method, out.Method)
	assert.JSONEq(t, string(in.Params), string(out.Params))
}

func TestBridge_CapCallAndStream(t *testing.T) {
	t.Parallel()

	hostConn, kernConn := net.Pipe()
	k := &fakeKernel{conn: kernConn}

	var gotCall Call
	// Precompute the reply on the test goroutine so the dispatcher (invoked from
	// the Serve goroutine) need not marshal/assert off-goroutine.
	res := mustMarshal(t, []map[string]any{{"n": 1}})
	disp := funcDispatcher(func(_ context.Context, c Call) Reply {
		gotCall = c
		return Reply{Result: res}
	})
	sink := chanSink{ch: make(chan sinkItem, 8)}

	b := New(hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- b.Serve(ctx, disp, sink) }()

	// Submit blocks until the kernel reads the frame off the pipe, so run it in a
	// goroutine and channel its error back to the main goroutine. All require/assert
	// calls (including the k.read/k.write/k.notify helpers) stay on this goroutine.
	submitErr := make(chan error, 1)
	go func() { submitErr <- b.Submit(ctx, "e1", "print(sql('select 1'))", Limits{}) }()

	// 1. receive exec.submit.
	m := k.read(t)
	assert.Equal(t, MethodExecSubmit, m.Method)
	var sp execSubmitParams
	require.NoError(t, json.Unmarshal(m.Params, &sp))
	assert.Equal(t, "e1", sp.ExecID)
	require.NoError(t, <-submitErr)

	// 2. issue a blocking cap.call and read the host's response.
	args := mustMarshal(t, map[string]any{"query": "select 1"})
	cp := mustMarshal(t, capCallParams{ExecID: "e1", Name: "sql", Args: args})
	k.write(t, &message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(`1`), Method: MethodCapCall, Params: cp})
	resp := k.read(t)
	require.Nil(t, resp.Error)
	assert.JSONEq(t, `[{"n":1}]`, string(resp.Result))
	assert.JSONEq(t, `1`, string(resp.ID))

	// 3. stream output then terminate.
	k.notify(t, MethodOutStdout, map[string]any{"exec_id": "e1", "chunk": "hi"})
	k.notify(t, MethodExecResult, map[string]any{"exec_id": "e1", "text": "None"})
	k.notify(t, MethodExecDone, map[string]any{"exec_id": "e1"})

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

	// Serve reads on its own goroutine, so this kernel-side dialog can run on the
	// test goroutine: writes unblock as Serve reads, and the assertions stay here.
	cp := mustMarshal(t, capCallParams{ExecID: "e1", Name: "pg_upsert", Args: json.RawMessage(`{}`)})
	k.write(t, &message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(`7`), Method: MethodCapCall, Params: cp})

	resp := k.read(t)
	require.NotNil(t, resp.Error)
	assert.Equal(t, codeCapabilityError, resp.Error.Code)
	assert.Equal(t, "write refused in plan mode", resp.Error.Message)
	var d capErrorData
	require.NoError(t, json.Unmarshal(resp.Error.Data, &d))
	assert.Equal(t, "PlanModeViolation", d.Type)
	assert.JSONEq(t, `7`, string(resp.ID))
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

	// Cancel blocks until the kernel reads the frame off the pipe; run it in a
	// goroutine and assert on the test goroutine.
	cancelErr := make(chan error, 1)
	go func() { cancelErr <- b.Cancel(ctx, "e1") }()

	m := k.read(t)
	assert.Equal(t, MethodExecCancel, m.Method)
	var cp execCancelParams
	require.NoError(t, json.Unmarshal(m.Params, &cp))
	assert.Equal(t, "e1", cp.ExecID)
	require.NoError(t, <-cancelErr)
}
