package persistence_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func requirePostgres(t *testing.T) {
	t.Helper()

	conf := configuration.Use()
	addr := net.JoinHostPort(conf.Database.Host, conf.Database.Port)
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Skipf("postgres not available at %s: %v", addr, err)
		return
	}
	_ = conn.Close()
}

// setupTest creates all necessary dependencies for tests including a database user
func setupTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	requirePostgres(t)

	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	// Create role first (required for user)
	roleRepo := persistence.NewRoleRepository()
	testRole := role.New(
		"test-role",
		role.WithTenantID(env.Tenant.ID),
	)
	createdRole, err := roleRepo.Create(env.Ctx, testRole)
	require.NoError(t, err, "failed to create test role")

	// Create user with the role using proper repository
	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)

	testUser := itf.User()
	newUser := user.New(
		testUser.FirstName(),
		testUser.LastName(),
		testUser.Email(),
		testUser.UILanguage(),
		user.WithTenantID(env.Tenant.ID),
	).AddRole(createdRole)

	createdUser, err := userRepo.Create(env.Ctx, newUser)
	require.NoError(t, err, "failed to create test user")

	// Update environment with the created user
	env.User = createdUser
	env.Ctx = composables.WithUser(env.Ctx, createdUser)

	return env
}
