package automations_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// bot is a connector with an exclusive account and a platform credential it sets up at the start.
type bot struct {
	mu      sync.Mutex
	started []string
}

func (*bot) Kind() catalog.ConnectorKind       { return catalog.KindTelegramBot }
func (*bot) Nodes() []catalog.NodeType         { return nil }
func (*bot) Triggers() []catalog.TriggerSource { return nil }
func (*bot) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: catalog.KindTelegramBot}
}
func (*bot) ExternalID(c connectors.Credentials) string { return "acct-" + c["token"] }
func (*bot) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{}, nil
}
func (b *bot) StartPlatform(_ context.Context, c connectors.Credentials) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = append(b.started, c["token"])
	return nil
}
func (b *bot) startedWith() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.started...)
}

type directory struct{ list []connectors.Connector }

func (d directory) Register(connectors.Connector) error { return nil }
func (d directory) List() []connectors.Connector        { return d.list }
func (d directory) Get(kind catalog.ConnectorKind) (connectors.Connector, bool) {
	for _, c := range d.list {
		if c.Kind() == kind {
			return c, true
		}
	}
	return nil, false
}

// pinned is a platform credential set that cannot be withdrawn: only the read side of PlatformCredentials.
type pinned struct{ connectors.PlatformCredentials }

type logged struct {
	mu   sync.Mutex
	logs []string
}

func (l *logged) warn(kind catalog.ConnectorKind, format string, _ error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, string(kind)+": "+format)
}

func startEnv(t *testing.T) (*pgxpool.Pool, uuid.UUID, *bot, connectors.Directory, map[catalog.ConnectorKind]connectors.PlatformCredential) {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_platform_start",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	b := &bot{}
	creds := map[catalog.ConnectorKind]connectors.PlatformCredential{catalog.KindTelegramBot: {Credentials: connectors.Credentials{"token": "platform"}}}
	return scope.Pool, scope.Tenant.ID, b, directory{list: []connectors.Connector{b}}, creds
}

// holdAccount saves a tenant's connection and reserves the platform credential's account for it.
func holdAccount(t *testing.T, db *pgxpool.Pool, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)})
	require.NoError(t, err)
	store := connectors.NewStore(db, keys, nil, nil, nil)
	held, err := store.Save(ctx, connectors.Connection{TenantID: tenant, Kind: catalog.KindTelegramBot, Name: "held", DisplayName: "Held"}, connectors.Credentials{"token": "platform"})
	require.NoError(t, err)
	require.NoError(t, pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		return connectors.ReserveIdentity(ctx, tx, catalog.KindTelegramBot, "acct-platform", held.ID)
	}))
	return held.ID
}

// A tenant's connection holds the account of the platform credential: the credential is withdrawn (it is served to
// nobody) and its webhook is not set, so the tenant's webhook is not replaced.
func TestAPlatformCredentialOfAHeldAccountIsWithdrawn(t *testing.T) {
	db, tenant, b, dir, creds := startEnv(t)
	ctx := context.Background()
	holdAccount(t, db, tenant)
	platform := connectors.NewPlatformCredentials(dir, creds)
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, db, nil, log.warn))
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.False(t, has, "the withdrawn credential is not served")
	assert.Empty(t, b.startedWith(), "and its webhook is not set")
	assert.NotEmpty(t, log.logs)
}

// A credential that cannot be withdrawn is not left usable next to the account a tenant holds: the start fails.
func TestAPlatformCredentialThatCannotBeWithdrawnStopsTheStart(t *testing.T) {
	db, tenant, b, dir, creds := startEnv(t)
	ctx := context.Background()
	holdAccount(t, db, tenant)
	platform := pinned{connectors.NewPlatformCredentials(dir, creds)}
	err := automations.StartPlatformConnectors(ctx, dir, platform, db, nil, (&logged{}).warn)
	require.ErrorIs(t, err, connectors.ErrIdentityTaken)
	assert.Empty(t, b.startedWith())
}

// A free account is reserved for the platform and its webhook is set; with no database to check the reservation the
// webhook is not set (the credential still sends).
func TestAPlatformCredentialIsStartedOnlyWithItsReservation(t *testing.T) {
	db, _, b, dir, creds := startEnv(t)
	ctx := context.Background()
	platform := connectors.NewPlatformCredentials(dir, creds)
	require.NoError(t, automations.StartPlatformConnectors(ctx, dir, platform, db, nil, (&logged{}).warn))
	assert.Equal(t, []string{"platform"}, b.startedWith())
	_, _, has := platform.Platform(catalog.KindTelegramBot)
	assert.True(t, has)

	offline := &bot{}
	offlineDir := directory{list: []connectors.Connector{offline}}
	log := &logged{}
	require.NoError(t, automations.StartPlatformConnectors(ctx, offlineDir, connectors.NewPlatformCredentials(offlineDir, creds), nil, fmt.Errorf("no database"), log.warn))
	assert.Empty(t, offline.startedWith(), "the reservation cannot be made: no webhook that could replace a tenant's")
	assert.NotEmpty(t, log.logs)
}
