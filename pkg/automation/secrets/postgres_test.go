package secrets_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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
	require.ErrorIs(t, err, secrets.ErrVersionConflict, "a stale version")

	list, err := e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotNil(t, list[0].LastUsedAt, "the use is recorded")

	require.NoError(t, e.mgr.Delete(e.ctx, e.tenant, s.ID))
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrNotFound)
	_, err = e.store.Read(e.ctx, s.Ref)
	require.ErrorIs(t, err, secrets.ErrNotFound, "the value is gone with the record")
	require.ErrorIs(t, e.mgr.Delete(e.ctx, e.tenant, s.ID), secrets.ErrNotFound)
}

func TestCreateValidatesAndRefusesATakenName(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	for _, in := range []secrets.CreateInput{
		{Name: "api_key", Value: "x"}, {Name: "API_KEY", Value: ""}, {Name: "API_KEY", Value: string(make([]byte, secrets.MaxValueBytes+1))},
	} {
		_, err := e.mgr.Create(e.ctx, e.tenant, in)
		require.ErrorIs(t, err, secrets.ErrInvalid)
	}
	_, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "A", Value: "x"})
	require.NoError(t, err)
	_, err = e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "A", Value: "y"})
	require.ErrorIs(t, err, secrets.ErrNameTaken)
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
	require.ErrorIs(t, err, secrets.ErrVersionConflict)
	_, err = e.mgr.Update(e.ctx, e.tenant, secrets.UpdateInput{ID: uuid.New(), Version: 1, Description: &desc})
	require.ErrorIs(t, err, secrets.ErrNotFound)
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
	require.ErrorIs(t, err, secrets.ErrNotFound)
	// The path of A's value under B's tenant: not found; without a tenant: refused.
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: b, Path: s.Ref.Path})
	require.ErrorIs(t, err, secrets.ErrNotFound)
	_, err = e.store.Read(e.ctx, secrets.Ref{Path: s.Ref.Path})
	require.ErrorIs(t, err, secrets.ErrForbidden)
	_, err = e.store.Put(e.ctx, secrets.Ref{}, map[string]string{"value": "x"})
	require.ErrorIs(t, err, secrets.ErrForbidden)
	require.ErrorIs(t, e.mgr.Delete(e.ctx, b, s.ID), secrets.ErrNotFound)
	_, err = e.mgr.Rotate(e.ctx, b, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "stolen"})
	require.ErrorIs(t, err, secrets.ErrNotFound)
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
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	require.NotErrorIs(t, err, secrets.ErrNotFound)

	// The records unreachable (a closed context) are an outage too, not a missing secret.
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	_, err = e.res.Resolve(ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrUnavailable)

	// No keyring: the sealed adapter is unavailable, never empty.
	noKeys := connectors.NewSealedSecrets(e.db, nil)
	_, err = secrets.NewResolver(e.db, noKeys).Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	require.ErrorIs(t, err, connectors.ErrNotConfigured)
}

func TestSealedValuesAreVersioned(t *testing.T) {
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
	// An earlier version stays readable by number until it is pruned.
	v, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: "p/1", Version: 1})
	require.NoError(t, err)
	assert.Equal(t, secrets.Value{Data: map[string]string{"a": "1", "b": "2"}, Version: 1}, v)
	require.NoError(t, e.store.Prune(e.ctx, ref, 2))
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: "p/1", Version: 1})
	require.ErrorIs(t, err, secrets.ErrNotFound)

	// A row moved to another path does not open (the path is bound).
	_, err = e.db.Exec(e.ctx, `UPDATE automation.secret_values SET path = 'p/2' WHERE tenant_id = $1`, e.tenant)
	require.NoError(t, err)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: "p/2"})
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	require.ErrorIs(t, err, connectors.ErrUndecryptable)
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

// cancelAfterPut is a Writer whose Put succeeds and then cancels the caller's context: the records' write that
// follows the value's fails, as a lost connection or a crash between the two writes would make it.
type cancelAfterPut struct {
	secrets.Writer
	cancel context.CancelFunc
}

func (w cancelAfterPut) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	md, err := w.Writer.Put(context.WithoutCancel(ctx), ref, data)
	w.cancel()
	return md, err
}

// failingDelete is a Writer whose Delete fails (the store is down when a secret is deleted).
type failingDelete struct{ secrets.Writer }

func (failingDelete) Delete(context.Context, secrets.Ref) error { return secrets.ErrUnavailable }

