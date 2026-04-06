package seed

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRegisterProviders_RegistersCoreRepositories(t *testing.T) {
	deps := &application.SeedDeps{Logger: logrus.New()}
	RegisterProviders(deps)

	err := deps.Invoke(context.Background(), func(
		ctx context.Context,
		tenantRepo tenant.Repository,
		currencyRepo currency.Repository,
		permissionRepo permission.Repository,
		uploadRepo upload.Repository,
		roleRepo role.Repository,
		userRepo user.Repository,
		groupRepo group.Repository,
	) error {
		require.NotNil(t, tenantRepo)
		require.NotNil(t, currencyRepo)
		require.NotNil(t, permissionRepo)
		require.NotNil(t, uploadRepo)
		require.NotNil(t, roleRepo)
		require.NotNil(t, userRepo)
		require.NotNil(t, groupRepo)
		return nil
	})
	require.NoError(t, err)
}
