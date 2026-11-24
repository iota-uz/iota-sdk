package mcp

import (
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// minSecret is the shortest value replaced inside text: a shorter one (a bearer token may be any
// non-empty string) could be any word, so metadata that carries it is dropped instead of edited, and
// output that carries it is replaced as a whole.
const minSecret = 6

// scrubber removes the connection's secrets from what the server sends back: the server holds the bearer
// token and may echo it (or anything else it was given) in a tool's title, schema, text or error. The
// engine masks the credentials it opened; a token refreshed during the call is new to it, so the
// connector scrubs with the token it actually sent too.
type scrubber struct {
	secrets []string // long enough to replace inside text
	short   []string // non-empty, shorter than minSecret
}

func newScrubber(creds connectors.Credentials, sent ...string) scrubber {
	var s scrubber
	add := func(v string) {
		if v == "" {
			return
		}
		forms := []string{v}
		if q, err := json.Marshal(v); err == nil {
			if e := string(q[1 : len(q)-1]); e != v {
				forms = append(forms, e) // its form inside a JSON string
			}
		}
		for _, f := range forms {
			if len(f) < minSecret {
				s.short = append(s.short, f)
			} else {
				s.secrets = append(s.secrets, f)
			}
		}
	}
	for _, k := range []string{KeyToken, KeyClientSecret, KeyAccessToken, KeyRefreshToken} {
		add(creds[k])
	}
	for _, v := range sent {
		add(v)
	}
	return s
}

// carriesShort reports whether t holds a short secret.
func (s scrubber) carriesShort(t string) bool {
	for _, v := range s.short {
		if strings.Contains(t, v) {
			return true
		}
	}
	return false
}

// text replaces long secrets; text that holds a short one is withheld as a whole.
func (s scrubber) text(t string) string {
	if s.carriesShort(t) {
		return "***"
	}
	for _, v := range s.secrets {
		t = strings.ReplaceAll(t, v, "***")
	}
	return t
}

// has reports whether t carries a secret.
func (s scrubber) has(t string) bool { return s.text(t) != t }

// raw scrubs JSON; a document that holds a short secret, or is no longer JSON after the replacement,
// becomes an empty object.
func (s scrubber) raw(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return r
	}
	if s.carriesShort(string(r)) {
		return json.RawMessage(`{}`)
	}
	out := json.RawMessage(s.text(string(r)))
	if !json.Valid(out) {
		return json.RawMessage(`{}`)
	}
	return out
}

// tools scrubs a tools/list: a tool whose name or schemas carry a secret (a pin of an edited schema would
// not be the server's), or whose title or description carry a short one, is left out; a long secret
// in a title or a description is replaced.
func (s scrubber) tools(list []Tool) []Tool {
	out := make([]Tool, 0, len(list))
	for _, t := range list {
		if s.has(t.Name) || s.has(string(t.InputSchema)) || s.has(string(t.OutputSchema)) || s.carriesShort(t.Title+"\x00"+t.Description) {
			continue
		}
		t.Title, t.Description = s.text(t.Title), s.text(t.Description)
		out = append(out, t)
	}
	return out
}
