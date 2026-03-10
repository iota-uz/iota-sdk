// Package middleware provides this package.
package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type statusCaptureWriter struct {
	http.ResponseWriter
	statusCode    int
	statusWritten bool
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	// Always forward 1xx informational responses without latching.
	if code < 200 {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if !w.statusWritten {
		w.statusCode = code
		w.statusWritten = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusCaptureWriter) markStatus(code int) {
	if !w.statusWritten {
		w.statusCode = code
		w.statusWritten = true
	}
}

// Status returns the HTTP status code written to the response.
// Defaults to 200 if WriteHeader was never called explicitly.
func (w *statusCaptureWriter) Status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}

	return w.statusCode
}

func (w *statusCaptureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusCaptureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	const op = serrors.Op("middleware.statusCaptureWriter.Hijack")

	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, serrors.E(op, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker"))
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, serrors.E(op, err)
	}

	w.markStatus(http.StatusSwitchingProtocols)
	return conn, rw, nil
}

func wrapStatusCaptureWriter(w http.ResponseWriter) *statusCaptureWriter {
	return &statusCaptureWriter{ResponseWriter: w}
}
