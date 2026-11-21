package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage/managetest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool/pooltest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// With the runtime enabled the process keeps one engine: the runtime attaches
// its package executor to the core's engine instead of building its own, and
// a native workflow started through the runtime's host runs on it.
func TestRuntimeAttachesToTheCoreEngine(t *testing.T) {
	natives, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_runtime_engine",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	done := workflow.NativeRunnerFunc(func(context.Context, workflow.NativeRun) (json.RawMessage, error) {
		return json.RawMessage(`"done"`), nil
	})
	eng, err := workflow.New(workflow.Config{
		DB: scope.Pool, Subjects: hosttest.NewSubjects(),
		Natives: map[string]workflow.NativeContract{"test.rt.native": {Runner: done, Signals: map[string]manifest.Type{}}},
	})
	require.NoError(t, err)
	rt, err := runtime.New(runtime.Config{
		Natives: natives, WorkerScript: pooltest.BuildWorker(t), CacheDir: t.TempDir(), PoolSize: 1,
		Builder: &managetest.Builder{}, Kits: &managetest.Kits{}, DisableBackground: true,
		Workflow: runtime.WorkflowConfig{Concurrency: 3},
	}, runtime.Deps{DB: scope.Pool, Workflows: eng})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	require.Same(t, eng, rt.Workflows, "the runtime builds no second engine")
	require.Equal(t, 3, eng.PackageConcurrency(), "the runtime's workflow concurrency reaches the shared engine")
	require.NoError(t, eng.Ready(context.Background()))

	ctx := context.Background()
	tx, err := scope.Pool.Begin(ctx)
	require.NoError(t, err)
	raw, err := eng.Start(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: scope.Tenant.ID, UserID: 1}}, spi.StartIntent{Workflow: "test.rt.native", Input: json.RawMessage(`{}`), Key: "rt-1"})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	id := uuid.MustParse(raw)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := eng.RunOnce(ctx)
		assert.NoError(c, err)
		hist, err := eng.Get(ctx, scope.Tenant.ID, id)
		if assert.NoError(c, err) {
			assert.Equal(c, workflow.StatusCompleted, hist.Instance.Status)
		}
	}, 10*time.Second, 50*time.Millisecond)
}
