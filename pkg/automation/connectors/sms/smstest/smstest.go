// Package smstest is fakes of the HTTP APIs of Eskiz and Twilio for tests of the SMS connectors: they
// check credentials, record what they were asked to send and answer as the test scripts them (a status,
// a Retry-After, no answer at all).
package smstest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
)

// Reply is how a fake answers a send.
type Reply struct {
	// Status is the HTTP status; 0 is the fake's own success answer.
	Status int
	// Header is added to the response (Retry-After).
	Header map[string]string
	// Body replaces the body of the answer.
	Body string
	// Hang accepts the request and never answers: the connection stays open until the fake is closed.
	Hang bool
}

// Sent is a message a fake received.
type Sent struct {
	To, From, Text string
	// Service is the Messaging Service SID of a Twilio message.
	Service string
	At      int // 1 for the first send the fake saw
}

type base struct {
	srv   *httptest.Server
	mu    sync.Mutex
	sends []Sent
	reply func(n int) Reply
	done  chan struct{}
	once  sync.Once
}

func (b *base) init(h http.Handler) {
	b.done = make(chan struct{})
	b.srv = httptest.NewServer(h)
}

// URL is the base address of the fake.
func (b *base) URL() string { return b.srv.URL }

// Close stops the fake and releases the requests that hang.
func (b *base) Close() {
	b.once.Do(func() { close(b.done) })
	b.srv.CloseClientConnections()
	b.srv.Close()
}

// Script sets how the fake answers its n-th send (1 for the first); nil: always success.
func (b *base) Script(f func(n int) Reply) {
	b.mu.Lock()
	b.reply = f
	b.mu.Unlock()
}

// Sends are the messages received, hung or refused ones included once they were parsed.
func (b *base) Sends() []Sent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Sent(nil), b.sends...)
}

func (b *base) record(s Sent) (Sent, Reply) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s.At = len(b.sends) + 1
	b.sends = append(b.sends, s)
	var r Reply
	if b.reply != nil {
		r = b.reply(s.At)
	}
	return s, r
}

func (b *base) write(w http.ResponseWriter, req *http.Request, r Reply, ok int, okBody string) {
	if r.Hang {
		select {
		case <-b.done:
		case <-req.Context().Done():
		}
		return
	}
	for k, v := range r.Header {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	status, body := ok, okBody
	if r.Status != 0 {
		status = r.Status
		body = r.Body
		if body == "" {
			body = fmt.Sprintf(`{"message":"scripted %d"}`, status)
		}
	} else if r.Body != "" {
		body = r.Body
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// redirect sends every request to the server of a fake, whatever host it names.
type redirect struct {
	target *url.URL
	next   http.RoundTripper
}

func (t redirect) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme, r2.URL.Host = t.target.Scheme, t.target.Host
	r2.Host = t.target.Host
	return t.next.RoundTrip(r2)
}

func newRedirect(raw string) http.RoundTripper {
	u, _ := url.Parse(raw)
	return redirect{target: u, next: http.DefaultTransport}
}

func bearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
