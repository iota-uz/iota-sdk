package secrets_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

type env struct {
	t      *testing.T
	ctx    context.Context
	db     *pgxpool.Pool
	tenant uuid.UUID
	store  secrets.Store
	mgr    secrets.Manager
	res    secrets.Resolver
}

func newEnv(t *testing.T) *env {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_secrets",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	st := connectors.NewSealedSecrets(scope.Pool, keys)
	return &env{t: t, ctx: context.Background(), db: scope.Pool, tenant: scope.Tenant.ID, store: st,
		mgr: secrets.NewManager(scope.Pool, st, st, secrets.DefaultLocator("test/")), res: secrets.NewResolver(scope.Pool, st)}
}

func (e *env) newTenant() uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	_, err := e.db.Exec(e.ctx, `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.t, err)
	return id
}

func TestResolveRotateDelete(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Description: "crm", Value: "v1", CreatedBy: 7})
	require.NoError(t, err)
	assert.Equal(t, 1, s.ValueVersion)
	assert.Contains(t, s.Ref.Path, s.ID.String())

	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v1", got)

	rotated, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err)
	assert.Equal(t, 2, rotated.ValueVersion)
	assert.Equal(t, s.Version+1, rotated.Version)
	// The next read of this resolver and of another one built later see the new value.
	got, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
	other := secrets.NewResolver(e.db, e.store)
	got, err = other.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)

	_, err = e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v3"})
	assert.ErrorIs(t, err, secrets.ErrVersionConflict, "a stale version")

	list, err := e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotNil(t, list[0].LastUsedAt, "the use is recorded")

	require.NoError(t, e.mgr.Delete(e.ctx, e.tenant, s.ID))
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	assert.ErrorIs(t, err, secrets.ErrNotFound)
	_, err = e.store.Read(e.ctx, s.Ref)
	assert.ErrorIs(t, err, secrets.ErrNotFound, "the value is gone with the record")
	assert.ErrorIs(t, e.mgr.Delete(e.ctx, e.tenant, s.ID), secrets.ErrNotFound)
}

func TestCreateValidatesAndRefusesATakenName(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, in := range []secrets.CreateInput{
		{Name: "api_key", Value: "x"}, {Name: "API_KEY", Value: ""}, {Name: "API_KEY", Value: string(make([]byte, secrets.MaxValueBytes+1))},
	} {
		_, err := e.mgr.Create(e.ctx, e.tenant, in)
		assert.ErrorIs(t, err, secrets.ErrInvalid)
	}
	_, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "A", Value: "x"})
	require.NoError(t, err)
	_, err = e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "A", Value: "y"})
	assert.ErrorIs(t, err, secrets.ErrNameTaken)
	var n int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1`, e.tenant).Scan(&n))
	assert.Equal(t, 1, n, "the value of the refused secret is cleaned up")
}

func TestUpdateRenamesWithVersionCheck(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "A", Value: "x"})
	require.NoError(t, err)
	name, desc := "B", "new"
	u, err := e.mgr.Update(e.ctx, e.tenant, secrets.UpdateInput{ID: s.ID, Version: s.Version, Name: &name, Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, "B", u.Name)
	_, err = e.mgr.Update(e.ctx, e.tenant, secrets.UpdateInput{ID: s.ID, Version: s.Version, Description: &desc})
	assert.ErrorIs(t, err, secrets.ErrVersionConflict)
	_, err = e.mgr.Update(e.ctx, e.tenant, secrets.UpdateInput{ID: uuid.New(), Version: 1, Description: &desc})
	assert.ErrorIs(t, err, secrets.ErrNotFound)
	got, err := e.res.Resolve(e.ctx, e.tenant, "B")
	require.NoError(t, err)
	assert.Equal(t, "x", got)
}

