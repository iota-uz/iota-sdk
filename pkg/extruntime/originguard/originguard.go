// Package originguard rejects cross-site state-changing requests to the
// extension runtime's cookie-authenticated surfaces: the browser RPC
// (/_ext/rpc), the management API (/_ext/manage) used by the configurator and
// the endpoints that run as the signed-in user.
//
// It is the SDK's own CSRF mechanism, filippo.io/csrf (what
// middleware.CSRF wraps, and net/http.CrossOriginProtection in Go 1.25): the
// browser's Sec-Fetch-Site header, or, for browsers without it, the Origin
// compared with the Host. A request with neither header is a same-origin
// request or does not come from a browser, and is allowed. Safe methods (GET,
// HEAD, OPTIONS) are never checked: they must not change state. No token is
// involved, so no page has a secret to leak.
package originguard

import (
	"errors"
	"fmt"
	"net/http"

	"filippo.io/csrf"
)

// ErrCrossSite is matched by every refusal.
var ErrCrossSite = errors.New("extruntime: cross-site request refused")

// Code is the error code of a refusal in JSON error bodies.
const Code = "runtime.crossSite"

// Guard checks requests; the zero value and nil are valid and trust no origin
// besides the request's own.
type Guard struct {
	p *csrf.Protection
}

// New returns a guard that also accepts requests from the trusted origins
// (scheme://host[:port]), for deployments whose UI is served from another
// origin than the API.
func New(trusted ...string) (*Guard, error) {
	p := csrf.New()
	for _, o := range trusted {
		if err := p.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("originguard: trusted origin %q: %w", o, err)
		}
	}
	return &Guard{p: p}, nil
}

var same = &Guard{p: csrf.New()}

// Check returns an error matching ErrCrossSite for a state-changing request
// made from another site.
func (g *Guard) Check(r *http.Request) error {
	if g == nil || g.p == nil {
		g = same
	}
	if err := g.p.Check(r); err != nil {
		return fmt.Errorf("%w: %w", ErrCrossSite, err)
	}
	return nil
}
