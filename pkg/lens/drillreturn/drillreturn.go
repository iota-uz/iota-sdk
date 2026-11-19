// Package drillreturn is the wire protocol that carries a Lens exploration
// into a host page (operational records) and back without losing it.
//
// When a reader leaves a drill level for a host page, the Lens runtime adds
// Param to the destination URL. Its value is an opaque, versioned token the
// host decodes with FromRequest: the site-relative URL of the exploration
// (levels, perspective and filters are all in that URL, so it works in a new
// tab without any memory of the previous one), the dashboard title and the
// labels of the levels the reader passed. The host renders a Back link with
// BackURL. The host's own state (sort, page, scroll of its records) travels
// back as an opaque HostState in HostParam; the runtime keeps it in its URL
// and returns the same state to the same leaf the next time the reader opens
// it from that exploration. Nothing here is stored server side.
//
// The TypeScript runtime implements the same encoding
// (web/lens/src/runtime/returnContext.ts); fixtures/return-context.json is the
// shared acceptance fixture of both sides.
package drillreturn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Param carries the encoded Context on the URL of a host page opened from Lens.
	Param = "lens-return"
	// HostParam carries the encoded HostState on the Lens URL the host returns to.
	HostParam = "lens-host"
	// Version is the only protocol version this package reads and writes.
	Version = 1
	// MaxEncodedBytes bounds an encoded token so it stays within URL limits.
	MaxEncodedBytes = 6 << 10
)

// ErrInvalid reports a token that is malformed, of another version, too
// large, or whose destination is not a site-relative URL.
var ErrInvalid = errors.New("drillreturn: invalid return context")

// Context describes the exploration a host page was opened from.
type Context struct {
	V int `json:"v"`
	// Href is the site-relative Lens URL that restores the exploration.
	Href string `json:"href"`
	// Title is the dashboard (or report) title the reader came from.
	Title string `json:"title"`
	// Trail lists the labels of the drill levels, outermost first.
	Trail []string `json:"trail,omitempty"`
	// Host is the opaque state the host handed back for this leaf earlier.
	Host string `json:"host,omitempty"`
}

// HostState is the host page's own opaque state, keyed by the leaf path it
// belongs to so it is returned only to that page.
type HostState struct {
	V     int    `json:"v"`
	Leaf  string `json:"leaf"`
	State string `json:"state"`
}

func marshal(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(bytes.TrimRight(buffer.Bytes(), "\n"))
	if len(encoded) > MaxEncodedBytes {
		return "", fmt.Errorf("%w: token exceeds %d bytes", ErrInvalid, MaxEncodedBytes)
	}
	return encoded, nil
}

func unmarshal(token string, value any) error {
	if token == "" || len(token) > MaxEncodedBytes {
		return ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return ErrInvalid
	}
	return nil
}

// SiteRelative reports whether href is a path on the same site: it starts
// with a single slash and carries no scheme or host, so a token can never
// redirect the reader elsewhere.
func SiteRelative(href string) bool {
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") || strings.ContainsAny(href, "\\\r\n\t") {
		return false
	}
	parsed, err := url.Parse(href)
	return err == nil && parsed.Scheme == "" && parsed.Host == ""
}

func (c Context) validate() error {
	if c.V != Version || !SiteRelative(c.Href) || strings.TrimSpace(c.Title) == "" {
		return ErrInvalid
	}
	return nil
}

// Encode returns the URL token for c.
func Encode(c Context) (string, error) {
	if c.V == 0 {
		c.V = Version
	}
	if err := c.validate(); err != nil {
		return "", err
	}
	return marshal(c)
}

// Decode parses and validates a token produced by Encode or the runtime.
func Decode(token string) (Context, error) {
	var c Context
	if err := unmarshal(token, &c); err != nil {
		return Context{}, err
	}
	if err := c.validate(); err != nil {
		return Context{}, err
	}
	return c, nil
}

// FromRequest reads the return context of a host page request. ok is false
// when the page was not opened from Lens or the token is invalid; the host
// then renders its page without a Back link.
func FromRequest(r *http.Request) (Context, bool) {
	c, err := Decode(r.URL.Query().Get(Param))
	return c, err == nil
}

// EncodeHostState returns the URL token for a host state.
func EncodeHostState(s HostState) (string, error) {
	if s.V == 0 {
		s.V = Version
	}
	if s.V != Version || !SiteRelative(s.Leaf) || s.State == "" {
		return "", ErrInvalid
	}
	return marshal(s)
}

// DecodeHostState parses a token produced by EncodeHostState.
func DecodeHostState(token string) (HostState, error) {
	var s HostState
	if err := unmarshal(token, &s); err != nil {
		return HostState{}, err
	}
	if s.V != Version || !SiteRelative(s.Leaf) || s.State == "" {
		return HostState{}, ErrInvalid
	}
	return s, nil
}

// BackURL is the link back to the exploration. When state is not empty it is
// attached for leaf (the host page's own path, without query), so the next
// visit of that page from the same exploration receives it as Context.Host.
func (c Context) BackURL(leaf, state string) (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}
	target, err := url.Parse(c.Href)
	if err != nil {
		return "", ErrInvalid
	}
	query := target.Query()
	query.Del(HostParam)
	if state != "" {
		token, encodeErr := EncodeHostState(HostState{Leaf: leaf, State: state})
		if encodeErr != nil {
			return "", encodeErr
		}
		query.Set(HostParam, token)
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}