func TestAnotherTenantNeverReadsIt(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "mine"})
	require.NoError(t, err)
	b := e.newTenant()

	_, err = e.res.Resolve(e.ctx, b, "API_KEY")
	assert.ErrorIs(t, err, secrets.ErrNotFound)
	// The path of A's value under B's tenant: not found; without a tenant: refused.
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: b, Path: s.Ref.Path})
	assert.ErrorIs(t, err, secrets.ErrNotFound)
	_, err = e.store.Read(e.ctx, secrets.Ref{Path: s.Ref.Path})
	assert.ErrorIs(t, err, secrets.ErrForbidden)
	_, err = e.store.Put(e.ctx, secrets.Ref{}, map[string]string{"value": "x"})
	assert.ErrorIs(t, err, secrets.ErrForbidden)
	assert.ErrorIs(t, e.mgr.Delete(e.ctx, b, s.ID), secrets.ErrNotFound)
	_, err = e.mgr.Rotate(e.ctx, b, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "stolen"})
	assert.ErrorIs(t, err, secrets.ErrNotFound)
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "mine", got)
}

type failingReader struct{}

func (failingReader) Read(context.Context, secrets.Ref) (secrets.Value, error) {
	return secrets.Value{}, errors.New("connection refused to the vault")
}

func TestAStoreOutageIsAControlledError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v"})
	require.NoError(t, err)
	_, err = secrets.NewResolver(e.db, failingReader{}).Resolve(e.ctx, e.tenant, "API_KEY")
	assert.ErrorIs(t, err, secrets.ErrUnavailable)
	assert.NotErrorIs(t, err, secrets.ErrNotFound)

	// The records unreachable (a closed context) are an outage too, not a missing secret.
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	_, err = e.res.Resolve(ctx, e.tenant, "API_KEY")
	assert.ErrorIs(t, err, secrets.ErrUnavailable)

	// No keyring: the sealed adapter is unavailable, never empty.
	noKeys := connectors.NewSealedSecrets(e.db, nil)
	_, err = secrets.NewResolver(e.db, noKeys).Resolve(e.ctx, e.tenant, "API_KEY")
	assert.ErrorIs(t, err, secrets.ErrUnavailable)
	assert.ErrorIs(t, err, connectors.ErrNotConfigured)
}

func TestSealedValuesRoundTripAndKeepOneVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ref := secrets.Ref{TenantID: e.tenant, Path: "p/1"}
	md, err := e.store.Put(e.ctx, ref, map[string]string{"a": "1", "b": "2"})
	require.NoError(t, err)
	assert.Equal(t, 1, md.Version)
	md, err = e.store.Put(e.ctx, ref, map[string]string{"a": "3", "b": "4"})
	require.NoError(t, err)
	assert.Equal(t, 2, md.Version)
	v, err := e.store.Read(e.ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, secrets.Value{Data: map[string]string{"a": "3", "b": "4"}, Version: 2}, v)
	m, err := e.store.Metadata(e.ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, 2, m.Version)

	// A row moved to another path does not open (the path is bound).
	_, err = e.db.Exec(e.ctx, `UPDATE automation.secret_values SET path = 'p/2' WHERE tenant_id = $1`, e.tenant)
	require.NoError(t, err)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: "p/2"})
	assert.ErrorIs(t, err, secrets.ErrUnavailable)
	assert.ErrorIs(t, err, connectors.ErrUndecryptable)
}

func TestRewrapReSealsSecretValues(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v"})
	require.NoError(t, err)
	both, err := connectors.NewKeyring("k2", map[string][]byte{"k1": bytes.Repeat([]byte{1}, 32), "k2": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	cs := connectors.NewStore(e.db, both, nil, nil, nil)
	n, err := cs.Rewrap(e.ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	var keyID string
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT key_id FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, s.Ref.Path).Scan(&keyID))
	assert.Equal(t, "k2", keyID)
	// The re-sealed value reads under the new keyring alone, and keeps its version.
	only, err := connectors.NewKeyring("k2", map[string][]byte{"k2": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	v, err := connectors.NewSealedSecrets(e.db, only).Read(e.ctx, s.Ref)
	require.NoError(t, err)
	assert.Equal(t, "v", v.Data[secrets.ValueKey])
	assert.Equal(t, 1, v.Version)
}
