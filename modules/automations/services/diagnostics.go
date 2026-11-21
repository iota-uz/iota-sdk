package services

import (
	"bytes"
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

// Shapes of the public identity kinds a connector may declare. An identity of another kind, or of a
// kind whose shape it does not match, is not shown.
var identityShapes = map[connectors.IdentityKind]*regexp.Regexp{
	connectors.IdentityHandle: regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_]{1,63}$`),
	connectors.IdentityEmail:  regexp.MustCompile(`^[A-Za-z0-9._%+\-]{1,64}@[A-Za-z0-9\-]+(\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,24}$`),
	connectors.IdentityPhone:  regexp.MustCompile(`^\+?[0-9][0-9 ()\-]{4,23}$`),
}

// maxIdentityExternal bounds the provider identifier kept for matching (it is never shown).
const maxIdentityExternal = 128

// encodedRun is a run that could be base64 text; hexRun one that could be hex.
var (
	encodedRun = regexp.MustCompile(`[A-Za-z0-9+/_\-]{2,}`)
	hexRun     = regexp.MustCompile(`[0-9A-Fa-f]{2,}`)
)

// decodeOnce is every string s may stand for after one transformation: URL-unescaped, and every
// base64 (any alphabet) and hex decoding of each run, taken at each alignment, so an encoding embedded
// at any position decodes at one of them.
func decodeOnce(s string) []string {
	var out []string
	if u, err := url.QueryUnescape(s); err == nil && u != s {
		out = append(out, u)
	}
	if u, err := url.PathUnescape(s); err == nil && u != s {
		out = append(out, u)
	}
	for _, run := range encodedRun.FindAllString(s, -1) {
		for k := 0; k < 4 && k < len(run); k++ {
			std := strings.NewReplacer("-", "+", "_", "/").Replace(run[k:])
			if n := len(std) / 4 * 4; n > 0 {
				if b, err := base64.StdEncoding.DecodeString(std[:n]); err == nil {
					out = append(out, string(b))
				}
			}
			if len(std)%4 >= 2 {
				if b, err := base64.RawStdEncoding.DecodeString(std); err == nil {
					out = append(out, string(b))
				}
			}
		}
	}
	for _, run := range hexRun.FindAllString(s, -1) {
		for k := 0; k < 2 && k < len(run); k++ {
			part := run[k:]
			if n := len(part) / 2 * 2; n > 0 {
				if b, err := hex.DecodeString(part[:n]); err == nil {
					out = append(out, string(b))
				}
			}
		}
	}
	return out
}

// Layers of encoding the boundary unwraps, and the most candidates it examines; a text that needs more
// is refused rather than shown.
const (
	maxDecodeDepth = 4
	maxCandidates  = 4096
)

// carriesSecret reports whether s — as it is or after up to maxDecodeDepth layers of URL, base64 and
// hex encodings in any order — contains a secret credential value. It also reports true when s cannot
// be examined within maxCandidates: an output that cannot be shown to be safe is not shown.
func carriesSecret(s string, secret secretSet, creds ...connectors.Credentials) bool {
	var values [][]byte
	for _, c := range creds {
		for field, v := range c {
			if v != "" && secret(field) {
				values = append(values, bytes.ToLower([]byte(v)))
			}
		}
	}
	if len(values) == 0 || s == "" {
		return false
	}
	seen := map[string]bool{s: true}
	level := []string{s}
	for depth := 0; depth <= maxDecodeDepth && len(level) > 0; depth++ {
		var next []string
		for _, cand := range level {
			lower := bytes.ToLower([]byte(cand))
			for _, v := range values {
				if bytes.Contains(lower, v) {
					return true
				}
			}
			if depth == maxDecodeDepth {
				continue
			}
			for _, d := range decodeOnce(cand) {
				if seen[d] || len(d) < 1 {
					continue
				}
				seen[d] = true
				if len(seen) > maxCandidates {
					return true
				}
				next = append(next, d)
			}
		}
		level = next
	}
	return false
}

// validIdentity is the part of a connector's identity that may be stored and shown: Display only when
// the connector declared its kind, it has that kind's shape and carries no secret; External (kept for
// matching, never shown) only when it is bounded, printable and carries no secret.
func validIdentity(id connectors.Identity, secret secretSet, creds ...connectors.Credentials) connectors.Identity {
	id.Display = strings.TrimSpace(id.Display)
	id.External = strings.TrimSpace(id.External)
	shape, typed := identityShapes[id.Kind]
	if !typed || !shape.MatchString(id.Display) || carriesSecret(id.Display, secret, creds...) {
		id.Display, id.Kind = "", ""
	}
	if utf8.RuneCountInString(id.External) > maxIdentityExternal || strings.IndexFunc(id.External, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 ||
		carriesSecret(id.External, secret, creds...) {
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
