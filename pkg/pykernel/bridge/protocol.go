package bridge

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// JSON-RPC method names exchanged over the channel.
const (
	// host→kernel notifications
	MethodExecSubmit = "exec.submit"
	MethodExecCancel = "exec.cancel"

	// kernel→host request (blocking; expects a response)
	MethodCapCall = "cap.call"

	// kernel→host notifications
	MethodOutStdout  = "out.stdout"
	MethodOutMetric  = "out.metric"
	MethodOutLog     = "out.log"
	MethodExecResult = "exec.result"
	MethodExecError  = "exec.error"
	MethodExecDone   = "exec.done"
)

// JSON-RPC error codes. The reserved range mirrors the spec; capability errors
// use the implementation-defined server range.
const (
	codeMethodNotFound  = -32601
	codeCapabilityError = -32000 // a controlled error to raise inside the kernel
	codeCapabilityArgs  = -32001 // malformed cap.call params
)

// maxFrameSize bounds a single decoded frame to guard against a hostile or
// buggy kernel claiming an enormous length.
const maxFrameSize = 64 << 20 // 64 MiB

// ErrFrameTooLarge is returned when a frame's declared length exceeds the cap.
var ErrFrameTooLarge = errors.New("pykernel/bridge: frame exceeds maximum size")

// jsonrpcVersion is the only supported protocol version string.
const jsonrpcVersion = "2.0"

// message is a JSON-RPC 2.0 envelope. A frame is exactly one message.
//
//   - request:      Method set, ID set
//   - notification: Method set, ID absent
//   - response:     Method absent, ID set, one of Result/Error set
//
// ID is kept raw so a kernel may use a number or string id; the host echoes it
// verbatim in the response.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (m *message) isRequest() bool      { return m.Method != "" && len(m.ID) > 0 }
func (m *message) isNotification() bool { return m.Method != "" && len(m.ID) == 0 }

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// --- Wire payloads -------------------------------------------------------

// host→kernel
type execSubmitParams struct {
	ExecID string `json:"exec_id"`
	Code   string `json:"code"`
	Limits Limits `json:"limits"`
}

type execCancelParams struct {
	ExecID string `json:"exec_id"`
}

// kernel→host request
type capCallParams struct {
	ExecID string          `json:"exec_id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
}

// capErrorData travels in rpcError.Data so the kernel shim can reconstruct the
// correct Python exception class (e.g. PlanModeViolation) at the call site.
type capErrorData struct {
	Type string `json:"type"`
}

// --- Framing -------------------------------------------------------------

// writeFrame serializes msg and writes it as a 4-byte big-endian length prefix
// followed by the JSON payload, in a single Write so frames never interleave
// when multiple goroutines share the writer behind a mutex.
func writeFrame(w io.Writer, msg *message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("pykernel/bridge: marshal frame: %w", err)
	}
	if len(payload) > maxFrameSize {
		return ErrFrameTooLarge
	}
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(payload)))
	copy(buf[4:], payload)
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("pykernel/bridge: write frame: %w", err)
	}
	return nil
}

// readFrame reads one length-prefixed frame and decodes the JSON-RPC message.
func readFrame(r io.Reader) (*message, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err // io.EOF / io.ErrUnexpectedEOF propagate to the caller
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > maxFrameSize {
		return nil, ErrFrameTooLarge
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	var msg message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("pykernel/bridge: decode frame: %w", err)
	}
	return &msg, nil
}
