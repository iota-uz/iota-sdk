package services

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Layers of encoding, applied in any order on top of a composite of a login and a secret and embedded
// at any position, are unwrapped at the output boundary.
func TestCarriesSecretUnwrapsLayeredEncodings(t *testing.T) {
	t.Parallel()
	all := func(string) bool { return true }
	layers := map[string]func(string) string{
		"b64":    func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) },
		"rawurl": func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) },
		"hex":    func(s string) string { return hex.EncodeToString([]byte(s)) },
		"url":    url.QueryEscape,
	}
	names := []string{"b64", "rawurl", "hex", "url"}
	for _, pw := range []string{"p", "p1x", "Pa55", "s3cret!", "a&b c"} {
		creds := connectors.Credentials{"password": pw}
		base := "user:" + pw
		var chains [][]string
		for _, a := range names {
			chains = append(chains, []string{a})
			for _, b := range names {
				chains = append(chains, []string{a, b})
				for _, c := range names {
					chains = append(chains, []string{a, b, c})
				}
			}
		}
		for _, chain := range chains {
			s := base
			for _, l := range chain {
				s = layers[l](s)
			}
			for _, prefix := range []string{"", "x", "xy", "xyz", "bot "} {
				assert.True(t, carriesSecret(prefix+s, all, creds), "%v %q", chain, prefix+s)
			}
		}
	}
}

func TestValidIdentityKeepsOnlyTypedShapes(t *testing.T) {
	t.Parallel()
	all := func(string) bool { return true }
	creds := connectors.Credentials{"token": "123456:AAH-very-secret-bot-token", "password": "Zq7#long-pass"}
	keep := []connectors.Identity{
		{Display: "@acme_sales_bot", Kind: connectors.IdentityHandle},
		{Display: "robot@acme.uz", Kind: connectors.IdentityEmail},
		{Display: "+998 90 123 45 67", Kind: connectors.IdentityPhone},
	}
	for _, id := range keep {
		assert.Equal(t, id.Display, validIdentity(id, all, creds).Display, id.Display)
	}
	drop := []connectors.Identity{
		{Display: "Acme Customer Support Bot"},
		{Display: "Acme Customer Support Bot", Kind: connectors.IdentityHandle},
		{Display: "@a", Kind: connectors.IdentityHandle},
		{Display: "not-an-email", Kind: connectors.IdentityEmail},
		{Display: "+99890abc", Kind: connectors.IdentityPhone},
		{Display: "@acme_sales_bot", Kind: "custom"},
	}
	for _, id := range drop {
		got := validIdentity(id, all, creds)
		assert.Empty(t, got.Display, id.Display)
		assert.Empty(t, got.Kind, id.Display)
	}
}
