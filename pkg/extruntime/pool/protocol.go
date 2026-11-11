// Package pool runs a fixed-size set of warm Node.js worker processes and
// speaks the iota.runtime/1 wire protocol with them (docs/extension-runtime.md
// §3): 4-byte big-endian length frames carrying UTF-8 JSON over the worker's
// stdio.
package pool

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ProtocolVersion is the only protocol a worker may announce.
const ProtocolVersion = "iota.runtime/1"

// Kind selects how the worker interprets an invocation's export.
type Kind string

const (
	KindCommand      Kind = "command"
	KindAtomic       Kind = "atomic"
	KindOperation    Kind = "operation"
	KindQuery        Kind = "query"
	KindSubscription Kind = "subscription"
	KindWorkflow     Kind = "workflow"
	KindGuard        Kind = "guard"
	KindTemplate     Kind = "template"
	KindEndpoint     Kind = "endpoint"
	KindSchedule     Kind = "schedule"
)

// Artifact is an immutable directory addressed by content hash. The worker
// imports <Dir>/server.mjs once per Hash and keeps every hash it has seen.
type Artifact struct {
	Hash string `json:"hash"`
	Dir  string `json:"dir"`
}

// UserContext is the acting subject; the worker builds ctx.user (Principal)
// from it. ID is the int64 user id as a decimal string (wire int64).
type UserContext struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Locale      string   `json:"locale,omitempty"`
	Permissions []string `json:"permissions"`
}

// InvokeContext is the "ctx" member of an invoke frame.
type InvokeContext struct {
	Tenant     string          `json:"tenant"`
	User       UserContext     `json:"user"`
	Invocation string          `json:"invocation"`
	Trigger    string          `json:"trigger"`
	Parent     string          `json:"parent,omitempty"`
	Instance   string          `json:"instance,omitempty"`
	Occurrence json.RawMessage `json:"occurrence,omitempty"`
	Deadline   string          `json:"deadline,omitempty"`
	Tx         string          `json:"tx,omitempty"`
}

type helloFrame struct {
	T        string `json:"t"`
	Protocol string `json:"protocol"`
}

type readyFrame struct {
	Protocol string `json:"protocol"`
	PID      int    `json:"pid"`
}

type invokeFrame struct {
	T        string          `json:"t"`
	ID       string          `json:"id"`
	Artifact Artifact        `json:"artifact"`
	Kind     Kind            `json:"kind"`
	Export   string          `json:"export"`
	Resource string          `json:"resource"`
	Input    json.RawMessage `json:"input"`
	Ctx      InvokeContext   `json:"ctx"`
}

type callFrame struct {
	Inv    string          `json:"inv"`
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args"`
}

type replyFrame struct {
	T     string          `json:"t"`
	Inv   string          `json:"inv"`
	ID    uint64          `json:"id"`
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error *WireError      `json:"error,omitempty"`
}

type doneFrame struct {
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error *WireError      `json:"error,omitempty"`
}

type cancelFrame struct {
	T  string `json:"t"`
	ID string `json:"id"`
}

type logFrame struct {
	Inv   string `json:"inv"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func writeFrame(w io.Writer, payload []byte) error {
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	copy(buf[4:], payload)
	_, err := w.Write(buf)
	return err
}

func readFrame(r *bufio.Reader, maxSize int) ([]byte, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint32(head[:]))
	if n > maxSize {
		return nil, fmt.Errorf("frame of %d bytes exceeds limit %d", n, maxSize)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func formatDeadline(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
