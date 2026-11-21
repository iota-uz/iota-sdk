package services

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Whatever is combined with a secret and however it is encoded, embedded at any position of the
// identity, the identity is refused.
func TestIdentityRefusesEncodedSecretsAtAnyOffset(t *testing.T) {
	t.Parallel()
	all := func(string) bool { return true }
	encoders := map[string]func([]byte) string{
		"std": base64.StdEncoding.EncodeToString, "raw": base64.RawStdEncoding.EncodeToString,
		"url": base64.URLEncoding.EncodeToString, "rawurl": base64.RawURLEncoding.EncodeToString,
		"hex": hex.EncodeToString,
	}
	for _, pw := range []string{"p", "p1", "p1x", "Pa55", "s3cret!", "a&b c"} {
		creds := connectors.Credentials{"password": pw}
		for _, login := range []string{"", "u", "us", "usr", "user", "robot@acme.uz"} {
			for _, sep := range []string{":", "", " "} {
				payload := []byte(login + sep + pw)
				for name, enc := range encoders {
					for _, prefix := range []string{"", "a", "ab", "abc", "bot ", "bot_", "x-"} {
						id := prefix + enc(payload)
						assert.False(t, identityPartOK(id, maxIdentityDisplay, all, creds), "%s %q", name, id)
					}
				}
			}
		}
		assert.False(t, identityPartOK("bot "+url.QueryEscape(pw), maxIdentityDisplay, all, creds))
		assert.False(t, identityPartOK("bot "+pw, maxIdentityDisplay, all, creds))
	}
}

func TestIdentityKeepsOrdinaryIdentities(t *testing.T) {
	t.Parallel()
	all := func(string) bool { return true }
	creds := connectors.Credentials{"token": "123456:AAH-very-secret-bot-token", "password": "Zq7#long-pass"}
	for _, ok := range []string{"@acme_sales_bot", "robot@acme.uz", "Acme Customer Support Bot", "+998 90 123 45 67", "Ғайрат Қодиров", "O'Brien & Sons (Main)"} {
		assert.True(t, identityPartOK(ok, maxIdentityDisplay, all, creds), ok)
	}
	for _, bad := range []string{"bot=1", "a/b", "id:42", "x\u200by", strings.Repeat("a", maxIdentityDisplay+1), "tok eyJhbGciOiJIUzI1NiJ9abc123"} {
		assert.False(t, identityPartOK(bad, maxIdentityDisplay, all, creds), bad)
	}
}
