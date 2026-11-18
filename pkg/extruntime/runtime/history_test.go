package runtime_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

func sessionUser(t *testing.T, id uint) user.User {
	t.Helper()
	email, err := internet.NewEmail("u@example.com")
	require.NoError(t, err)
	return user.New("Ann", "Lee", email, user.UILanguageEN, user.WithID(id))
}

func TestHistoryExecutionDerivesTheContextOfBackgroundRoots(t *testing.T) {
	tenant := uuid.New()
	hook := runtime.HistoryExecution(nil)
	inv := func(kind host.TriggerKind, user int64) host.NativeInvocation {
		return host.NativeInvocation{
			Subject: spi.Subject{TenantID: tenant, UserID: user, Kind: spi.SubjectOwner},
			Trigger: host.Trigger{Kind: kind, Ref: "pkg.trigger"}, InvocationID: "inv-1", RootID: "root-1",
		}
	}

	t.Run("a subscription acts for the subject as the runtime", func(t *testing.T) {
		ctx := hook(context.Background(), inv(host.TriggerSubscription, 7))
		exec, ok := history.UseExecution(ctx)
		require.True(t, ok, "a root without an HTTP request gets an execution")
		require.Equal(t, businessevent.ChannelJob, exec.Channel)
		require.Equal(t, "inv-1", exec.OperationID)
		require.Equal(t, "root-1", exec.RunID)
		require.NotNil(t, exec.Actor)
		require.Equal(t, businessevent.ActorService, exec.Actor.Kind)
		require.Equal(t, "extruntime:subscription", exec.Actor.ID)
		require.NoError(t, exec.Actor.Validate())
		require.NotNil(t, exec.Initiator)
		require.Equal(t, "7", exec.Initiator.UserID)
		require.NoError(t, exec.Initiator.Validate())
	})

	t.Run("the channel follows the trigger", func(t *testing.T) {
		for kind, want := range map[host.TriggerKind]businessevent.Channel{
			host.TriggerEndpoint: businessevent.ChannelAPI, host.TriggerSchedule: businessevent.ChannelJob,
			host.TriggerWorkflow: businessevent.ChannelJob, host.TriggerMCP: businessevent.ChannelMCP,
			host.TriggerUI: businessevent.ChannelUI,
		} {
			exec, _ := history.UseExecution(hook(context.Background(), inv(kind, 7)))
			require.Equal(t, want, exec.Channel, kind)
		}
	})

	t.Run("a request of the acting user keeps its execution", func(t *testing.T) {
		own := history.Execution{Channel: businessevent.ChannelUI, OperationID: "http-op", RequestID: "req-1"}
		ctx := history.WithExecution(composables.WithUser(context.Background(), sessionUser(t, 7)), own)
		exec, _ := history.UseExecution(hook(ctx, inv(host.TriggerUI, 7)))
		require.Equal(t, own, exec)
	})

	t.Run("a request acting as someone else does not attribute the work to the session user", func(t *testing.T) {
		ctx := history.WithExecution(composables.WithUser(context.Background(), sessionUser(t, 9)),
			history.Execution{Channel: businessevent.ChannelUI, OperationID: "http-op", RequestID: "req-1"})
		exec, _ := history.UseExecution(hook(ctx, inv(host.TriggerEndpoint, 7)))
		require.NotNil(t, exec.Actor)
		require.Equal(t, "7", exec.Initiator.UserID)
		require.Equal(t, "req-1", exec.RequestID, "the request correlation stays")
		require.Equal(t, "inv-1", exec.OperationID)
	})

	t.Run("a session user without an execution acts itself", func(t *testing.T) {
		ctx := composables.WithUser(context.Background(), sessionUser(t, 7))
		exec, ok := history.UseExecution(hook(ctx, inv(host.TriggerUI, 7)))
		require.True(t, ok)
		require.Nil(t, exec.Actor, "the user is the actor")
	})
}
