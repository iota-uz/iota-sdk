package services

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

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

// Limits of an identity part a connector reports.
const (
	maxIdentityDisplay  = 100
	maxIdentityExternal = 128
)

// tokenLike is a run an identity must not contain: long, from a token/encoding alphabet and mixing
// letters with digits or ending in base64 padding — how a credential or an encoded one looks, unlike a
// name, an @handle, an email or a phone number.
var tokenRun = regexp.MustCompile(`[A-Za-z0-9+/=_\-]{16,}`)

func tokenLike(s string) bool {
	for _, run := range tokenRun.FindAllString(s, -1) {
		if strings.HasSuffix(run, "=") || (strings.ContainsAny(run, "0123456789") && strings.IndexFunc(run, unicode.IsLetter) >= 0) {
			return true
		}
	}
	return false
}

// forms are the ways a value may be echoed: verbatim, URL-escaped, base64 (any alphabet, padded or
// not) and hex.
func forms(v string) []string {
	b := []byte(v)
	return []string{v, url.QueryEscape(v), url.PathEscape(v),
		base64.StdEncoding.EncodeToString(b), base64.RawStdEncoding.EncodeToString(b),
		base64.URLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(b),
		hex.EncodeToString(b)}
}

// identityPartOK validates one part of an identity on its own: printable, bounded, nothing token-like,
// and no form of any secret credential value (non-secret values, such as a login, may appear: an
// email identity is the login).
func identityPartOK(s string, limit int, secret secretSet, creds ...connectors.Credentials) bool {
	if s == "" {
		return true
	}
	if utf8.RuneCountInString(s) > limit || tokenLike(s) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	lower := strings.ToLower(s)
	for _, c := range creds {
		for field, v := range c {
			if v == "" || !secret(field) {
				continue
			}
			for _, f := range forms(v) {
				if strings.Contains(lower, strings.ToLower(f)) {
					return false
				}
			}
		}
	}
	return true
}

// validIdentity is the identity a connector reported, each part kept only when it passes
// identityPartOK; a rejected part is emptied (nothing of it is stored or shown).
func validIdentity(id connectors.Identity, secret secretSet, creds ...connectors.Credentials) connectors.Identity {
	id.Display = strings.TrimSpace(id.Display)
	id.External = strings.TrimSpace(id.External)
	if !identityPartOK(id.Display, maxIdentityDisplay, secret, creds...) {
		id.Display = ""
	}
	if !identityPartOK(id.External, maxIdentityExternal, secret, creds...) {
		id.External = ""
	}
	return id
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
