package code

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.dny.dev/ssrf"
)

// Fetch defaults.
const (
	DefaultMaxFetchBody    = 10 << 20
	DefaultMaxRedirects    = 20
	defaultFetchTimeout    = 60 * time.Second
	defaultTLSHandshake    = 10 * time.Second
	defaultResponseHeaders = 30 * time.Second
)

// ErrResponseTooLarge is a response whose body exceeds the fetcher's limit after decoding.
var ErrResponseTooLarge = errors.New("response body is too large")

// Resolver resolves host names. *net.Resolver implements it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// DialFunc opens a connection to an address that passed the network policy.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// FetchOption configures NewGuardedFetcher.
type FetchOption func(*guardedFetcher)

// WithMaxBody caps a response body after decoding, in bytes (default 10 MiB).
func WithMaxBody(n int64) FetchOption {
	return func(f *guardedFetcher) {
		if n > 0 {
			f.maxBody = n
		}
	}
}

// WithMaxRedirects caps the redirects one fetch follows (default 20).
func WithMaxRedirects(n int) FetchOption {
	return func(f *guardedFetcher) {
		if n >= 0 {
			f.maxRedirects = n
		}
	}
}

// WithAllowedPorts sets the ports a URL may name (default 443 only).
func WithAllowedPorts(ports ...uint16) FetchOption {
	return func(f *guardedFetcher) {
		if len(ports) > 0 {
			f.ports = append([]uint16(nil), ports...)
		}
	}
}

// WithResolver replaces the DNS resolver (default net.DefaultResolver).
func WithResolver(r Resolver) FetchOption {
	return func(f *guardedFetcher) {
		if r != nil {
			f.resolver = r
		}
	}
}

// WithDialer replaces the dial of an address that already passed the name, DNS and address checks; the
// guarded default also checks the address at dial time. For tests.
func WithDialer(dial DialFunc) FetchOption {
	return func(f *guardedFetcher) { f.dial = dial }
}

// withTLSConfig sets the client TLS configuration (tests trust their own server with it).
func withTLSConfig(cfg *tls.Config) FetchOption {
	return func(f *guardedFetcher) { f.tlsConfig = cfg }
}

type guardedFetcher struct {
	maxBody      int64
	maxRedirects int
	ports        []uint16
	resolver     Resolver
	dial         DialFunc
	tlsConfig    *tls.Config
	guardian     *ssrf.Guardian
	transport    *http.Transport
}

// NewGuardedFetcher returns the Fetcher of the network policy: https only, port 443 unless allowed
// otherwise, no local or metadata host names, no private, loopback, link-local or shared addresses (a name
// with any private DNS answer is refused as a whole, and the address is checked again when dialing), at most
// 20 redirects each checked the same way, credentials and every non-safelisted header dropped once a
// redirect leaves the origin, no proxy,
// and the body capped after decoding.
func NewGuardedFetcher(opts ...FetchOption) Fetcher {
	f := &guardedFetcher{
		maxBody:      DefaultMaxFetchBody,
		maxRedirects: DefaultMaxRedirects,
		ports:        []uint16{443},
		resolver:     net.DefaultResolver,
	}
	for _, o := range opts {
		o(f)
	}
	f.guardian = ssrf.New(ssrf.WithPorts(f.ports...))
	if f.dial == nil {
		d := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			Control: func(network, address string, _ syscall.RawConn) error {
				if err := f.guardian.Safe(network, address, nil); err != nil {
					host, _, _ := net.SplitHostPort(address)
					return &NetworkBlockedError{Host: host, Address: address, Reason: "private_address"}
				}
				return nil
			},
		}
		f.dial = d.DialContext
	}
	f.transport = &http.Transport{
		Proxy:                 nil,
		DialContext:           f.dialContext,
		TLSClientConfig:       f.tlsConfig,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   defaultTLSHandshake,
		ResponseHeaderTimeout: defaultResponseHeaders,
		ExpectContinueTimeout: time.Second,
	}
	return f
}

// blockedName reports a host name refused before DNS.
func blockedName(host string) bool {
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "" {
		return true
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return false
	}
	switch h {
	case "localhost", "metadata", "metadata.google.internal":
		return true
	}
	for _, suffix := range []string{".localhost", ".internal", ".local"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	// A single label resolves through the search domains of the host's network.
	return !strings.Contains(h, ".")
}

// checkURL applies the scheme, port and name rules to one request or redirect hop.
func (f *guardedFetcher) checkURL(u *url.URL) error {
	host := u.Hostname()
	if u.Scheme != "https" {
		return &NetworkBlockedError{Host: host, Reason: "scheme"}
	}
	port := uint16(443)
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return &NetworkBlockedError{Host: host, Address: u.Host, Reason: "port"}
		}
		port = uint16(n)
	}
	if !slices.Contains(f.ports, port) {
		return &NetworkBlockedError{Host: host, Address: u.Host, Reason: "port"}
	}
	if blockedName(host) {
		return &NetworkBlockedError{Host: host, Reason: "name"}
	}
	return nil
}

