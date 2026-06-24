package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Limits bounds a single exec. Zero fields mean "use the kernel/policy default".
type Limits struct {
	WallClockMS int64 `json:"wall_clock_ms,omitempty"`
	OutputCap   int   `json:"output_cap,omitempty"`
}

// Call is a kernel→host capability invocation. It deliberately carries no
// tenant, permissions, or run Mode — the CallDispatcher injects those host-side
// from the Session (the host-binds-context invariant).
type Call struct {
	ExecID string
	Name   string
	Args   json.RawMessage
}

// Reply is the host's answer to a Call. Exactly one of Result/Err is meaningful:
// when Err is non-nil it is surfaced inside the kernel as a raised Python
// exception; otherwise Result (already JSON-encoded by the caller) is returned.
type Reply struct {
	Result json.RawMessage
	Err    *CallError
}

// CallError is a controlled failure the kernel should raise at the call site —
// e.g. a plan-mode write refusal or a permission denial. Type names the Python
// exception class for the shim to instantiate.
type CallError struct {
	Type    string
	Message string
}

func (e *CallError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Type == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// CallDispatcher resolves a kernel capability call to a host handler. The
// implementation binds the Session's tenant + Mode and routes through the
// pykernel plan/apply authorization before invoking the capability. Dispatch
// must always return a Reply (never panic); unexpected failures become a
// CallError so the kernel still gets a definite answer.
type CallDispatcher interface {
	Dispatch(ctx context.Context, call Call) Reply
}

// RawEvent is a decoded kernel→host output notification handed to the sink. Kind
// is the short event name ("stdout", "metric", "log", "result", "error",
// "done"); Params is the raw JSON payload for that kind.
type RawEvent struct {
	Kind   string
	Params json.RawMessage
}

// Event kind names used in RawEvent.Kind.
const (
	KindStdout = "stdout"
	KindMetric = "metric"
	KindLog    = "log"
	KindResult = "result"
	KindError  = "error"
	KindDone   = "done"
)

// EventSink receives kernel→host output notifications keyed by exec id. The
// Manager fans these into the per-Exec event channel. Emit must not block the
// bridge read loop for long.
type EventSink interface {
	Emit(execID string, ev RawEvent)
}

// Bridge is the host side of one kernel's control channel.
type Bridge interface {
	// Serve runs the read loop until ctx is cancelled or the connection closes.
	// Inbound cap.call requests are dispatched (and answered) via dispatcher;
	// inbound output notifications are forwarded to sink. Serve returns the
	// terminating error (nil on a clean ctx cancel / EOF after Close).
	Serve(ctx context.Context, dispatcher CallDispatcher, sink EventSink) error
	// Submit sends an exec.submit notification to the kernel.
	Submit(ctx context.Context, execID, code string, limits Limits) error
	// Cancel sends a cooperative exec.cancel notification to the kernel.
	Cancel(ctx context.Context, execID string) error
	// Close closes the underlying connection, unblocking Serve.
	Close() error
}

// New returns a Bridge over conn. conn is typically a per-kernel unix-socket
// connection, but any full-duplex io.ReadWriteCloser works (tests use net.Pipe).
func New(conn io.ReadWriteCloser) Bridge {
	return &bridge{conn: conn}
}

type bridge struct {
	conn io.ReadWriteCloser

	writeMu   sync.Mutex // serializes all frame writes (responses + notifications)
	closeOnce sync.Once
	closeErr  error
}

func (b *bridge) Serve(ctx context.Context, dispatcher CallDispatcher, sink EventSink) error {
	// Closing the connection is the portable way to unblock the blocking
	// ReadFull in readFrame when ctx is cancelled.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = b.Close()
		case <-stop:
		}
	}()

	for {
		msg, err := readFrame(b.conn)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled: the close above caused this read error
			}
			if err == io.EOF {
				return nil // kernel closed cleanly
			}
			return err
		}
		switch {
		case msg.isRequest():
			// The kernel serializes one capability call at a time, so handling
			// it inline keeps response ordering trivial. A hung handler is
			// bounded by the manager's wall-clock timer, which closes conn.
			b.handleCall(ctx, dispatcher, msg)
		case msg.isNotification():
			b.handleNotification(sink, msg)
		default:
			// A bare response with no matching request: ignore.
		}
	}
}