func TestARotationThatFailsAfterTheValueKeepsThePublishedVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(e.ctx)
	broken := secrets.NewManager(e.db, cancelAfterPut{Writer: e.store, cancel: cancel}, e.store, secrets.DefaultLocator("test/"))
	_, err = broken.Rotate(ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.Error(t, err, "the rotation is reported as failed")

	// The value's version 2 was written, yet nothing reads it: the record still publishes version 1.
	md, err := e.store.Metadata(e.ctx, s.Ref)
	require.NoError(t, err)
	assert.Equal(t, 2, md.Version)
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v1", got)
	list, err := e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, s.Version, list[0].Version)
	assert.Equal(t, 1, list[0].ValueVersion)

	// A retry publishes a new version and prunes the ones before it.
	rotated, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v3"})
	require.NoError(t, err)
	assert.Equal(t, 3, rotated.ValueVersion)
	got, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v3", got)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, Version: 1})
	require.ErrorIs(t, err, secrets.ErrNotFound, "earlier versions are pruned once a later one is published")
}

func TestACreateThatFailsAfterTheValueLeavesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx, cancel := context.WithCancel(e.ctx)
	broken := secrets.NewManager(e.db, cancelAfterPut{Writer: e.store, cancel: cancel}, e.store, secrets.DefaultLocator("test/"))
	_, err := broken.Create(ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.Error(t, err)
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrNotFound)
	var n int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1`, e.tenant).Scan(&n))
	assert.Zero(t, n, "the value written for the record that never committed is removed")
}

func TestADeleteWhoseValueCannotBeRemovedIsRetried(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)

	down := secrets.NewManager(e.db, failingDelete{Writer: e.store}, e.store, secrets.DefaultLocator("test/"))
	require.NoError(t, down.Delete(e.ctx, e.tenant, s.ID))
	// The name is gone at once, never live without its value; the value waits for the store.
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrNotFound)
	has, err := e.res.Has(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.False(t, has)
	list, err := down.List(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Empty(t, list)
	var values int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1`, e.tenant).Scan(&values))
	assert.Equal(t, 1, values)
	// The name is free for a new secret meanwhile.
	again, err := down.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v2"})
	require.NoError(t, err)

	// The next call of a manager with a working store removes the old value; the tombstone stays while a
	// write claimed just before the delete could still land, and goes once that window is over.
	_, err = e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, s.Ref.Path).Scan(&values))
	assert.Zero(t, values)
	var tombs int
	_, err = e.db.Exec(e.ctx, `UPDATE automation.secrets SET value_write_at = now() - interval '1 hour' WHERE id = $1`, s.ID)
	require.NoError(t, err)
	_, err = e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secrets WHERE tenant_id = $1 AND deleted_at IS NOT NULL`, e.tenant).Scan(&tombs))
	assert.Zero(t, tombs)
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
	assert.NotEqual(t, s.ID, again.ID)
}

// rotatingReader rotates the secret twice before its first read of a published version: the version the
// resolver looked up is pruned between its lookup and its read, as a rotation racing a reader does.
type rotatingReader struct {
	secrets.Reader
	once   *bool
	rotate func()
}

func (r rotatingReader) Read(ctx context.Context, ref secrets.Ref) (secrets.Value, error) {
	if !*r.once && ref.Version > 0 {
		*r.once = true
		r.rotate()
	}
	return r.Reader.Read(ctx, ref)
}

func TestAReaderFollowsARotationThatPrunedItsVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	done := false
	res := secrets.NewResolver(e.db, rotatingReader{Reader: e.store, once: &done, rotate: func() {
		r1, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
		require.NoError(t, err)
		_, err = e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: r1.Version, Value: "v3"})
		require.NoError(t, err)
	}})
	got, err := res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err, "a live secret on a healthy store resolves")
	assert.Equal(t, "v3", got)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, Version: 1})
	require.ErrorIs(t, err, secrets.ErrNotFound, "version 1 was pruned under the reader")

	// The published version missing while the record stays as it is: a broken store, a controlled error.
	_, err = e.db.Exec(e.ctx, `DELETE FROM automation.secret_values WHERE tenant_id = $1`, e.tenant)
	require.NoError(t, err)
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrUnavailable)
}

// The previous version stays readable after a rotation, for a reader that looked the record up before it.
func TestARotationKeepsThePreviousVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	r, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err)
	v, err := e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, Version: 1})
	require.NoError(t, err)
	assert.Equal(t, "v1", v.Data[secrets.ValueKey])
	_, err = e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: r.Version, Value: "v3"})
	require.NoError(t, err)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, Version: 1})
	require.ErrorIs(t, err, secrets.ErrNotFound)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, Version: 2})
	require.NoError(t, err)
}

// A COMMIT whose reply is lost after the record was published: the manager settles it from the record and
// never removes the published value.
func TestALostPublishReplyKeepsThePublishedValue(t *testing.T) {
	e := newEnv(t)
	restore := secrets.SetPublishHook(errors.New("connection reset while reading the COMMIT reply"))
	defer restore()
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err, "the record is published: the create happened")
	assert.Equal(t, 1, s.ValueVersion)
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v1", got)

	r, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err, "the rotation is published: it happened")
	assert.Equal(t, 2, r.ValueVersion)
	restore()
	got, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
}

// With a pool of one connection (the store and the records share it) every mutation completes: no step
// holds a connection while the value store waits for one.
func TestMutationsCompleteOnAPoolOfOneConnection(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	cfg := e.db.Config().Copy()
	cfg.MaxConns, cfg.MinConns = 1, 0
	one, err := pgxpool.NewWithConfig(e.ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(one.Close)
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	st := connectors.NewSealedSecrets(one, keys)
	mgr := secrets.NewManager(one, st, st, secrets.DefaultLocator("test/"))
	res := secrets.NewResolver(one, st)

	ctx, cancel := context.WithTimeout(e.ctx, 20*time.Second)
	defer cancel()
	s, err := mgr.Create(ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	r, err := mgr.Rotate(ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err)
	got, err := res.Resolve(ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
	_, err = mgr.Update(ctx, e.tenant, secrets.UpdateInput{ID: s.ID, Version: r.Version, Name: ptr("CRM_KEY")})
	require.NoError(t, err)
	require.NoError(t, mgr.Delete(ctx, e.tenant, s.ID))
	_, err = mgr.List(ctx, e.tenant)
	require.NoError(t, err)
}

func ptr[T any](v T) *T { return &v }

// A staged record that never published (a create that died after its value was written) is removed with its
// value once it is stale; until then its name is reserved and nothing reads it.
func TestAStaleStagedRecordIsPurged(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	id := uuid.New()
	path := "test/tenants/" + e.tenant.String() + "/automation/secrets/" + id.String()
	_, err := e.db.Exec(e.ctx, `INSERT INTO automation.secrets (id, tenant_id, name, ref_path, value_version, created_at, updated_at)
		VALUES ($1, $2, 'API_KEY', $3, 0, now() - interval '1 hour', now() - interval '1 hour')`, id, e.tenant, path)
	require.NoError(t, err)
	_, err = e.store.Put(e.ctx, secrets.Ref{TenantID: e.tenant, Path: path}, map[string]string{secrets.ValueKey: "orphan"})
	require.NoError(t, err)

	has, err := e.res.Has(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.False(t, has, "a staged record is not readable")
	_, err = e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "fresh"})
	require.NoError(t, err, "the stale staged record is purged first, freeing its name")
	var n int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, path).Scan(&n))
	assert.Zero(t, n)
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "fresh", got)
}

// Concurrent Puts at one path each get the version they wrote: none is handed a later writer's version.
func TestConcurrentPutsReturnTheirOwnVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ref := secrets.Ref{TenantID: e.tenant, Path: "p/race"}
	const writers = 8
	type wrote struct {
		value   string
		version int
		err     error
	}
	out := make(chan wrote, writers)
	for i := range writers {
		go func() {
			v := fmt.Sprintf("value-%d", i)
			md, err := e.store.Put(e.ctx, ref, map[string]string{secrets.ValueKey: v})
			out <- wrote{value: v, version: md.Version, err: err}
		}()
	}
	seen := map[int]bool{}
	for range writers {
		w := <-out
		require.NoError(t, w.err)
		assert.False(t, seen[w.version], "version %d returned twice", w.version)
		seen[w.version] = true
		got, err := e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: ref.Path, Version: w.version})
		require.NoError(t, err)
		assert.Equal(t, w.value, got.Data[secrets.ValueKey], "version %d holds what its Put wrote", w.version)
	}
}

// overlapping runs another rotation of the same secret inside this one's Put, after its value is written.
type overlapping struct {
	secrets.Writer
	during func()
}

func (w overlapping) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	md, err := w.Writer.Put(ctx, ref, data)
	if w.during != nil {
		w.during()
	}
	return md, err
}

// Two rotations of one secret from the same version: only one publishes, and what is published is the value
// of the one that won.
func TestOverlappingRotationsPublishTheWinnersValue(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	a := secrets.NewManager(e.db, overlapping{Writer: e.store, during: func() {
		_, err := e.mgr.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "from-b"})
		require.NoError(t, err)
	}}, e.store, secrets.DefaultLocator("test/"))
	_, err = a.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "from-a"})
	require.ErrorIs(t, err, secrets.ErrVersionConflict, "the rotation that lost reports it")
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "from-b", got)
}

// A delete that overtakes a rotation: the value the rotation writes afterwards is removed, by the rotation
// itself, or by purge from the tombstone when the rotation's process stops before it could.
func TestALateRotationAfterADeleteLeavesNoValue(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	late := secrets.NewManager(e.db, deleteFirst{Writer: e.store, del: func() { require.NoError(t, e.mgr.Delete(e.ctx, e.tenant, s.ID)) }},
		e.store, secrets.DefaultLocator("test/"))
	_, err = late.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.ErrorIs(t, err, secrets.ErrNotFound)
	values := func() int {
		var n int
		require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, s.Ref.Path).Scan(&n))
		return n
	}
	assert.Zero(t, values(), "the rotation removed what it wrote after the delete")

	// The rotation's process stopped right after its write: the tombstone (kept for the claim) leads purge to it.
	_, err = e.store.Put(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path}, map[string]string{secrets.ValueKey: "landed-late"})
	require.NoError(t, err)
	require.Equal(t, 1, values())
	_, err = e.mgr.List(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Zero(t, values(), "purge found the late value through the tombstone")
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrNotFound)
}

// deleteFirst deletes the secret (another administrator) right before its Put writes.
type deleteFirst struct {
	secrets.Writer
	del func()
}

func (w deleteFirst) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	w.del()
	return w.Writer.Put(ctx, ref, data)
}

// flakyStore fails Delete while down (a transient outage of the value store).
type flakyStore struct {
	secrets.Writer
	down *bool
}

func (f flakyStore) Delete(ctx context.Context, ref secrets.Ref) error {
	if *f.down {
		return secrets.ErrUnavailable
	}
	return f.Writer.Delete(ctx, ref)
}

// A create given up while the store cannot delete its value: the staged record becomes a tombstone that keeps
// the reference, so the value is removed once the store is back.
func TestAGivenUpCreateKeepsItsReferenceAcrossAStoreOutage(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	down := true
	ctx, cancel := context.WithCancel(e.ctx)
	w := flakyStore{Writer: cancelAfterPut{Writer: e.store, cancel: cancel}, down: &down}
	m := secrets.NewManager(e.db, w, e.store, secrets.DefaultLocator("test/"))
	_, err := m.Create(ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.Error(t, err)

	var path string
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT ref_path FROM automation.secrets WHERE tenant_id = $1 AND deleted_at IS NOT NULL`, e.tenant).Scan(&path),
		"the given-up record is a tombstone with its reference")
	var n int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, path).Scan(&n))
	assert.Equal(t, 1, n, "the store was down: the value is still there")
	_, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.ErrorIs(t, err, secrets.ErrNotFound)
	// While the store is down a cleanup changes nothing; once it is back, List (the first healthy call)
	// removes the value.
	_, err = m.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, path).Scan(&n))
	assert.Equal(t, 1, n, "still down: the value waits")
	down = false
	_, err = m.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, path).Scan(&n))
	assert.Zero(t, n, "purge removed it once the store was back")

	_, err = e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v2"})
	require.NoError(t, err, "the name is free")
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
}

