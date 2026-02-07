package controllers

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
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

func setupControllerTest(t *testing.T) *itf.TestEnvironment {
	t.Helper()

	requirePostgres(t)

	return itf.Setup(
		t,
		itf.WithModules(modules.BuiltInModules...),
	)
}

func createCoreUser(t *testing.T, env *itf.TestEnvironment, email string) coreuser.User {
	t.Helper()

	roleRepo := corepersistence.NewRoleRepository()
	testRole := role.New(
		"bichat-controllers-test-"+uuid.NewString()[:8],
		role.WithTenantID(env.Tenant.ID),
	)
	createdRole, err := roleRepo.Create(env.Ctx, testRole)
	require.NoError(t, err, "failed to create test role")

	uploadRepo := corepersistence.NewUploadRepository()
	userRepo := corepersistence.NewUserRepository(uploadRepo)

	emailVO, err := internet.NewEmail(email)
	require.NoError(t, err, "failed to create email value object")

	u := coreuser.New(
		"Test",
		"User",
		emailVO,
		coreuser.UILanguageEN,
		coreuser.WithTenantID(env.Tenant.ID),
	).AddRole(createdRole)

	createdUser, err := userRepo.Create(env.Ctx, u)
	require.NoError(t, err, "failed to create test user")

	env.User = createdUser
	env.Ctx = composables.WithUser(env.Ctx, createdUser)

	return createdUser
}
