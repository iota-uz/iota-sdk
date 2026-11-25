package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSameCursor(t *testing.T) {
	t.Parallel()
	raw := func(s string) json.RawMessage {
		if s == "" {
			return nil
		}
		return json.RawMessage(s)
	}
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"both none", "", "", true},
		{"none and null", "", "null", true},
		{"whitespace", `{"uid": 4, "mailbox": "INBOX"}`, `{"uid":4,"mailbox":"INBOX"}`, true},
		{"a seeded cursor differs", `{"uid":4}`, "", false},
		{"another position", `{"uid":4}`, `{"uid":5}`, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, sameCursor(raw(c.a), raw(c.b)), c.name)
	}
}
