package secrets_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets/openbao"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The records over the OpenBao adapter, as the module composes them: a manager over a writer identity and
// a resolver over a reader identity. Runs against a real OpenBao when OPENBAO_TEST_ADDR and
// OPENBAO_TEST_TOKEN (a root token of a dev server) are set.
func TestSecretsOverOpenBao(t *testing.T) {
	t.Parallel()
	addr, root := os.Getenv("OPENBAO_TEST_ADDR"), os.Getenv("OPENBAO_TEST_TOKEN")
	if addr == "" || root == "" {
		t.Skip("OPENBAO_TEST_ADDR and OPENBAO_TEST_TOKEN are not set")
	}
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automation_secrets_openbao",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx, tenant := context.Background(), scope.Tenant.ID
	// The root token stands in for both identities here; the policies of the roles are the adapter's tests.
	bao := func(address string) *openbao.Client {
		c, err := openbao.New(openbao.Config{Address: address, Auth: openbao.TokenAuth(openbao.Literal(root))})
		require.NoError(t, err)
		return c
	}
	writer, reader := bao(addr), bao(addr)
	prefix := "rec" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10] + "/"
	mgr := secrets.NewManager(scope.Pool, writer.Writer(), secrets.DefaultLocator(prefix))
	res := secrets.NewResolver(scope.Pool, reader.Reader())

	s, err := mgr.Create(ctx, tenant, secrets.CreateInput{Name: "CRM_TOKEN", Value: "crm-v1", CreatedBy: 1})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(s.Ref.Path, prefix+"tenants/"+tenant.String()+"/"), s.Ref.Path)
	got, err := res.Resolve(ctx, tenant, "CRM_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "crm-v1", got)

	// A rotation applies to the running resolver: no restart, no cache.
	s, err = mgr.Rotate(ctx, tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "crm-v2"})
	require.NoError(t, err)
	assert.Equal(t, 2, s.ValueVersion)
	got, err = res.Resolve(ctx, tenant, "CRM_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "crm-v2", got)

	// The store unavailable: a controlled ErrUnavailable that quotes no value, never the old one.
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	_, err = secrets.NewResolver(scope.Pool, bao(down.URL).Reader()).Resolve(ctx, tenant, "CRM_TOKEN")
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	assert.NotContains(t, err.Error(), "crm-v")
	_, err = secrets.NewManager(scope.Pool, bao(down.URL).Writer(), secrets.DefaultLocator(prefix)).
		Rotate(ctx, tenant, secrets.RotateInput{ID: s.ID, Version: s.Version, Value: "crm-v3"})
	require.ErrorIs(t, err, secrets.ErrUnavailable)
	assert.NotContains(t, err.Error(), "crm-v3")
	got, err = res.Resolve(ctx, tenant, "CRM_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "crm-v2", got, "a failed rotation leaves readers on the published version")

	// Another tenant's resolver does not see it.
	other := uuid.New()
	_, err = scope.Pool.Exec(ctx, `INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		other, "t-"+other.String()[:8], other.String()[:8]+".test")
	require.NoError(t, err)
	_, err = res.Resolve(ctx, other, "CRM_TOKEN")
	require.ErrorIs(t, err, secrets.ErrNotFound)

	// Deleted: the value goes from OpenBao with its versions.
	require.NoError(t, mgr.Delete(ctx, tenant, s.ID))
	paths, err := writer.Writer().List(ctx, secrets.DefaultLocator(prefix).Root(tenant))
	require.NoError(t, err)
	assert.Empty(t, paths, fmt.Sprint(paths))
	_, err = reader.Reader().Read(ctx, s.Ref)
	require.ErrorIs(t, err, secrets.ErrNotFound)
}
