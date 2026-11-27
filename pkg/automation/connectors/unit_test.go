package connectors_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

func TestParseKeyring(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(key('a'))
	raw := base64.RawURLEncoding.EncodeToString(key('b'))
	k, err := connectors.ParseKeyring(" new:" + b64 + ", old:" + raw + ",hex:" + strings.Repeat("0f", 32))
	require.NoError(t, err)
	id, active := k.Active()
	assert.Equal(t, "new", id)
	assert.Equal(t, key('a'), active)
	old, ok := k.Key("old")
	require.True(t, ok)
	assert.Equal(t, key('b'), old)
	_, ok = k.Key("hex")
	assert.True(t, ok)

	k, err = connectors.ParseKeyringActive("new:"+b64+",old:"+raw, "old")
	require.NoError(t, err)
	id, _ = k.Active()
	assert.Equal(t, "old", id, "a new key rolled out decrypt-only")

	k, err = connectors.ParseKeyring("  ")
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
	assert.Nil(t, k)
	for _, bad := range []string{"nokey", "a:" + base64.StdEncoding.EncodeToString([]byte("short")), "a:" + b64 + ",a:" + b64, "bad id:" + b64} {
		_, err := connectors.ParseKeyring(bad)
		require.Error(t, err, bad)
		if err != nil {
			assert.NotContains(t, err.Error(), b64)
		}
	}
	_, err = connectors.NewKeyring("missing", map[string][]byte{"k": key('a')})
	require.Error(t, err)
}

func TestDirectoryChecksDescriptors(t *testing.T) {
	reg := catalog.NewRegistry(nil)
	dir := connectors.NewDirectory(reg)
	tg := telegramConnector(t)
	require.NoError(t, dir.Register(tg))
	_, ok := reg.Node("telegram.send")
	assert.True(t, ok, "the directory registers the contribution")
	got, ok := dir.Get(catalog.KindTelegramBot)
	require.True(t, ok)
	assert.Same(t, tg, got)

	smtp := telegramConnector(t)
	smtp.kind = catalog.KindSMTP
	smtp.nodes, smtp.sources = nil, nil
	require.NoError(t, dir.Register(smtp))
	list := dir.List()
	require.Len(t, list, 2)
	assert.Equal(t, catalog.KindSMTP, list[0].Kind())

	oauthless := telegramConnector(t)
	oauthless.kind, oauthless.nodes, oauthless.sources = "sheets", nil, nil
	oauthless.auth = connectors.AuthOAuth2
	require.ErrorIs(t, dir.Register(oauthless), connectors.ErrInvalid)
	badPolicy := telegramConnector(t)
	badPolicy.kind, badPolicy.nodes, badPolicy.sources = "x", nil, nil
	badPolicy.policy = "everyone"
	require.ErrorIs(t, dir.Register(badPolicy), connectors.ErrInvalid)

	d := tg.Descriptor()
	assert.Equal(t, map[string]bool{"token": true, "access_token": true}, connectors.SecretFields(d))
	assert.Equal(t, map[string]string{"username": "support"}, connectors.RedactedCredentials(d, connectors.Credentials{"token": botToken, "username": "support"}))
	assert.Equal(t, map[string]string{}, connectors.RedactedCredentials(d, connectors.Credentials{"undeclared": botToken}), "undeclared fields are withheld")
	assert.Empty(t, connectors.RedactedCredentials(connectors.Descriptor{}, connectors.Credentials{"username": "x"}), "no schema shows nothing")
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"token": botToken}))

	// An OAuth2 connector offered on the platform needs a client credential apart from its kind.
	leaky := telegramConnector(t)
	leaky.kind, leaky.nodes, leaky.sources = "sheets", nil, nil
	leaky.auth, leaky.policy = connectors.AuthOAuth2, connectors.PlatformAuto
	leaky.oauth = &connectors.OAuth2Config{AuthURL: "https://a", TokenURL: "https://t"}
	require.ErrorIs(t, dir.Register(leaky), connectors.ErrInvalid)
	leaky.oauth.ClientCredential = "sheets_client"
	require.NoError(t, dir.Register(leaky))
}

// The connections table refuses a name that is not a slug.
func TestConnectionNameMustBeASlug(t *testing.T) {
	e := newEnv(t)
	_, err := e.db.Exec(e.ctx, `INSERT INTO automation.connections (tenant_id, id, kind, name, display_name, status, key_id, nonce, secret, version)
		VALUES ($1, gen_random_uuid(), 'smtp', 'Bad', 'x', 'connected', 'k', '\x00', '\x00', 1)`, e.tenant)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr, "a name that is not a slug")
	require.Equal(t, "23514", pgErr.Code, "a check violation")
	require.Equal(t, "connections_name_check", pgErr.ConstraintName)
}

