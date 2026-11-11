package pool

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrWorkerLost is matched by every invocation failure caused by the worker
	// process dying or being killed while the invocation was in flight. The
	// invocation is never re-run by the pool: its effects may be partial.
	ErrWorkerLost = errors.New("extruntime: worker lost")
	// ErrProtocol is matched when a worker announces or speaks an unsupported
	// protocol.
	ErrProtocol = errors.New("extruntime: unsupported worker protocol")
	// ErrArtifactMissing is matched when the artifact directory or its
	// server.mjs does not exist.
	ErrArtifactMissing = errors.New("extruntime: artifact missing")
	// ErrExportMissing is matched when server.mjs lacks the requested export or
	// the export is not an implementation of the requested kind.
	ErrExportMissing = errors.New("extruntime: export missing")
	// ErrPoolClosed is returned for invocations submitted after Close.
	ErrPoolClosed = errors.New("extruntime: pool closed")
	// ErrQueueFull is returned when all workers are busy and the queue is at
	// its configured limit.
	ErrQueueFull = errors.New("extruntime: invocation queue full")
)

// ErrorKind is the closed set of error kinds crossing the boundary in both
// directions. KindSuspend is a control signal of the workflow engine, not a
// failure.
type ErrorKind string

const (
	ErrKindDomain     ErrorKind = "domain"
	ErrKindValidation ErrorKind = "validation"
	ErrKindDenied     ErrorKind = "denied"
	ErrKindInternal   ErrorKind = "internal"
	ErrKindSuspend    ErrorKind = "suspend"
)

// Codes of runtime-originated internal errors.
const (
	CodeArtifactMissing = "runtime.artifactMissing"
	CodeExportMissing   = "runtime.exportMissing"
	CodeProtocol        = "runtime.protocol"
	CodeHostCall        = "runtime.hostCall"
	CodeSuspended       = "runtime.suspended"
	CodeCanceled        = "runtime.canceled"
)

// Issue is one validation issue (docs §2.3).
type Issue struct {
	Code   string          `json:"code"`
	Phase  string          `json:"phase,omitempty"`
	Source string          `json:"source,omitempty"`
	Path   json.RawMessage `json:"path,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// WireError is the structured error that crosses the Go/Node boundary in both
// directions without losing code, params or issues.
type WireError struct {
	Kind    ErrorKind       `json:"kind"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Issues  []Issue         `json:"issues,omitempty"`
	Suspend json.RawMessage `json:"suspend,omitempty"`
	Stack   string          `json:"stack,omitempty"`
}

func (e *WireError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("%s %s: %s", e.Kind, e.Code, e.Message)
	case e.Code != "":
		return fmt.Sprintf("%s %s", e.Kind, e.Code)
	case e.Message != "":
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	default:
		return string(e.Kind)
	}
}

// Is lets runtime-originated codes match the package sentinels.
func (e *WireError) Is(target error) bool {
	switch target {
	case ErrArtifactMissing:
		return e.Code == CodeArtifactMissing
	case ErrExportMissing:
		return e.Code == CodeExportMissing
	case ErrProtocol:
		return e.Code == CodeProtocol
	}
	return false
}

// Internal builds an internal wire error.
func Internal(code, message string) *WireError {
	return &WireError{Kind: ErrKindInternal, Code: code, Message: message}
}

// AsWireError returns the structured error carried by err, if any.
func AsWireError(err error) (*WireError, bool) {
	var we *WireError
	if errors.As(err, &we) {
		return we, true
	}
	return nil, false
}

// IsSuspend reports whether err is the workflow suspend signal.
func IsSuspend(err error) bool {
	we, ok := AsWireError(err)
	return ok && we.Kind == ErrKindSuspend
}

// WorkerLostError is the typed diagnostic of an invocation whose worker died.
type WorkerLostError struct {
	Worker     int
	PID        int
	Invocation string
	Cause      string
	Stderr     string
}

func (e *WorkerLostError) Error() string {
	msg := fmt.Sprintf("extruntime: worker %d (pid %d) lost during invocation %s: %s", e.Worker, e.PID, e.Invocation, e.Cause)
	if e.Stderr != "" {
		msg += "; stderr: " + e.Stderr
	}
	return msg
}

func (e *WorkerLostError) Is(target error) bool { return target == ErrWorkerLost }

// toWire converts any error returned by a host-call handler into a wire error.
func toWire(err error) *WireError {
	if we, ok := AsWireError(err); ok {
		return we
	}
	return Internal(CodeHostCall, err.Error())
}