func (b *bridge) handleCall(ctx context.Context, dispatcher CallDispatcher, msg *message) {
	if msg.Method != MethodCapCall {
		b.writeError(msg.ID, codeMethodNotFound, "unknown method: "+msg.Method, "")
		return
	}
	var p capCallParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		b.writeError(msg.ID, codeCapabilityArgs, "invalid cap.call params: "+err.Error(), "")
		return
	}
	reply := dispatcher.Dispatch(ctx, Call(p))
	if reply.Err != nil {
		b.writeError(msg.ID, codeCapabilityError, reply.Err.Message, reply.Err.Type)
		return
	}
	b.writeResult(msg.ID, reply.Result)
}

func (b *bridge) handleNotification(sink EventSink, msg *message) {
	kind, ok := notificationKind(msg.Method)
	if !ok {
		return // unknown notification: ignore
	}
	execID := peekExecID(msg.Params)
	sink.Emit(execID, RawEvent{Kind: kind, Params: msg.Params})
}

// notificationKind maps a kernel→host notification method to a RawEvent kind.
func notificationKind(method string) (string, bool) {
	switch method {
	case MethodOutStdout:
		return KindStdout, true
	case MethodOutMetric:
		return KindMetric, true
	case MethodOutLog:
		return KindLog, true
	case MethodExecResult:
		return KindResult, true
	case MethodExecError:
		return KindError, true
	case MethodExecDone:
		return KindDone, true
	default:
		return "", false
	}
}

// peekExecID extracts only the exec_id from a notification payload so the sink
// can key the event without fully decoding the (kind-specific) params.
func peekExecID(params json.RawMessage) string {
	var head struct {
		ExecID string `json:"exec_id"`
	}
	_ = json.Unmarshal(params, &head)
	return head.ExecID
}

func (b *bridge) Submit(_ context.Context, execID, code string, limits Limits) error {
	params, err := json.Marshal(execSubmitParams{ExecID: execID, Code: code, Limits: limits})
	if err != nil {
		return fmt.Errorf("pykernel/bridge: marshal exec.submit: %w", err)
	}
	return b.writeMessage(&message{JSONRPC: jsonrpcVersion, Method: MethodExecSubmit, Params: params})
}

func (b *bridge) Cancel(_ context.Context, execID string) error {
	params, err := json.Marshal(execCancelParams{ExecID: execID})
	if err != nil {
		return fmt.Errorf("pykernel/bridge: marshal exec.cancel: %w", err)
	}
	return b.writeMessage(&message{JSONRPC: jsonrpcVersion, Method: MethodExecCancel, Params: params})
}

func (b *bridge) Close() error {
	b.closeOnce.Do(func() { b.closeErr = b.conn.Close() })
	return b.closeErr
}

func (b *bridge) writeResult(id, result json.RawMessage) {
	if result == nil {
		result = json.RawMessage("null")
	}
	_ = b.writeMessage(&message{JSONRPC: jsonrpcVersion, ID: id, Result: result})
}

func (b *bridge) writeError(id json.RawMessage, code int, msg, excType string) {
	rerr := &rpcError{Code: code, Message: msg}
	if excType != "" {
		if data, err := json.Marshal(capErrorData{Type: excType}); err == nil {
			rerr.Data = data
		}
	}
	_ = b.writeMessage(&message{JSONRPC: jsonrpcVersion, ID: id, Error: rerr})
}

func (b *bridge) writeMessage(msg *message) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return writeFrame(b.conn, msg)
}
