package secrets_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
)

// valueVersions is how many versions of a value the sealed store holds at path.
func (e *env) valueVersions(path string) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.db.QueryRow(e.ctx, `SELECT count(*) FROM automation.secret_values WHERE tenant_id = $1 AND path = $2`, e.tenant, path).Scan(&n))
	return n
}

// A live secret created under the old prefix (test/) at a prefix change (v2/): a locator that drops the old
// root refuses its rotation before anything is written (a write there would lie under no swept root); one
// that keeps it in FormerRoots rotates it, and a write that lands under the old root after the secret was
// deleted and its tombstone retired is swept.
func TestALiveSecretUnderAFormerRootAtCutover(t *testing.T) {
	t.Parallel()
	e := newEnv(t) // e.mgr locates under test/
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)

	dropped := secrets.NewManager(e.db, e.store, secrets.DefaultLocator("v2/"))
	_, err = dropped.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.ErrorIs(t, err, secrets.ErrUnavailable, "the old root is swept no more: no write lands there")
	assert.Equal(t, 1, e.valueVersions(s.Ref.Path), "nothing was written")
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v1", got)

	moved := secrets.NewManager(e.db, e.store, movedLocator{Locator: secrets.DefaultLocator("v2/")})
	rotated, err := moved.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.NoError(t, err, "the former root is still swept: the rotation goes on under it")
	assert.Equal(t, s.Ref.Path, rotated.Ref.Path, "a record keeps its path")
	got, err = e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)

	// Deleted, its tombstone retired; a rotation's write the store held lands under the old root now.
	require.NoError(t, moved.Delete(e.ctx, e.tenant, s.ID))
	_, err = e.db.Exec(e.ctx, `DELETE FROM automation.secrets WHERE id = $1`, s.ID)
	require.NoError(t, err)
	_, err = e.store.Put(e.ctx, secrets.Ref{TenantID: e.tenant, Path: s.Ref.Path}, map[string]string{secrets.ValueKey: "late"})
	require.NoError(t, err)
	_, err = moved.List(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Zero(t, e.valueVersions(s.Ref.Path), "swept from the former root")
}

// hangingStore hangs on every List and Delete until its context ends (a store that times out), and
// counts the calls.
type hangingStore struct {
	secrets.Writer
	mu    *sync.Mutex
	calls *int
}

func (h hangingStore) wait(ctx context.Context) error {
	h.mu.Lock()
	*h.calls++
	h.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (h hangingStore) List(ctx context.Context, _ secrets.Ref) ([]string, error) {
	return nil, h.wait(ctx)
}

func (h hangingStore) Delete(ctx context.Context, _ secrets.Ref) error { return h.wait(ctx) }

// threeRoots sweeps v2/ and two former roots.
type threeRoots struct{ secrets.Locator }

func (threeRoots) FormerRoots(tenantID uuid.UUID) []secrets.Ref {
	return []secrets.Ref{secrets.DefaultLocator("test/").Root(tenantID), secrets.DefaultLocator("v1/").Root(tenantID)}
}

// A caller that gives up does not wait for the cleanup: with a tombstone and three roots on a store that
// hangs, a cancelled List returns at once (not after one cleanup deadline per phase or per root), and an
// uncancelled one waits for one cleanup deadline in all.
func TestACancelledCallerDoesNotWaitForTheCleanup(t *testing.T) {
	restore := secrets.SetCleanupTimeout(2 * time.Second)
	defer restore()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "STUCK", Value: "v"})
	require.NoError(t, err)
	_, err = e.db.Exec(e.ctx, `UPDATE automation.secrets SET deleted_at = now() WHERE id = $1`, s.ID)
	require.NoError(t, err)

	var mu sync.Mutex
	calls := 0
	m := secrets.NewManager(e.db, hangingStore{Writer: e.store, mu: &mu, calls: &calls}, threeRoots{Locator: secrets.DefaultLocator("v2/")})

	ctx, cancel := context.WithCancel(e.ctx)
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()
	started := time.Now()
	_, err = m.List(ctx, e.tenant)
	require.NoError(t, err, "the records were read before the caller gave up")
	assert.Less(t, time.Since(started), time.Second, "the cleanup ended with the caller")

	started = time.Now()
	_, err = m.List(e.ctx, e.tenant)
	require.NoError(t, err)
	assert.Less(t, time.Since(started), 3*time.Second, "one cleanup deadline for the tombstones and every root")
}

// putSpy hangs on Put until its context ends and records the context's deadline.
type putSpy struct {
	secrets.Writer
	deadline *time.Time
}

func (p putSpy) Put(ctx context.Context, _ secrets.Ref, _ map[string]string) (secrets.Metadata, error) {
	d, ok := ctx.Deadline()
	if ok {
		*p.deadline = d
	}
	<-ctx.Done()
	return secrets.Metadata{}, ctx.Err()
}

// The store's write is bounded by the write window: a Put that hangs is given up by the claim's fence,
// before the tombstone of a concurrent delete may be retired, and nothing is published.
func TestAStoreWriteIsBoundedByTheWriteWindow(t *testing.T) {
	restore := secrets.SetWriteWindow(time.Second)
	defer restore()
	e := newEnv(t)
	s, err := e.mgr.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "API_KEY", Value: "v1"})
	require.NoError(t, err)

	var deadline time.Time
	m := secrets.NewManager(e.db, putSpy{Writer: e.store, deadline: &deadline}, secrets.DefaultLocator("test/"))
	started := time.Now()
	_, err = m.Rotate(e.ctx, e.tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "v2"})
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	require.False(t, deadline.IsZero(), "the store's write has a deadline")
	// The claim is taken after the call's purge, so its fence is a little after started + window/2.
	assert.Less(t, deadline.Sub(started), secrets.WriteWindow()/2+200*time.Millisecond, "by the claim's fence")
	assert.Less(t, time.Since(started), secrets.WriteWindow(), "given up inside the window")

	_, err = m.Create(e.ctx, e.tenant, secrets.CreateInput{Name: "OTHER", Value: "v"})
	require.ErrorIs(t, err, secrets.ErrUnavailable, "a create's write too")
	got, err := e.res.Resolve(e.ctx, e.tenant, "API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "v1", got, "nothing published")
}
