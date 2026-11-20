package wiring_test

import (
	"context"
	"embed"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// recorder is a Go consumer of core.user.created.v1.
type recorder struct {
	mu   sync.Mutex
	seen map[string]int
	last events.Envelope
}

func (r *recorder) ID() string { return "test.wiring.userCreated" }

func (r *recorder) Events(context.Context) ([]string, error) {
	return []string{"core.user.created.v1"}, nil
}

func (r *recorder) AfterSeq(context.Context) (int64, error) { return 0, nil }

func (r *recorder) Deliver(_ context.Context, _ pgx.Tx, ev events.Envelope) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[ev.ID]++
	r.last = ev
	return nil
}

func (r *recorder) snapshot() (map[string]int, events.Envelope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.seen))
	for k, v := range r.seen {
		out[k] = v
	}
	return out, r.last
}

// consumerComponent is an application component registering a Go consumer.
type consumerComponent struct{ rec *recorder }

func (c consumerComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "test.consumers"}
}

func (c consumerComponent) LocaleFS() []*embed.FS { return nil }

func (c consumerComponent) Build(b *composition.Builder) error {
	wiring.ContributeConsumers(b, func(*composition.Container) ([]events.Consumer, error) {
		return []events.Consumer{c.rec}, nil
	})
	return nil
}

// Without EXTRUNTIME_ENABLED and without node_modules/@iota-uz/sdk the core
// still serves the outbox: creating a user writes the event, a Go consumer
// receives it once, and the row is pruned after the retention.
func TestCoreDeliversNativeEventsToGoConsumersWithoutTheRuntime(t *testing.T) {
	t.Setenv("IOTA_SDK_DIST", t.TempDir()) // no @iota-uz/sdk anywhere
	rec := &recorder{seen: map[string]int{}}
	const retention = 1500 * time.Millisecond
	section := extruntimeconfig.Config{AppRoot: t.TempDir(), Events: extruntimeconfig.EventsConfig{Retention: retention}}
	components := append(modules.Components(),
		wiring.NewComponent(wiring.Options{
			Natives: modules.Natives(), Section: &section,
			Events: events.Config{PollInterval: 50 * time.Millisecond, PruneInterval: 100 * time.Millisecond},
		}),
		consumerComponent{rec: rec},
	)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "extruntime_wiring_core",
		Components: components,
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := itf.TestExecution(composables.WithSession(composables.WithParams(scope.Ctx, itf.DefaultParams()), itf.MockSession()))

	// The core is composed: the native catalog, the subject loader, one
	// dispatcher, started by the core's own hook.
	set, err := composition.Resolve[*native.Set](scope.Container)
	require.NoError(t, err)
	_, ok := set.Event("core.user.created.v1")
	require.True(t, ok, "the native catalog is composed without the runtime")
	require.NotEmpty(t, set.Events())
	_, err = composition.Resolve[host.SubjectLoader](scope.Container)
	require.NoError(t, err)
	_, err = composition.Resolve[*events.Dispatcher](scope.Container)
	require.NoError(t, err)
	require.Equal(t, 1, events.Running(), "one dispatcher serves the process")

	// An administrator creates a user through the core service.
	perms := []permission.Permission{corePerms.UserCreate, corePerms.UserRead}
	var admin user.User
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		for _, p := range perms {
			if err := persistence.NewPermissionRepository().Save(txCtx, p); err != nil {
				return err
			}
		}
		var err error
		admin, err = persistence.NewUserRepository(persistence.NewUploadRepository()).Create(txCtx, user.New("Ada", "Admin",
			internet.MustParseEmail("ada@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID), user.WithPermissions(perms)))
		return err
	}))
	users, err := composition.Resolve[*services.UserService](scope.Container)
	require.NoError(t, err)
	created, err := users.Create(composables.WithUser(ctx, admin), user.New("Bob", "Builder",
		internet.MustParseEmail("bob@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID)))
	require.NoError(t, err)

	var eventID string
	require.NoError(t, scope.Pool.QueryRow(context.Background(),
		`SELECT id::text FROM extruntime.outbox_events WHERE event = 'core.user.created.v1' AND tenant_id = $1`, scope.Tenant.ID).Scan(&eventID))

	require.Eventually(t, func() bool { seen, _ := rec.snapshot(); return seen[eventID] == 1 }, 10*time.Second, 20*time.Millisecond,
		"the Go consumer receives the event")
	seen, last := rec.snapshot()
	require.Len(t, seen, 1)
	require.Equal(t, scope.Tenant.ID, last.TenantID)
	require.Equal(t, int64(admin.ID()), *last.InitiatorUserID)
	require.Equal(t, &spi.ObjectRef{Entity: "core.user", Key: strconv.FormatUint(uint64(created.ID()), 10)}, last.Subject)
	var payload struct {
		Email string `json:"email"`
	}
	require.NoError(t, json.Unmarshal(last.Payload, &payload))
	require.Equal(t, "bob@example.com", payload.Email)

	// After the retention the delivered event is pruned, and never delivered again.
	require.Eventually(t, func() bool {
		var n int
		require.NoError(t, scope.Pool.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.outbox_events WHERE id = $1`, eventID).Scan(&n))
		return n == 0
	}, 15*time.Second, 50*time.Millisecond, "the delivered event is pruned after the retention")
	seen, _ = rec.snapshot()
	require.Equal(t, 1, seen[eventID], "delivered exactly once")
}