// A fingerprint stays verifiable with the key that made it after the active key rotates, and only
// for the same payload.
func TestFingerprintSurvivesKeyRotation(t *testing.T) {
	k1, err := connectors.NewKeyring("k1", map[string][]byte{"k1": key('a')})
	require.NoError(t, err)
	k2, err := connectors.NewKeyring("k2", map[string][]byte{"k1": key('a'), "k2": key('b')})
	require.NoError(t, err)
	before := connectors.NewStore(nil, k1, nil, nil, nil)
	after := connectors.NewStore(nil, k2, nil, nil, nil)
	fp, err := before.Fingerprint([]byte("payload"))
	require.NoError(t, err)
	now, err := after.Fingerprint([]byte("payload"))
	require.NoError(t, err)
	assert.NotEqual(t, fp, now)
	assert.True(t, after.MatchesFingerprint(fp, []byte("payload")))
	assert.False(t, after.MatchesFingerprint(fp, []byte("other")))
	assert.False(t, connectors.NewStore(nil, k1, nil, nil, nil).MatchesFingerprint(now, []byte("payload")), "k2 is not readable")
	assert.False(t, after.MatchesFingerprint("garbage", []byte("payload")))
}

// A credential is text; a property declared integer, number or boolean is validated as that scalar
// read from the text, exact digits included.
func TestValidateCredentialsReadsDeclaredScalars(t *testing.T) {
	d := connectors.Descriptor{Credentials: []byte(`{"type":"object","properties":{
		"port":{"type":"integer","minimum":1},"ratio":{"type":"number"},"tls":{"type":"boolean"},"host":{"type":"string"}}}`)}
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"port": "587", "ratio": "0.5", "tls": "true", "host": "42"}))
	require.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"port": "9007199254740993"}))
	for _, bad := range []connectors.Credentials{{"port": "58.7"}, {"port": "0"}, {"port": "abc"}, {"tls": "yes"}, {"ratio": "1e"}} {
		assert.Error(t, connectors.ValidateCredentials(d, bad), "%v", bad)
	}
}

// A text that still decodes past the examined depth into something long enough to hold a value is
// refused; ordinary identities stay.
func TestCarriesSecretFailsClosedPastTheDecodingDepth(t *testing.T) {
	token := "Zx9-kQ7p"
	all := func(string) bool { return true }
	creds := connectors.Credentials{"token": token}
	layered := token
	for i := 0; i < 5; i++ {
		layered = base64.RawURLEncoding.EncodeToString([]byte(layered))
	}
	assert.True(t, connectors.CarriesSecret(layered, all, creds), "five layers")
	assert.Empty(t, connectors.SafeIdentity(connectors.Identity{Display: layered + "@acme.uz", Kind: connectors.IdentityEmail}, all, creds).Display)
	for _, ok := range []string{"robot@acme.uz", "@acme_sales_bot", "+998 90 123 45 67", "Sales bot", "docs.team+drive@acme-group.uz"} {
		assert.False(t, connectors.CarriesSecret(ok, all, creds), ok)
	}
}

// A connector with providers to sign in with (OAuth2Presets) and its own authorization server
// (ConnectionAuthorizer) would have the state of one sign-in finished as the other: it is refused when it is
// registered.
type presetAuthorizer struct {
	connectors.ConnectionAuthorizer
}

func (presetAuthorizer) Kind() catalog.ConnectorKind { return "both" }
func (presetAuthorizer) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind: "both", Title: map[string]string{"en": "Both"}, Category: connectors.CategoryOther, Auth: connectors.AuthCustom,
		Credentials:   []byte(`{"type":"object","properties":{}}`),
		OAuth2Presets: map[string]connectors.OAuth2Config{"p": {AuthURL: "https://a.example/auth", TokenURL: "https://a.example/token", ClientCredential: "p_client"}},
	}
}
func (presetAuthorizer) Nodes() []catalog.NodeType         { return nil }
func (presetAuthorizer) Triggers() []catalog.TriggerSource { return nil }
func (presetAuthorizer) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{}, nil
}

func TestDirectoryRefusesPresetsOnAConnectionAuthorizer(t *testing.T) {
	err := connectors.NewDirectory(nil).Register(presetAuthorizer{})
	require.ErrorIs(t, err, connectors.ErrInvalid)
	require.ErrorContains(t, err, "ConnectionAuthorizer", "refused for this reason, not for an invalid descriptor")
}

// x-authorization-only fails closed: a non-boolean value withholds the field and the directory refuses the
// schema; only an explicit false serves it.
func TestAuthorizationOnlyFailsClosed(t *testing.T) {
	d := connectors.Descriptor{Credentials: json.RawMessage(`{"type":"object","properties":{
		"token":{"type":"string","x-secret":true,"x-authorization-only":true},
		"password":{"type":"string","x-secret":true,"x-authorization-only":"false"},
		"pin":{"type":"string","x-secret":true,"x-authorization-only":false},
		"otp":{"type":"string","x-secret":true,"x-authorization-only":null},
		"key":{"type":"string","x-secret":true}}}`)}
	assert.Equal(t, map[string]bool{"token": true, "password": true, "otp": true}, connectors.AuthorizationOnlyFields(d))

	bad := telegramConnector(t)
	bad.kind, bad.nodes, bad.sources = "api", nil, nil
	bad.schema = d.Credentials
	require.ErrorIs(t, connectors.NewDirectory(nil).Register(bad), connectors.ErrInvalid)
	bad.schema = json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","x-secret":true,"x-authorization-only":null}}}`)
	require.ErrorIs(t, connectors.NewDirectory(nil).Register(bad), connectors.ErrInvalid, "null is not a boolean")
	bad.schema = json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","x-secret":true,"x-authorization-only":true}}}`)
	require.NoError(t, connectors.NewDirectory(nil).Register(bad))
}