// A fenced Put commits only before its NotAfter, by the database's clock.
func TestTheSealedStoreRefusesAWritePastItsFence(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	past := secrets.Ref{TenantID: e.tenant, Path: "p/fenced", NotAfter: time.Now().Add(-time.Second)}
	_, err := e.store.Put(e.ctx, past, map[string]string{secrets.ValueKey: "late"})
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	_, err = e.store.Read(e.ctx, secrets.Ref{TenantID: e.tenant, Path: "p/fenced"})
	require.ErrorIs(t, err, secrets.ErrNotFound, "nothing was written")

	future := secrets.Ref{TenantID: e.tenant, Path: "p/fenced", NotAfter: time.Now().Add(time.Minute)}
	md, err := e.store.Put(e.ctx, future, map[string]string{secrets.ValueKey: "in time"})
	require.NoError(t, err)
	assert.Equal(t, 1, md.Version)
}

// fenceSpy records the fence of every Put; with expire set, its first Delete first makes the record's claim
// expire (the clock passing the window while the store deletes) and the Deletes after it fail (the store goes
// down, so only the first cleanup's decision counts).
type fenceSpy struct {
	secrets.Writer
	fences  *[]time.Time
	expire  func()
	deletes *int
}

func (f fenceSpy) Put(ctx context.Context, ref secrets.Ref, data map[string]string) (secrets.Metadata, error) {
	*f.fences = append(*f.fences, ref.NotAfter)
	return f.Writer.Put(ctx, ref, data)
}

