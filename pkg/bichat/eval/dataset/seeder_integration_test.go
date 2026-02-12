package dataset_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval/dataset"
	"github.com/stretchr/testify/require"
)

func TestSeeder_SeedsAndBuildsOracle(t *testing.T) {
	t.Parallel()

	dsn := os.Getenv("BICHAT_EVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("BICHAT_EVAL_TEST_DSN is not set")
	}

	ctx := context.Background()
	seeder, err := dataset.NewSeeder(ctx, dsn, dataset.DefaultRegistry())
	require.NoError(t, err)
	defer seeder.Close()

	tenantID := uuid.New()
	facts, err := seeder.SeedAndBuildOracle(ctx, dataset.AnalyticsBaselineV1ID, tenantID)
	require.NoError(t, err)
	require.NotEmpty(t, facts)
	require.Contains(t, facts, "analytics_baseline_v1.q1_total_income_minor")
	require.Contains(t, facts, "analytics_baseline_v1.q1_net_cashflow_minor")
}
