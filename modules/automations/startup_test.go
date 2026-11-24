package automations_test

import (
	"context"
	"embed"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// schemaProbe starts after the other components and reads the tables their start hooks read.
type schemaProbe struct{ tables []string }

func (schemaProbe) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "automations.schema_probe"}
}
func (schemaProbe) LocaleFS() []*embed.FS { return nil }

func (p schemaProbe) Build(b *composition.Builder) error {
	composition.ContributeHooks(b, func(c *composition.Container) ([]composition.Hook, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		return []composition.Hook{{Name: "automations.schema_probe", Start: func(ctx context.Context) (composition.StopFn, error) {
			for _, table := range p.tables {
				var n int
				if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
					return nil, err
				}
			}
			return noStop, nil
		}}}, nil
	})
	return nil
}

// A composition with the automations module and the extension runtime disabled (the default of
// wiring.NewComponent, as in cmd/server) starts its hooks on a migrated schema: the engine, the
// event core and the workflow engine read their tables when they start, and the start must not
// run ahead of the migrations (the failure of pkg/extruntime/e2e after the engine was wired in).
func TestStartsOnMigratedSchemaWithExtruntimeDisabled(t *testing.T) {
	t.Parallel()
	components := append(modules.Components(),
		wiring.NewComponent(wiring.Options{Natives: modules.Natives(), Section: &extruntimeconfig.Config{}}),
		schemaProbe{tables: []string{
			"automation.triggers", "automation.runs", "automation.subject_queue", "automation.waits",
			"automation.steps", "automation.inbound_collect", "extruntime.activations", "business_event_deliveries",
		}},
	)
	h := itf.OpenHarness(t, itf.HarnessConfig{
		Name:       "automations_start_test",
		Components: components,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
	})
	require.NoError(t, h.Close())
}

func noStop(context.Context) error { return nil }