// dialContext resolves the host, refuses it if any answer is not public and dials the public answers.
func (f *guardedFetcher) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if blockedName(host) {
		return nil, &NetworkBlockedError{Host: host, Reason: "name"}
	}
	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{ip}
	} else {
		addrs, err = f.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("no address for %s", host)
		}
	}
	for _, ip := range addrs {
		if err := f.guardian.SafeAddr(ip); err != nil {
			return nil, &NetworkBlockedError{Host: host, Address: ip.String(), Reason: "private_address"}
		}
	}
	var lastErr error
	for _, ip := range addrs {
		conn, err := f.dial(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		var blocked *NetworkBlockedError
		if errors.As(err, &blocked) {
			return nil, blocked
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func origin(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return u.Scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
}

// crossOriginHeaders are the request headers that follow a redirect to another origin.
var crossOriginHeaders = map[string]bool{
	"Accept": true, "Accept-Language": true, "Content-Language": true, "Content-Type": true,
	"User-Agent": true, "Cache-Control": true, "Pragma": true,
}

var errRedirectMode = errors.New(`redirect refused: the request was made with redirect: "error"`)

func (f *guardedFetcher) Fetch(ctx context.Context, req FetchRequest) (*FetchResponse, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.User != nil {
		return nil, errors.New("invalid URL: credentials in the URL are not allowed")
	}
	if err := f.checkURL(u); err != nil {
		return nil, err
	}
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = http.MethodGet
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultFetchTimeout)
		defer cancel()
	}
	var body io.Reader
	if len(req.Body) > 0 {
		body = strings.NewReader(string(req.Body))
	}
	hreq, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	for k, vs := range req.Header {
		for _, v := range vs {
			hreq.Header.Add(k, v)
		}
	}
	// The transport negotiates compression itself, so the body limit applies to the decoded body.
	hreq.Header.Del("Accept-Encoding")

	start := origin(u)
	sensitive := make(map[string]bool, len(req.Sensitive))
	for _, h := range req.Sensitive {
		sensitive[http.CanonicalHeaderKey(h)] = true
	}
	redirects := 0
	client := &http.Client{
		Transport: f.transport,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			switch req.Redirect {
			case "manual":
				return http.ErrUseLastResponse
			case "error":
				return errRedirectMode
			}
			if len(via) > f.maxRedirects {
				return &NetworkBlockedError{Host: next.URL.Hostname(), Reason: "redirect_limit"}
			}
			if err := f.checkURL(next.URL); err != nil {
				return err
			}
			// Once any hop leaves the origin the credentials stay dropped, even back on the first origin.
			crossed := origin(next.URL) != start
			for _, v := range via {
				crossed = crossed || origin(v.URL) != start
			}
			if crossed {
				if req.SameOrigin {
					return &OriginError{From: start, To: origin(next.URL)}
				}
				// Authorization, Cookie and Proxy-Authorization, and with them every header the code set that
				// could carry a credential (an API key header): only the safelisted ones go to another origin,
				// and never one the request names sensitive.
				for k := range next.Header {
					if !crossOriginHeaders[http.CanonicalHeaderKey(k)] || sensitive[http.CanonicalHeaderKey(k)] {
						next.Header.Del(k)
					}
				}
			}
			redirects++
			return nil
		},
	}
	resp, err := client.Do(hreq)
	if err != nil {
		var blocked *NetworkBlockedError
		if errors.As(err, &blocked) {
			return nil, blocked
		}
		var left *OriginError
		if errors.As(err, &left) {
			return nil, left
		}
		if errors.Is(err, errRedirectMode) {
			return nil, errRedirectMode
		}
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return nil, fmt.Errorf("fetch %s: %w", hreq.URL.Redacted(), uerr.Err)
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// Sized up front when the length is known, so reading does not leave a trail of grown buffers.
	var buf bytes.Buffer
	if n := resp.ContentLength; n > 0 && n <= f.maxBody {
		buf.Grow(int(n) + 1)
	}
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, f.maxBody+1))
	data := buf.Bytes()
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if int64(len(data)) > f.maxBody {
		return nil, fmt.Errorf("%w: it exceeds %d bytes", ErrResponseTooLarge, f.maxBody)
	}
	return &FetchResponse{
		Status:     resp.StatusCode,
		StatusText: strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode))),
		Header:     resp.Header.Clone(),
		Body:       data,
		URL:        resp.Request.URL.String(),
		Redirected: redirects > 0,
	}, nil
}
