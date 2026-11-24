package mcp

import (
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// minSecret is the shortest value scrubbed: shorter text would replace ordinary words.
const minSecret = 6

// scrubber removes the connection's secrets from what the server sends back: the server holds the bearer
// token and may echo it (or anything else it was given) in a tool's title, schema, text or error. The
// engine masks the credentials it opened; a token refreshed during the call is new to it, so the
// connector scrubs with the token it actually sent too.
type scrubber struct{ secrets []string }

func newScrubber(creds connectors.Credentials, sent ...string) scrubber {
	var s scrubber
	add := func(v string) {
		if len(v) < minSecret {
			return
		}
		s.secrets = append(s.secrets, v)
		if q, err := json.Marshal(v); err == nil {
			if e := string(q[1 : len(q)-1]); e != v {
				s.secrets = append(s.secrets, e) // its form inside a JSON string
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

func (s scrubber) text(t string) string {
	for _, v := range s.secrets {
		t = strings.ReplaceAll(t, v, "***")
	}
	return t
}

// has reports whether t carries a secret.
func (s scrubber) has(t string) bool { return s.text(t) != t }

// raw scrubs JSON; a document that is no longer JSON after it becomes an empty object.
func (s scrubber) raw(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return r
	}
	out := json.RawMessage(s.text(string(r)))
	if !json.Valid(out) {
		return json.RawMessage(`{}`)
	}
	return out
}

// tools scrubs a tools/list: a tool whose name carries a secret is left out.
func (s scrubber) tools(list []Tool) []Tool {
	out := make([]Tool, 0, len(list))
	for _, t := range list {
		if s.has(t.Name) {
			continue
		}
		t.Title, t.Description = s.text(t.Title), s.text(t.Description)
		t.InputSchema, t.OutputSchema = s.raw(t.InputSchema), s.raw(t.OutputSchema)
		out = append(out, t)
	}
	return out
}
