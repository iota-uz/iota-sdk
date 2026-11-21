package wiring_test

import (
	"context"
	"embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const greetFlow = "test.wiring.greet"

// greeter waits for a "hello" signal and completes with its payload.
var greeter = workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
	w, err := run.WaitFor(ctx, "hello", 0, "hello", 0)
	if err != nil {
		return nil, err
	}
	return w.Payload, nil
})

// workflowComponent registers a native workflow contract.
type workflowComponent struct{ ids []string }

func (c workflowComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "test.workflows"}
}

func (c workflowComponent) LocaleFS() []*embed.FS { return nil }

func (c workflowComponent) Build(b *composition.Builder) error {
	for _, id := range c.ids {
		wiring.ContributeNativeWorkflows(b, func(*composition.Container) (map[string]workflow.NativeContract, error) {
			return map[string]workflow.NativeContract{id: {
				Runner: greeter, Signals: map[string]manifest.Type{"hello": {Kind: manifest.KindJSON}}, Concurrency: 1,
			}}, nil
		})
	}
	return nil
}

func coreHarness(t *testing.T, name string, ids ...string) *itf.Scope {
	t.Helper()
	t.Setenv("IOTA_SDK_DIST", t.TempDir()) // no @iota-uz/sdk anywhere
	section := extruntimeconfig.Config{AppRoot: t.TempDir(), WorkflowConcurrency: 7}
	components := append(modules.Components(),
		wiring.NewComponent(wiring.Options{Natives: modules.Natives(), Section: &section}),
		workflowComponent{ids: ids},
	)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       name,
		Components: components,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	return h.Scope(t)
}

// Without EXTRUNTIME_ENABLED and without the @iota-uz/sdk distribution a
// native workflow contract starts, waits for a signal and completes: the core
// builds and serves the engine without the Node pool.
func TestCoreRunsNativeWorkflowsWithoutTheRuntime(t *testing.T) {
	scope := coreHarness(t, "extruntime_wiring_workflows", greetFlow)
	ctx := itf.TestExecution(composables.WithSession(composables.WithParams(scope.Ctx, itf.DefaultParams()), itf.MockSession()))

	var u user.User
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		u, err = persistence.NewUserRepository(persistence.NewUploadRepository()).Create(txCtx, user.New("Wf", "User",
			internet.MustParseEmail("wf@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID)))
		return err
	}))

	eng, err := composition.Resolve[*workflow.Engine](scope.Container)
	require.NoError(t, err)
	_, ok := eng.Native(greetFlow)
	require.True(t, ok)
	require.Equal(t, 7, eng.PackageConcurrency(), "EXTRUNTIME_WORKFLOWCONCURRENCY sizes the package workers")

	bg := context.Background()
	tx, err := scope.Pool.Begin(bg)
	require.NoError(t, err)
	subject := spi.Subject{TenantID: scope.Tenant.ID, UserID: int64(u.ID()), Kind: spi.SubjectInitiator}
	raw, err := eng.Start(bg, tx, spi.Scope{Subject: subject}, spi.StartIntent{Workflow: greetFlow, Input: json.RawMessage(`{}`), Key: "greet-1"})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(bg))
	id := uuid.MustParse(raw)

	status := func() workflow.Instance {
		h, err := eng.Get(bg, scope.Tenant.ID, id)
		require.NoError(t, err)
		return h.Instance
	}
	reaches := func(want, msg string) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			h, err := eng.Get(bg, scope.Tenant.ID, id)
			if assert.NoError(c, err) {
				assert.Equal(c, want, h.Instance.Status)
			}
		}, 15*time.Second, 50*time.Millisecond, msg)
	}
	reaches(workflow.StatusWaiting, "the served engine runs the instance")
	require.NoError(t, eng.Signal(bg, scope.Tenant.ID, int64(u.ID()), id, "hello", json.RawMessage(`{"name":"core"}`), ""))
	reaches(workflow.StatusCompleted, "the signal completes it")
	inst := status()
	require.True(t, inst.Native)
	require.JSONEq(t, `{"name":"core"}`, string(inst.Result))
}
