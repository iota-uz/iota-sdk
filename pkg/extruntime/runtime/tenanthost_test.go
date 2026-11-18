package runtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestTenantByHost(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_tenant_host",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase, Pool: itf.PoolConfig{MaxConns: 4}},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := context.Background()
	active, inactive, shared1, shared2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		id     uuid.UUID
		name   string
		domain string
		active bool
	}{
		{active, "Acme", "Acme.example.org", true},
		{inactive, "Dormant", "dormant.example.org", false},
		{shared1, "Twin one", "twin.example.org", true},
		{shared2, "Twin two", "twin.example.org", true},
	} {
		_, err := scope.Pool.Exec(ctx, `INSERT INTO tenants (id, name, domain, is_active) VALUES ($1, $2, $3, $4)`, row.id, row.name, row.domain, row.active)
		require.NoError(t, err)
	}

	resolve := runtime.TenantByHost(scope.Pool)
	at := func(host string) (uuid.UUID, bool) {
		r := httptest.NewRequest(http.MethodPost, "/_ext/endpoints/acme.pay/hook", nil)
		r.Host = host
		return resolve(r)
	}

	id, ok := at("acme.example.org")
	require.True(t, ok)
	require.Equal(t, active, id, "the host name is matched case-insensitively")
	id, ok = at("Acme.Example.Org:8443")
	require.True(t, ok, "the port is ignored")
	require.Equal(t, active, id)
	id, ok = at("acme.example.org.")
	require.True(t, ok, "the root dot of a fully qualified name is ignored")
	require.Equal(t, active, id)
	_, ok = at("dormant.example.org")
	require.False(t, ok, "an inactive tenant is not served")
	_, ok = at("twin.example.org")
	require.False(t, ok, "a host shared by two tenants is ambiguous, so it names none")
	_, ok = at("unknown.example.org")
	require.False(t, ok)
	_, ok = at("")
	require.False(t, ok)
}
