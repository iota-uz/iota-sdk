package connectors_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

// The migration applies and rolls back: its Down then its Up run on a migrated database.
func TestMigrationRollsBackAndApplies(t *testing.T) {
	e := newEnv(t)
	const name = "changes-1795256615.sql"
	up, down := migrationSections(t, name)
	// Later migrations may depend on these tables: they are rolled back first, newest first, and
	// applied again after.
	later := laterMigrations(t, name)
	for i := len(later) - 1; i >= 0; i-- {
		_, d := migrationSections(t, later[i])
		_, err := e.db.Exec(e.ctx, d)
		require.NoError(t, err, later[i])
	}

	tables := func() int {
		var n int
		require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'automation'`).Scan(&n))
		return n
	}
	assert.Equal(t, 3, tables())
	_, err := e.db.Exec(e.ctx, down)
	require.NoError(t, err)
	assert.Equal(t, 0, tables())
	var schema int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'automation'`).Scan(&schema))
	assert.Zero(t, schema)
	_, err = e.db.Exec(e.ctx, up)
	require.NoError(t, err)
	assert.Equal(t, 3, tables())
	for _, m := range later {
		u, _ := migrationSections(t, m)
		_, err := e.db.Exec(e.ctx, u)
		require.NoError(t, err, m)
	}

	// Constraints of the table.
	_, err = e.db.Exec(e.ctx, `INSERT INTO automation.connections (tenant_id, id, kind, name, display_name, status, key_id, nonce, secret, version)
		VALUES ($1, gen_random_uuid(), 'smtp', 'Bad', 'x', 'connected', 'k', '\x00', '\x00', 1)`, e.tenant)
	require.Error(t, err, "a name that is not a slug")
}

// migrationSections are the Up and Down SQL of a migration file.
func migrationSections(t *testing.T, name string) (string, string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("migrations", name))
	require.NoError(t, err)
	up, down, ok := strings.Cut(string(body), "-- +migrate Down")
	require.True(t, ok, name)
	return strings.TrimPrefix(up, "-- +migrate Up"), down
}

// laterMigrations are the migration files applied after name that use the automation schema (the
// ones that can depend on its tables), oldest first. Others are left alone: a notransaction
// migration cannot run as one statement batch.
func laterMigrations(t *testing.T, name string) []string {
	t.Helper()
	entries, err := os.ReadDir("migrations")
	require.NoError(t, err)
	var out []string
	for _, entry := range entries {
		n := entry.Name()
		if !strings.HasSuffix(n, ".sql") || n <= name {
			continue
		}
		body, err := os.ReadFile(filepath.Join("migrations", n))
		require.NoError(t, err)
		if strings.Contains(string(body), "automation.") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
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
