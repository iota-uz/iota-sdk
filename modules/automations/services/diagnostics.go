package services

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Reason is the category of a provider's refusal. The page explains each in the user's language with a
// step to fix it; the provider's own text is never the message.
type Reason string

const (
	ReasonAuth      Reason = "auth"       // the credentials are wrong, revoked or lack rights
	ReasonNetwork   Reason = "network"    // the service did not answer or is unavailable
	ReasonRateLimit Reason = "rate_limit" // the service throttles the account
	ReasonConfig    Reason = "config"     // a setting (an address, an ID) is wrong
	ReasonReauth    Reason = "reauth"     // an OAuth2 sign-in is required again
	ReasonUnknown   Reason = "unknown"
)

// Reasons are every Reason, for a client that maps them to phrases.
var Reasons = []Reason{ReasonAuth, ReasonNetwork, ReasonRateLimit, ReasonConfig, ReasonReauth, ReasonUnknown}

// IsReason reports whether s is a known Reason (a stored status reason of this module).
func IsReason(s string) bool {
	for _, r := range Reasons {
		if string(r) == s {
			return true
		}
	}
	return false
}

var reasonPatterns = []struct {
	reason Reason
	re     *regexp.Regexp
}{
	{ReasonRateLimit, regexp.MustCompile(`(?i)\b429\b|rate.?limit|too many requests|quota`)},
	{ReasonAuth, regexp.MustCompile(`(?i)\b(401|403|535)\b|unauthori[sz]ed|forbidden|authenticat|invalid[ _-]?(api[ _-]?key|token|credentials|grant|client)|incorrect (password|login)|access denied|permission denied|revoked`)},
	{ReasonNetwork, regexp.MustCompile(`(?i)\b(50[234])\b|timeout|timed out|deadline exceeded|connection (refused|reset)|no such host|dial tcp|\beof\b|unreachable|tls|certificate|service unavailable`)},
	{ReasonConfig, regexp.MustCompile(`(?i)\b(400|404|422)\b|not found|bad request|invalid|unknown (chat|host|user)`)},
}

// classify is the Reason of a provider's message.
func classify(msg string) Reason {
	for _, p := range reasonPatterns {
		if p.re.MatchString(msg) {
			return p.reason
		}
	}
	return ReasonUnknown
}

// maxDetail bounds the provider's text kept for the «show the service's answer» disclosure.
const maxDetail = 500

// secretSet tells which credential fields are secret: the x-secret fields of the connector's schema;
// every field when the schema declares none (an OAuth2 token set).
type secretSet func(field string) bool

func secretsOf(d connectors.Descriptor) secretSet {
	fields := connectors.SecretFields(d)
	if len(fields) == 0 {
		return func(string) bool { return true }
	}
	return func(f string) bool { return fields[f] }
}

// minMaskable is the shortest secret value that is masked inside text. A shorter one cannot be
// recognised reliably once encoded (base64 shifts with its neighbours, a Basic header joins it with
// the login), so text that may carry it is not returned at all.
const minMaskable = 8

// opaque is a run that looks like a token or an encoding (base64, hex): a provider's text can carry a
// credential in a form no mask anticipates, so such runs are redacted from the detail too.
var opaque = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{20,}`)

// forms are the ways a provider may echo v: verbatim, URL-escaped, base64 (any alphabet, padded or
// not) and hex.
func forms(v string) []string {
	b := []byte(v)
	return []string{v, url.QueryEscape(v), url.PathEscape(v),
		base64.StdEncoding.EncodeToString(b), base64.RawStdEncoding.EncodeToString(b),
		base64.URLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(b),
		hex.EncodeToString(b)}
}

// masked replaces every form of every credential value of at least minMaskable characters. It reports
// false when a secret value is too short to mask and one of its forms occurs in s.
func masked(s string, secret secretSet, creds ...connectors.Credentials) (string, bool) {
	var long []string
	lower := strings.ToLower(s)
	for _, c := range creds {
		for field, v := range c {
			switch {
			case v == "":
			case len(v) < minMaskable:
				if !secret(field) {
					continue
				}
				for _, f := range forms(v) {
					if strings.Contains(lower, strings.ToLower(f)) {
						return "", false
					}
				}
			default:
				long = append(long, forms(v)...)
			}
		}
	}
	// Longer forms first: one may contain another.
	sort.Slice(long, func(i, j int) bool { return len(long[i]) > len(long[j]) })
	for _, v := range long {
		s = strings.ReplaceAll(s, v, "•••")
	}
	return s, true
}

// hasShortSecret reports whether a secret value is too short to be masked in text.
func hasShortSecret(secret secretSet, creds ...connectors.Credentials) bool {
	for _, c := range creds {
		for field, v := range c {
			if v != "" && len(v) < minMaskable && secret(field) {
				return true
			}
		}
	}
	return false
}

// sanitizeIdentity is the identity a connector returned, safe to store and show: every form of every
// credential value masked; a part that carries a form of a value too short to mask is emptied.
func sanitizeIdentity(id connectors.Identity, secret secretSet, creds ...connectors.Credentials) connectors.Identity {
	var ok bool
	if id.Display, ok = masked(id.Display, secret, creds...); !ok {
		id.Display = ""
	}
	if id.External, ok = masked(id.External, secret, creds...); !ok {
		id.External = ""
	}
	return id
}

// detail is the provider's message for the «show the service's answer» disclosure, returned only when
// it is safe: never when a secret value is too short to be masked (whatever form it may take in the
// text); otherwise with every form of every value masked, token-like runs redacted, bounded.
func detail(msg string, secret secretSet, creds ...connectors.Credentials) string {
	if hasShortSecret(secret, creds...) {
		return ""
	}
	s, ok := masked(msg, secret, creds...)
	if !ok {
		return ""
	}
	s = opaque.ReplaceAllString(s, "•••")
	return truncate(strings.TrimSpace(s), maxDetail)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
