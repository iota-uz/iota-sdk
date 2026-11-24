package sms

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// maxRetryAfter bounds a Retry-After the provider asks for; the engine repeats only within the node's
// deadline anyway.
const maxRetryAfter = time.Hour

// hop is the last request a call made: what is known about whether the provider could have accepted it.
type hop struct {
	send       bool // the request is the one that sends the SMS, not a login or a read
	written    bool // the head of the request left this process: the provider may have acted on it
	status     int  // response status; 0: no response
	retryAfter time.Duration
}

// callTrace records the requests of one call. It is how a connector tells "the provider did not get the
// request" (safe to say unavailable) from "the request may have been accepted" (outcome_unknown), which
// the clients' errors do not tell.
type callTrace struct {
	mu   sync.Mutex
	last hop
	any  bool
}

func (t *callTrace) set(h hop) {
	t.mu.Lock()
	t.last, t.any = h, true
	t.mu.Unlock()
}

func (t *callTrace) update(f func(*hop)) {
	t.mu.Lock()
	f(&t.last)
	t.mu.Unlock()
}

func (t *callTrace) snapshot() hop {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.last
}

type traceKey struct{}

func withTrace(ctx context.Context, t *callTrace) context.Context {
	return context.WithValue(ctx, traceKey{}, t)
}

// tracker is the RoundTripper provider clients send through: it records each request of a call in the
// callTrace of its context (or of fixed, for a client that does not pass a context).
type tracker struct {
	base   http.RoundTripper
	isSend func(*http.Request) bool
	// fixed is the call this tracker serves and its context; set for clients without a context argument.
	fixed *callTrace
	ctx   context.Context
}

func (t *tracker) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.ctx != nil {
		req = req.WithContext(t.ctx)
	}
	tr := t.fixed
	if tr == nil {
		tr, _ = req.Context().Value(traceKey{}).(*callTrace)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if tr == nil {
		return base.RoundTrip(req)
	}
	tr.set(hop{send: t.isSend(req)})
	ctx := httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		WroteHeaders: func() { tr.update(func(h *hop) { h.written = true }) },
		WroteRequest: func(httptrace.WroteRequestInfo) { tr.update(func(h *hop) { h.written = true }) },
	})
	resp, err := base.RoundTrip(req.WithContext(ctx))
	if resp != nil {
		after := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		tr.update(func(h *hop) { h.status, h.retryAfter = resp.StatusCode, after })
	}
	return resp, err
}

// parseRetryAfter reads a Retry-After header: seconds or an HTTP date; 0 when absent or unusable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	var d time.Duration
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
		if n > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		d = time.Duration(n) * time.Second
	} else if at, err := http.ParseTime(v); err == nil {
		d = at.Sub(now)
	}
	return min(max(d, 0), maxRetryAfter)
}

// failure is how a failed call ends.
type failure struct {
	// retry asks the engine to repeat the call later (the provider certainly did not accept it).
	retry      bool
	retryAfter time.Duration
	err        *catalog.CallError
}

// classify says how a call that failed with an error ended, from the last request it made. It is
// conservative about the one thing that matters: only a response that guarantees the SMS was not
// accepted, or a request that never left, is anything but outcome_unknown.
func classify(h hop, detail string) failure {
	call := func(code, msg string) failure {
		if detail != "" {
			msg += ": " + detail
		}
		return failure{err: &catalog.CallError{Code: code, Message: msg}}
	}
	switch {
	case h.status == 0 && h.send && h.written:
		return call(catalog.CallOutcomeUnknown, "the request was sent and no answer came; the SMS may have been sent")
	case h.status == 0:
		return call(catalog.CallUnavailable, "the provider was not reached; nothing was sent")
	case h.status == http.StatusTooManyRequests || h.status == http.StatusRequestTimeout:
		ra := h.retryAfter
		if ra <= 0 {
			ra = time.Millisecond // a repeat after the engine's backoff
		}
		return failure{retry: true, retryAfter: ra}
	case h.status == http.StatusUnauthorized, !h.send && h.status >= 400 && h.status < 500:
		return call(catalog.CallUnauthorized, "the provider refused the credentials")
	case h.status >= 500:
		return call(catalog.CallUnavailable, "the provider is unavailable")
	case h.status >= 400:
		return call(catalog.CallRejected, "the provider rejected the message")
	case h.send:
		// A success status with an unusable answer: the message may have been accepted.
		return call(catalog.CallOutcomeUnknown, "the provider's answer could not be read; the SMS may have been sent")
	}
	return call(catalog.CallUnavailable, "the provider's answer could not be read; nothing was sent")
}
