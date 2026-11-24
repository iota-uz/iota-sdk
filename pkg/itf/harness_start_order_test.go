package itf

import (
	"context"
	"embed"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// tableProbe is a component whose start hook reads a table, as the extension
// runtime's workflow engine and the automation engine's passes do.
type tableProbe struct {
	table string
	read  *bool
}

func (tableProbe) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "table_probe"}
}
func (tableProbe) LocaleFS() []*embed.FS { return nil }

func (p tableProbe) Build(b *composition.Builder) error {
	composition.ContributeHooks(b, func(c *composition.Container) ([]composition.Hook, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{{Name: "table_probe", Start: func(ctx context.Context) (composition.StopFn, error) {
			var n int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+p.table).Scan(&n); err != nil {
				return nil, err
			}
			*p.read = true
			return noStop, nil
		}}}, nil
	})
	return nil
}

// The harness applies its migration policy before the start hooks run: a hook
// that reads a migrated table at start finds it, as it does in a deployment, where
// the schema is migrated before the process starts. Falsely green if the hook is skipped, so the test
// also checks that it ran.
func TestHarnessMigratesBeforeStartHooks(t *testing.T) {
	t.Parallel()
	var read bool
	h := OpenHarness(t, HarnessConfig{
		Name:       "start_order_test",
		Components: []composition.Component{tableProbe{table: "tenants", read: &read}},
		Database:   DatabaseConfig{Provisioning: ProvisioningPerTestDatabase},
	})
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	require.True(t, read, "the start hook did not run")
}

func noStop(context.Context) error { return nil }
