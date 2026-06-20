// Package middleware provides this package.
package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// HTMXCacheControl protects against an HTMX fragment response being cached and
// later served in place of a full-page navigation.
//
// Controllers answer an HTMX request with a layout-less, <head>-less fragment
// and a normal navigation with a full HTML document, but both responses share
// the same URL. Without a Vary header that distinguishes them, any shared cache
// (the browser HTTP cache, a reverse proxy, or a CDN) may serve the cached
// fragment to a back/forward or address-bar navigation, rendering the page with
// no stylesheet at all — completely unstyled raw HTML.
//
// For text/html responses the middleware:
//   - adds "Vary: Hx-Request" so a cache key can never match an HTMX fragment to
//     a normal navigation; and
//   - sets "Cache-Control: no-store" (these pages are per-user / per-tenant and
//     must never be cached) unless a handler already set Cache-Control — for
//     example the static-asset handler, which keeps its own public caching.
//
// It also normalizes history-restore requests. On a local history-cache miss
// htmx issues a GET carrying "Hx-History-Restore-Request: true" together with
// "Hx-Request: true"; left untouched, controllers answer with a fragment that
// htmx swaps into <body>, discarding the surrounding layout shell. Removing the
// HTMX request markers makes controllers render the full page, which htmx
// restores correctly (it extracts <body> and drops the duplicate <head>).
func HTMXCacheControl() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// A history-restore is semantically a full navigation: render the
			// whole page so the layout shell survives a history-cache miss.
			if r.Header.Get("Hx-History-Restore-Request") == "true" {
				r.Header.Del("Hx-Request")
				r.Header.Del("Hx-Boosted")
			}
			next.ServeHTTP(&htmxCacheControlWriter{ResponseWriter: w}, r)
		})
	}
}

type htmxCacheControlWriter struct {
	http.ResponseWriter
	decorated bool
}

// decorate sets cache-safety headers for HTML responses. It runs once, when the
// response Content-Type becomes known. When the handler left Content-Type unset,
// net/http sniffs it from the first body bytes; we mirror that with
// http.DetectContentType(body) so sniffed HTML responses still get protected.
// With no Content-Type and no body yet (a bare WriteHeader), it stays undecided
// and re-runs on the following Write.
func (w *htmxCacheControlWriter) decorate(body []byte) {
	if w.decorated {
		return
	}
	h := w.Header()
	ct := h.Get("Content-Type")
	if ct == "" {
		if len(body) == 0 {
			return
		}
		ct = http.DetectContentType(body)
	}
	w.decorated = true
	if !strings.HasPrefix(ct, "text/html") {
		return
	}
	h.Add("Vary", "Hx-Request")
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
}

func (w *htmxCacheControlWriter) WriteHeader(code int) {
	// Don't latch on 1xx informational responses.
	if code >= 200 {
		w.decorate(nil)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *htmxCacheControlWriter) Write(b []byte) (int, error) {
	w.decorate(b)
	return w.ResponseWriter.Write(b)
}

// Flush forwards to the underlying writer so streamed responses
// (templ WithStreaming) and SSE keep working.
func (w *htmxCacheControlWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying writer so WebSocket / SSE upgrades keep working.
func (w *htmxCacheControlWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("htmxCacheControlWriter: underlying ResponseWriter does not implement http.Hijacker")
}
