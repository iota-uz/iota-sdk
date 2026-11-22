package services

import (
	"regexp"

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

// A provider's text is never returned: only its Reason (classify). Masking a credential inside free
// text cannot be made complete — a provider may echo it encoded together with other data (a Basic
// header, a signed payload) — so the page explains the category and nothing of the text reaches a
// client, a log of the page or the stored status.

// carriesSecret and validIdentity are the connectors package's checks (shared with the store's own
// writers, such as the automatic OAuth2 refresh).
func carriesSecret(s string, secret secretSet, creds ...connectors.Credentials) bool {
	return connectors.CarriesSecret(s, secret, creds...)
}

func validIdentity(id connectors.Identity, secret secretSet, creds ...connectors.Credentials) connectors.Identity {
	return connectors.SafeIdentity(id, secret, creds...)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
