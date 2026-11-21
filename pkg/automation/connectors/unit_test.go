package connectors_test

import (
	"encoding/base64"
	"os"
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

	k, err = connectors.ParseKeyring("  ")
	assert.NoError(t, err)
	assert.Nil(t, k)
	for _, bad := range []string{"nokey", "a:" + base64.StdEncoding.EncodeToString([]byte("short")), "a:" + b64 + ",a:" + b64, "bad id:" + b64} {
		_, err := connectors.ParseKeyring(bad)
		assert.Error(t, err, bad)
		if err != nil {
			assert.NotContains(t, err.Error(), b64)
		}
	}
	_, err = connectors.NewKeyring("missing", map[string][]byte{"k": key('a')})
	assert.Error(t, err)
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
	assert.ErrorIs(t, dir.Register(oauthless), connectors.ErrInvalid)
	badPolicy := telegramConnector(t)
	badPolicy.kind, badPolicy.nodes, badPolicy.sources = "x", nil, nil
	badPolicy.policy = "everyone"
	assert.ErrorIs(t, dir.Register(badPolicy), connectors.ErrInvalid)

	d := tg.Descriptor()
	assert.Equal(t, map[string]bool{"token": true, "access_token": true}, connectors.SecretFields(d))
	assert.Equal(t, map[string]string{"username": "support"}, connectors.RedactedCredentials(d, connectors.Credentials{"token": botToken, "username": "support"}))
	assert.NoError(t, connectors.ValidateCredentials(d, connectors.Credentials{"token": botToken}))
}

// The migration applies and rolls back: its Down then its Up run on a migrated database.
func TestMigrationRollsBackAndApplies(t *testing.T) {
	e := newEnv(t)
	body, err := os.ReadFile("migrations/changes-1795256615.sql")
	require.NoError(t, err)
	up, down, ok := strings.Cut(string(body), "-- +migrate Down")
	require.True(t, ok)
	up = strings.TrimPrefix(up, "-- +migrate Up")

	tables := func() int {
		var n int
		require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'automation'`).Scan(&n))
		return n
	}
	assert.Equal(t, 3, tables())
	_, err = e.db.Exec(e.ctx, down)
	require.NoError(t, err)
	assert.Equal(t, 0, tables())
	var schema int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'automation'`).Scan(&schema))
	assert.Zero(t, schema)
	_, err = e.db.Exec(e.ctx, up)
	require.NoError(t, err)
	assert.Equal(t, 3, tables())

	// Constraints of the table.
	_, err = e.db.Exec(e.ctx, `INSERT INTO automation.connections (tenant_id, id, kind, name, display_name, status, key_id, nonce, secret, version)
		VALUES ($1, gen_random_uuid(), 'smtp', 'Bad', 'x', 'connected', 'k', '\x00', '\x00', 1)`, e.tenant)
	assert.Error(t, err, "a name that is not a slug")
}
