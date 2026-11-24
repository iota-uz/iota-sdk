package mcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"syscall"
	"time"

	"code.dny.dev/ssrf"
)

// ErrBlocked is an address the network policy refuses: not https, a port that is not allowed, or a
// private, loopback, link-local or metadata address (checked at every dial, so on every redirect hop).
var ErrBlocked = errors.New("mcp: the address is not allowed")

const maxRedirects = 10

// network is the guarded HTTP of the connector: the policy of pkg/automation/code's fetch (#1329).
type network struct {
	ports    []uint16
	guard    func(network, address string) error
	resolve  func(ctx context.Context, host string) ([]netip.Addr, error)
	tls      *tls.Config
	dialer   *net.Dialer
	base     *http.Transport
	redirect func(req *http.Request, via []*http.Request) error
}

func newNetwork() *network {
	return &network{ports: []uint16{443}}
}

// build makes the transport once the options are applied.
func (n *network) build() {
	if n.guard == nil {
		g := ssrf.New(ssrf.WithPorts(n.ports...))
		n.guard = func(network, address string) error { return g.Safe(network, address, nil) }
	}
	if n.resolve == nil {
		n.resolve = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	n.dialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if err := n.guard(network, address); err != nil {
				return ErrBlocked
			}
			return nil
		},
	}
	n.base = &http.Transport{
		Proxy:                 nil,
		DialContext:           n.dial,
		TLSClientConfig:       n.tls,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 0, // a tool may take long: the call's deadline bounds it
		ExpectContinueTimeout: time.Second,
	}
	n.redirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("%w: too many redirects", ErrBlocked)
		}
		if err := n.checkURL(req.URL); err != nil {
			return err
		}
		// Credentials never leave the origin they were given for.
		if first := via[0].URL; first.Scheme != req.URL.Scheme || first.Host != req.URL.Host {
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
		}
		return nil
	}
}

// checkURL is the scheme and port rule of a request or a redirect hop; the address is checked at dial.
func (n *network) checkURL(u *url.URL) error {
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%w: not https", ErrBlocked)
	}
	port := uint16(443)
	if p := u.Port(); p != "" {
		v, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return fmt.Errorf("%w: port", ErrBlocked)
		}
		port = uint16(v)
	}
	if !slices.Contains(n.ports, port) {
		return fmt.Errorf("%w: port %d", ErrBlocked, port)
	}
	return nil
}

// dial resolves the host, refuses it when any answer is not public and dials the answers; the dialer's
// Control checks the address once more.
func (n *network) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{ip}
	} else {
		if addrs, err = n.resolve(ctx, host); err != nil {
			return nil, err
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("mcp: %s has no address", host)
	}
	for _, a := range addrs {
		if err := n.guard(network, net.JoinHostPort(a.Unmap().String(), port)); err != nil {
			return nil, ErrBlocked
		}
	}
	var last error
	for _, a := range addrs {
		conn, err := n.dialer.DialContext(ctx, network, net.JoinHostPort(a.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// client is an HTTP client over the guarded transport; blocked (may be nil) hears of every request or
// redirect the policy refused.
func (n *network) client(wrap func(http.RoundTripper) http.RoundTripper, blocked func()) *http.Client {
	var rt http.RoundTripper = guardedRT{n: n}
	if wrap != nil {
		rt = wrap(rt)
	}
	return &http.Client{Transport: rt, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		err := n.redirect(req, via)
		if err != nil && blocked != nil {
			blocked()
		}
		return err
	}}
}

// guardedRT checks the URL of every request (the first one and each redirect) before the transport.
type guardedRT struct{ n *network }

func (g guardedRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := g.n.checkURL(req.URL); err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	return g.n.base.RoundTrip(req)
}
