package connectors

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The metadata of a connection (its display name, its identity) must never carry a value of its
// credentials: every writer that keeps metadata across a credential change (an update, a
// re-authorization, an automatic OAuth2 refresh) checks it against the old and the new credentials
// with CarriesSecret and SafeIdentity, and readers check it against the current ones.

// Shapes of the public identity kinds a connector may declare. An identity of another kind, or of a
// kind whose shape it does not match, is not shown.
var identityShapes = map[IdentityKind]*regexp.Regexp{
	IdentityHandle: regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_]{1,63}$`),
	IdentityEmail:  regexp.MustCompile(`^[A-Za-z0-9._%+\-]{1,64}@[A-Za-z0-9\-]+(\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,24}$`),
	IdentityPhone:  regexp.MustCompile(`^\+?[0-9][0-9 ()\-]{4,23}$`),
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

// CarriesSecret reports whether s — as it is or after up to maxDecodeDepth layers of URL, base64 and
// hex encodings in any order — contains a secret credential value. It also reports true when s cannot
// be examined within maxCandidates, or when a text still decodes past maxDecodeDepth into one long
// enough to hold a value: an output that cannot be shown to be safe is not shown.
func CarriesSecret(s string, secret func(field string) bool, creds ...Credentials) bool {
	var values [][]byte
	shortest := 0
	for _, c := range creds {
		for field, v := range c {
			if v != "" && secret(field) {
				values = append(values, bytes.ToLower([]byte(v)))
				if shortest == 0 || len(v) < shortest {
					shortest = len(v)
				}
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
				// One more layer that could still hold a value is beyond what is examined: refuse.
				for _, d := range decodeOnce(cand) {
					if len(d) >= shortest && !seen[d] {
						return true
					}
				}
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

// SafeIdentity is the part of a connector's identity that may be stored and shown: Display only when
// the connector declared its kind, it has that kind's shape and carries no secret; External (kept for
// matching, never shown) only when it is bounded, printable and carries no secret.
func SafeIdentity(id Identity, secret func(field string) bool, creds ...Credentials) Identity {
	id.Display = strings.TrimSpace(id.Display)
	id.External = strings.TrimSpace(id.External)
	shape, typed := identityShapes[id.Kind]
	if !typed || !shape.MatchString(id.Display) || CarriesSecret(id.Display, secret, creds...) {
		id.Display, id.Kind = "", ""
	}
	if utf8.RuneCountInString(id.External) > maxIdentityExternal || strings.IndexFunc(id.External, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 ||
		CarriesSecret(id.External, secret, creds...) {
		id.External = ""
	}
	return id
}
