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

// A status code decides only where a status stands; other numbers in the text are not statuses.
func TestClassifyReadsStatusInItsPosition(t *testing.T) {
	cases := map[string]Reason{
		"HTTP 503 Service Unavailable (request reference 401)": ReasonNetwork,
		"401 Unauthorized: token is not accepted":              ReasonAuth,
		"535 5.7.8 Authentication credentials invalid":         ReasonAuth,
		"telegram: status code: 429, retry later":              ReasonRateLimit,
		"HTTP/1.1 404 chat 401 not found":                      ReasonConfig,
		"request 401 failed: dial tcp: i/o timeout":            ReasonNetwork,
		"order 503 was rejected":                               ReasonUnknown,
		"invalid api key":                                      ReasonAuth,
	}
	for msg, want := range cases {
		assert.Equal(t, want, classify(msg), msg)
	}
}

// Without x-secret fields every field is secret: a blank one keeps its stored value on an edit.
func TestMergeKeepsBlankSecretsOfASchemalessConnector(t *testing.T) {
	stored := connectors.Credentials{"token": "t0k3n", "chat": "-100"}
	got := mergeCredentials(connectors.Descriptor{}, stored, connectors.Credentials{"token": "", "chat": "-200"})
	assert.Equal(t, connectors.Credentials{"token": "t0k3n", "chat": "-200"}, got)
	d := connectors.Descriptor{Credentials: []byte(`{"type":"object","properties":{"token":{"type":"string","x-secret":true},"chat":{"type":"string"}}}`)}
	got = mergeCredentials(d, stored, connectors.Credentials{"token": "", "chat": ""})
	assert.Equal(t, connectors.Credentials{"token": "t0k3n"}, got, "a public field is removed when blank")
}

// An OAuth2 token set is secret as a whole: a schema that marks only refresh_token does not make the
// access token showable.
func TestOAuth2TokenFieldsAreAllSecret(t *testing.T) {
	d := connectors.Descriptor{Auth: connectors.AuthOAuth2, Credentials: []byte(`{"type":"object","properties":{
		"access_token":{"type":"string"},"refresh_token":{"type":"string","x-secret":true}}}`)}
	secret := secretsOf(d)
	assert.True(t, secret("access_token"))
	creds := connectors.Credentials{"access_token": "abcdef"}
	got := validIdentity(connectors.Identity{Display: "@abcdef", Kind: connectors.IdentityHandle}, secret, creds)
	assert.Empty(t, got.Display)
	plain := connectors.Descriptor{Auth: connectors.AuthAPIKey, Credentials: d.Credentials}
	assert.False(t, secretsOf(plain)("access_token"), "other connectors keep the schema's selection")
}
