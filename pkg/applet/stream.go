package applet

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// StreamWriter provides utilities for Server-Sent Events (SSE) streaming.
// It handles proper SSE formatting and flushing for real-time communication
// with React/Next.js applets.
type StreamWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewStreamWriter creates a new StreamWriter from an http.ResponseWriter.
// It sets appropriate SSE headers and verifies that streaming is supported.
// Returns an error if the ResponseWriter doesn't support flushing (required for SSE).
func NewStreamWriter(w http.ResponseWriter) (*StreamWriter, error) {
	const op serrors.Op = "NewStreamWriter"

	// Verify flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, serrors.E(op, serrors.Internal, "http.ResponseWriter does not support flushing")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	return &StreamWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteEvent writes a Server-Sent Event with the given event type and data.
// Format: event: <event>\ndata: <data>\n\n
//
// Example:
//
//	sw.WriteEvent("message", "Hello, world!")
//	// Sends: event: message\ndata: Hello, world!\n\n
func (sw *StreamWriter) WriteEvent(event, data string) error {
	const op serrors.Op = "StreamWriter.WriteEvent"

	_, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return serrors.E(op, err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteJSON writes a Server-Sent Event with JSON-encoded data.
// Format: event: <event>\ndata: <json>\n\n
//
// Example:
//
//	sw.WriteJSON("update", map[string]string{"status": "processing"})
//	// Sends: event: update\ndata: {"status":"processing"}\n\n
func (sw *StreamWriter) WriteJSON(event string, data interface{}) error {
	const op serrors.Op = "StreamWriter.WriteJSON"

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return serrors.E(op, serrors.Internal, "failed to marshal JSON", err)
	}

	return sw.WriteEvent(event, string(jsonData))
}

// WriteDone writes a "done" event to signal the end of the stream.
// This is a convention used by many SSE clients to know when to stop listening.
//
// Example:
//
//	sw.WriteDone()
//	// Sends: event: done\ndata: \n\n
func (sw *StreamWriter) WriteDone() error {
	return sw.WriteEvent("done", "")
}

// WriteError writes an "error" event with the given error message.
// This allows clients to handle errors gracefully.
//
// Example:
//
//	sw.WriteError("Failed to process request")
//	// Sends: event: error\ndata: Failed to process request\n\n
func (sw *StreamWriter) WriteError(errMsg string) error {
	return sw.WriteEvent("error", errMsg)
}

// WriteErrorJSON writes an "error" event with JSON-encoded error details.
//
// Example:
//
//	sw.WriteErrorJSON(map[string]string{
//	    "code": "INVALID_INPUT",
//	    "message": "Invalid session ID",
//	})
//	// Sends: event: error\ndata: {"code":"INVALID_INPUT","message":"Invalid session ID"}\n\n
func (sw *StreamWriter) WriteErrorJSON(errData interface{}) error {
	return sw.WriteJSON("error", errData)
}

// WriteComment writes an SSE comment (ignored by clients, useful for keeping connection alive).
// Format: : <comment>\n
//
// Example:
//
//	sw.WriteComment("keep-alive")
//	// Sends: : keep-alive\n
func (sw *StreamWriter) WriteComment(comment string) error {
	const op serrors.Op = "StreamWriter.WriteComment"

	_, err := fmt.Fprintf(sw.w, ": %s\n", comment)
	if err != nil {
		return serrors.E(op, err)
	}

	sw.flusher.Flush()
	return nil
}