func (f fenceSpy) Delete(ctx context.Context, ref secrets.Ref) error {
	if f.expire != nil {
		*f.deletes++
		if *f.deletes > 1 {
			return secrets.ErrUnavailable
		}
		f.expire()
	}
	return f.Writer.Delete(ctx, ref)
}

// The expiration boundary of a tombstone. The manager fences each write it claims at the claim plus half the
// window. A cleanup that decided on a live claim keeps the tombstone even when the claim expires while the
// value is deleted; a write that lands before its fence after that delete is removed by the next cleanup,
// which decides on the expired claim and only then retires the tombstone. Expiry is driven by the records'
// clock (the claim is moved into the past), never by waiting.
func TestATombstoneOutlivesEveryWriteClaimedBeforeItsRetirement(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var fences []time.Time
	m := secrets.NewManager(e.db, fenceSpy{Writer: e.store, fences: &fences}, e.store, secrets.DefaultLocator("test/"))
	s, err := m.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)
	_, err = m.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err)
	require.Len(t, fences, 2)
	var claim time.Time
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT value_write_at FROM automation.secrets WHERE id = $1`, s.ID).Scan(&claim))
	window := secrets.WriteWindow()
	assert.True(t, fences[1].Equal(claim.Add(window/2)), "the rotation's fence is its claim plus half the window: %s, claim %s", fences[1], claim)
	fence := fences[1]

	// The delete's cleanup decides while the claim is live; the claim expires while the store deletes.
	deletes := 0
	stalled := secrets.NewManager(e.db, fenceSpy{Writer: e.store, fences: &fences, deletes: &deletes, expire: func() {
		_, err := e.db.Exec(e.ctx, `UPDATE automation.secrets SET value_write_at = value_write_at - make_interval(secs => $2) WHERE id = $1`,
			s.ID, 2*window.Seconds())
		require.NoError(t, err)
	}}, e.store, secrets.DefaultLocator("test/"))
	require.NoError(t, stalled.Delete(e.ctx, e.tenant, s.ID))
	var tombs int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secrets WHERE id = $1 AND deleted_at IS NOT NULL`, s.ID).Scan(&tombs))
	assert.Equal(t, 1, tombs, "decided on a live claim: the tombstone stays though the claim expired meanwhile")

	// The rotation claimed before the delete lands its write before its fence, after the value was deleted.
	_, err = e.store.Put(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path, NotAfter: fence}, map[string]string{secrets.ValueKey: "late"})
	require.NoError(t, err, "the manager's fence has not passed")
	values := func() int {
		var n int
		require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, s.Ref.Path).Scan(&n))
		return n
	}
	require.Equal(t, 1, values())

	// The next cleanup decides on the expired claim: it removes the late value and retires the tombstone.
	_, err = m.List(e.ctx, e.tenant)
	require.NoError(t, err)
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secrets WHERE id = $1`, s.ID).Scan(&tombs))
	assert.Zero(t, tombs)
	assert.Zero(t, values())
}

// blockingDelete is a store whose Delete hangs until its context ends (a store that times out).
type blockingDelete struct {
	secrets.Writer
	calls *int
}

func (b blockingDelete) Delete(ctx context.Context, _ secrets.Ref) error {
	*b.calls++
	<-ctx.Done()
	return ctx.Err()
}

// One cleanup batch is bounded as a whole: a store that hangs on every delete costs a call one cleanup
// deadline, not one per tombstone, and the rest waits for the next call.
func TestCleanupIsBoundedAcrossTheBatch(t *testing.T) {
	restore := secrets.SetCleanupTimeout(300 * time.Millisecond)
	defer restore()
	e := newEnv(t)
	for i := range 4 {
		s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: fmt.Sprintf("KEY_%d", i), Value: "v"})
		require.NoError(t, err)
		_, err = e.db.Exec(e.ctx, `UPDATE automation.secrets SET deleted_at = now() WHERE id = $1`, s.ID)
		require.NoError(t, err)
	}
	calls := 0
	m := secrets.NewManager(e.db, blockingDelete{Writer: e.store, calls: &calls}, e.store, secrets.DefaultLocator("test/"))
	started := time.Now()
	_, err := m.List(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Less(t, time.Since(started), 2*time.Second, "one deadline for the batch")
	assert.Equal(t, 1, calls, "the batch stopped at its deadline")
	var tombs int
	require.NoError(t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secrets WHERE tenant_id = $1 AND deleted_at IS NOT NULL`, e.tenant).Scan(&tombs))
	assert.Equal(t, 4, tombs, "unfinished tombstones wait for the next cleanup")
}
