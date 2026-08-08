package document

import (
	"context"
	"errors"
	"net"
	"strings"
)

// FailureReason is the closed vocabulary a reader-facing message is chosen
// from. It says what kind of thing went wrong, never which query, table or
// metric produced it: the runtime is generic, so the vocabulary has to be too,
// and anything narrower would leak a host's business concepts into the wire
// contract. Keep it small — a reason only earns a place here when the reader
// can be told something different and true because of it.
type FailureReason string

const (
	// FailureTimeout is work that was still running when its deadline passed.
	// For a reader that means the slice they asked for was too large to finish
	// in time, and a narrower one is the thing to try.
	FailureTimeout FailureReason = "timeout"
	// FailureCanceled is work abandoned before it finished, which is what
	// leaving the page or asking a new question does to the question in flight.
	// It is not a fault and must not be reported as one.
	FailureCanceled FailureReason = "canceled"
	// FailureUnknown is everything else: real but unclassified, and therefore
	// only worth the generic sentence.
	FailureUnknown FailureReason = "unknown"
)

// ClassifyFailure maps an execution error onto the reason vocabulary.
//
// Sentinel matching comes first and is the only exact answer. It is not the
// only one available, because an error crosses layers that are free to keep the
// text and drop the chain — a memoizer that caches a rendered message, a worker
// that reports a string, a driver that formats its own. The text checks are a
// deliberate second pass over that reality rather than a guess: they only run
// when the chain has already failed to answer.
func ClassifyFailure(err error) FailureReason {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailureTimeout
	}
	if errors.Is(err, context.Canceled) {
		return FailureCanceled
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return FailureTimeout
	}
	text := strings.ToLower(err.Error())
	switch {
	// A bare "timeout" substring also matches "invalid timeout configuration"
	// and similar setup errors that are not a deadline at all. Match the
	// signatures a driver or worker actually reports instead.
	case strings.Contains(text, context.DeadlineExceeded.Error()),
		strings.Contains(text, "statement timeout"),
		strings.Contains(text, "i/o timeout"),
		strings.Contains(text, "query timeout"),
		strings.Contains(text, "request timeout"),
		strings.Contains(text, "timed out"):
		return FailureTimeout
	case strings.Contains(text, context.Canceled.Error()),
		strings.Contains(text, "cancelled"):
		return FailureCanceled
	}
	return FailureUnknown
}
