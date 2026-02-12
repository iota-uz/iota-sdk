package dataset_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/eval/dataset"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistry_ContainsAnalyticsBaseline(t *testing.T) {
	t.Parallel()

	registry := dataset.DefaultRegistry()
	ds, err := registry.Get(dataset.AnalyticsBaselineV1ID)
	require.NoError(t, err)
	require.Equal(t, dataset.AnalyticsBaselineV1ID, ds.ID())
}

func TestToHarnessOracleFacts(t *testing.T) {
	t.Parallel()

	facts := map[string]dataset.Fact{
		"k1": {
			Key:           "k1",
			Description:   "desc",
			ExpectedValue: "42",
			ValueType:     "number",
			Tolerance:     0.1,
			Normalization: "currency_minor_units",
		},
	}

	out := dataset.ToHarnessOracleFacts(facts)
	require.Len(t, out, 1)
	require.Equal(t, "42", out["k1"].ExpectedValue)
	require.InEpsilon(t, 0.1, out["k1"].Tolerance, 1e-9)
}

type testDataset struct {
	id string
}

func (d testDataset) ID() string { return d.id }

func (d testDataset) Seed(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error { return nil }

func (d testDataset) BuildOracle(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]dataset.Fact, error) {
	return map[string]dataset.Fact{}, nil
}

func TestRegistry_RegisterAndIDs(t *testing.T) {
	t.Parallel()

	reg, err := dataset.NewRegistry()
	require.NoError(t, err)
	require.NoError(t, reg.Register(testDataset{id: "ext_dataset_a"}, testDataset{id: "ext_dataset_b"}))

	ids := reg.IDs()
	require.Equal(t, []string{"ext_dataset_a", "ext_dataset_b"}, ids)

	ds, err := reg.Get("ext_dataset_a")
	require.NoError(t, err)
	require.Equal(t, "ext_dataset_a", ds.ID())
}

func TestRegistry_RegisterRejectsDuplicate(t *testing.T) {
	t.Parallel()

	reg, err := dataset.NewRegistry(testDataset{id: "ext_dataset_a"})
	require.NoError(t, err)

	err = reg.Register(testDataset{id: "ext_dataset_a"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestNewRegistry_RejectsNilDataset(t *testing.T) {
	t.Parallel()

	_, err := dataset.NewRegistry(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dataset is nil")
}
