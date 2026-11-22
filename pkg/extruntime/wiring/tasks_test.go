package wiring_test

import (
	"context"
	"embed"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const reviewFlow = "test.wiring.review"

type observed struct {
	mu    sync.Mutex
	kinds []string
}

func (o *observed) TaskChanged(_ context.Context, _ pgx.Tx, _ workflow.Task, ch workflow.TaskChange) error {
	o.mu.Lock()
	o.kinds = append(o.kinds, ch.Kind)
	o.mu.Unlock()
	return nil
}

func (o *observed) seen() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.kinds...)
}

// taskComponent registers a native workflow that opens a group task, and a
// task observer.
type taskComponent struct {
	group *uuid.UUID
	obs   *observed
}

func (c taskComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "test.tasks"}
}

func (c taskComponent) LocaleFS() []*embed.FS { return nil }

func (c taskComponent) Build(b *composition.Builder) error {
	wiring.ContributeNativeWorkflows(b, func(*composition.Container) (map[string]workflow.NativeContract, error) {
		runner := workflow.NativeRunnerFunc(func(ctx context.Context, run workflow.NativeRun) (json.RawMessage, error) {
			r, err := run.Task(ctx, "review", 0, workflow.TaskSpec{
				Assignees: []workflow.TaskAssignee{{Kind: workflow.TaskAssignGroup, Ref: c.group.String()}},
			})
			if err != nil {
				return nil, err
			}
			return json.Marshal(r)
		})
		return map[string]workflow.NativeContract{reviewFlow: {Runner: runner, Concurrency: 1}}, nil
	})
	wiring.ContributeTaskObservers(b, func(*composition.Container) ([]workflow.TaskObserver, error) {
		return []workflow.TaskObserver{c.obs}, nil
	})
	return nil
}

// The core's engine judges group assignees with the container's
// TaskAudience (the core's groups) and calls the contributed observers.
func TestCoreTasksUseTheCoreAudienceAndContributedObservers(t *testing.T) {
	t.Setenv("IOTA_SDK_DIST", t.TempDir())
	group := uuid.New()
	obs := &observed{}
	section := extruntimeconfig.Config{AppRoot: t.TempDir()}
	components := append(modules.Components(),
		wiring.NewComponent(wiring.Options{Natives: modules.Natives(), Section: &section}),
		taskComponent{group: &group, obs: obs},
	)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "extruntime_wiring_tasks",
		Components: components,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := itf.TestExecution(composables.WithSession(composables.WithParams(scope.Ctx, itf.DefaultParams()), itf.MockSession()))
	bg := context.Background()

	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	var member, outsider user.User
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		if member, err = users.Create(txCtx, user.New("Mem", "Ber", internet.MustParseEmail("member@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID))); err != nil {
			return err
		}
		outsider, err = users.Create(txCtx, user.New("Out", "Sider", internet.MustParseEmail("outsider@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID)))
		return err
	}))
	_, err := scope.Pool.Exec(bg, `INSERT INTO user_groups (id, name, type, tenant_id) VALUES ($1, 'reviewers', 'user', $2)`, group, scope.Tenant.ID)
	require.NoError(t, err)
	_, err = scope.Pool.Exec(bg, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, group, int64(member.ID()))
	require.NoError(t, err)

	eng, err := composition.Resolve[*workflow.Engine](scope.Container)
	require.NoError(t, err)
	tx, err := scope.Pool.Begin(bg)
	require.NoError(t, err)
	starter := spi.Subject{TenantID: scope.Tenant.ID, UserID: int64(member.ID()), Kind: spi.SubjectInitiator}
	raw, err := eng.Start(bg, tx, spi.Scope{Subject: starter}, spi.StartIntent{Workflow: reviewFlow, Input: json.RawMessage(`{}`), Key: "r1"})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(bg))
	id := uuid.MustParse(raw)
	reaches := func(want string) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			h, err := eng.Get(bg, scope.Tenant.ID, id)
			if assert.NoError(c, err) {
				assert.Equal(c, want, h.Instance.Status)
			}
		}, 15*time.Second, 50*time.Millisecond)
	}
	reaches(workflow.StatusWaiting)

	subject := func(u user.User) spi.Subject {
		return spi.Subject{TenantID: scope.Tenant.ID, UserID: int64(u.ID()), Kind: spi.SubjectInitiator}
	}
	inbox, err := eng.Tasks(bg, subject(member), workflow.TaskFilter{})
	require.NoError(t, err)
	require.Len(t, inbox, 1, "the group member sees the task")
	inbox, err = eng.Tasks(bg, subject(outsider), workflow.TaskFilter{})
	require.NoError(t, err)
	require.Empty(t, inbox)
	require.ErrorIs(t, eng.DecideTask(bg, subject(outsider), inbox0(t, eng, subject(member)), json.RawMessage(`{}`)), workflow.ErrForbidden)
	require.NoError(t, eng.DecideTask(bg, subject(member), inbox0(t, eng, subject(member)), json.RawMessage(`{"ok":true}`)))
	reaches(workflow.StatusCompleted)
	require.Equal(t, []string{"created", "decision", "closed"}, obs.seen())
}

func inbox0(t *testing.T, eng *workflow.Engine, s spi.Subject) uuid.UUID {
	t.Helper()
	tasks, err := eng.Tasks(context.Background(), s, workflow.TaskFilter{Status: []string{"open"}})
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	return tasks[0].ID
}
