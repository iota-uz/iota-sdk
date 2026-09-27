package seed

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRegisterProviders_RegistersWebsiteRepositories(t *testing.T) {
	deps := &application.SeedDeps{Logger: logrus.New()}
	RegisterProviders(deps)

	err := deps.Invoke(context.Background(), func(ctx context.Context, repo aichatconfig.Repository) error {
		require.NotNil(t, repo)
		return nil
	})
	require.NoError(t, err)
}
