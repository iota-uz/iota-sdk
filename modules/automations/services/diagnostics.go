package services

import (
	"encoding/base64"
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

// sanitize returns s with every credential value masked, verbatim and in the forms a provider echoes
// (URL-escaped, base64 as in an Authorization header). A value shorter than four characters cannot be
// masked inside text without mangling it: when one appears as a word, nothing of s is returned.
func sanitize(s string, creds ...connectors.Credentials) (string, bool) {
	var long []string
	for _, c := range creds {
		for _, v := range c {
			switch {
			case v == "":
			case len(v) < 4:
				if regexp.MustCompile(`(^|[^\pL\pN])` + regexp.QuoteMeta(v) + `($|[^\pL\pN])`).MatchString(s) {
					return "", false
				}
			default:
				long = append(long, v, url.QueryEscape(v), url.PathEscape(v),
					base64.StdEncoding.EncodeToString([]byte(v)), base64.RawStdEncoding.EncodeToString([]byte(v)),
					base64.URLEncoding.EncodeToString([]byte(v)), base64.RawURLEncoding.EncodeToString([]byte(v)))
			}
		}
	}
	// Longer values first: one may contain another.
	sort.Slice(long, func(i, j int) bool { return len(long[i]) > len(long[j]) })
	for _, v := range long {
		s = strings.ReplaceAll(s, v, "•••")
	}
	return s, true
}

// detail is the provider's message prepared for the disclosure: sanitized and bounded; empty when it
// cannot be shown safely.
func detail(msg string, creds ...connectors.Credentials) string {
	s, ok := sanitize(msg, creds...)
	if !ok {
		return ""
	}
	return truncate(strings.TrimSpace(s), maxDetail)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
