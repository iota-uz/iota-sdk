package persistence_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	return itf.Setup(t, itf.WithComponents(modules.Components()...))
}

func createOIDCTestTenantAndUsers(t *testing.T, env *itf.TestEnvironment, userIDs ...int) uuid.UUID {
	t.Helper()

	tx, err := composables.UseTx(env.Ctx)
	require.NoError(t, err)

	tenantID := uuid.New()
	tenantName := fmt.Sprintf("oidc-test-tenant-%s", tenantID.String())

	_, err = tx.Exec(
		env.Ctx,
		`INSERT INTO tenants (id, name) VALUES ($1, $2)`,
		tenantID,
		tenantName,
	)
	require.NoError(t, err)

	for _, userID := range userIDs {
		email := fmt.Sprintf("oidc-test-user-%d-%s@example.com", userID, tenantID.String()[:8])
		_, err = tx.Exec(
			env.Ctx,
			`INSERT INTO users (id, tenant_id, type, first_name, last_name, email, ui_language)
			 VALUES ($1, $2, 'user', 'OIDC', 'Test', $3, 'en')
			 ON CONFLICT (id) DO NOTHING`,
			userID,
			tenantID,
			email,
		)
		require.NoError(t, err)
	}

	return tenantID
}

func createOIDCTestClients(t *testing.T, env *itf.TestEnvironment, clientIDs ...string) {
	t.Helper()

	tx, err := composables.UseTx(env.Ctx)
	require.NoError(t, err)

	for _, clientID := range clientIDs {
		_, err = tx.Exec(
			env.Ctx,
			`INSERT INTO oidc.clients (
				client_id, name, application_type, redirect_uris, grant_types, response_types, scopes, auth_method
			) VALUES (
				$1, $2, 'web',
				ARRAY['http://localhost:3000/callback'],
				ARRAY['authorization_code'],
				ARRAY['code'],
				ARRAY['openid', 'profile', 'email'],
				'client_secret_basic'
			) ON CONFLICT (client_id) DO NOTHING`,
			clientID,
			"Test "+clientID,
		)
		require.NoError(t, err)
	}
}
