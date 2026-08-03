package serve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExecutionSessionStartsIdlePrefetchOnlyAfterUsefulForeground(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	foregroundStarted := make(chan struct{})
	releaseForeground := make(chan struct{})
	backgroundStarted := make(chan struct{})

	foreground := session.submit(t.Context(), "foreground", priorityRootBase, 0, func(context.Context) (any, error) {
		close(foregroundStarted)
		<-releaseForeground
		return "foreground", nil
	})
	<-foregroundStarted
	background := session.submit(t.Context(), "background", priorityIdlePrefetch, 0, func(context.Context) (any, error) {
		close(backgroundStarted)
		return "background", nil
	})

	select {
	case <-backgroundStarted:
		t.Fatal("idle prefetch started before foreground produced a useful result")
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseForeground)
	require.Equal(t, "foreground", (<-foreground.result).value)
	select {
	case <-backgroundStarted:
		t.Fatal("idle prefetch started before the useful foreground wave was revealed")
	case <-time.After(30 * time.Millisecond):
	}
	session.enableBackground()
	select {
	case <-backgroundStarted:
	case <-time.After(time.Second):
		t.Fatal("idle prefetch did not start after foreground completion")
	}
	require.Equal(t, "background", (<-background.result).value)
}

func TestExecutionSessionPromotesQueuedPrefetchOnInteractiveActivation(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	release := make(chan struct{})
	started := make(chan string, 2)
	for _, key := range []string{"root-a", "root-b"} {
		key := key
		session.submit(t.Context(), key, priorityRootBase, 0, func(context.Context) (any, error) {
			started <- key
			<-release
			return key, nil
		})
	}
	<-started
	<-started

	promotedStarted := make(chan struct{})
	prefetch := session.submit(t.Context(), "child", priorityIdlePrefetch, 5, func(context.Context) (any, error) {
		close(promotedStarted)
		return "child", nil
	})
	interactive := session.submit(t.Context(), "child", priorityInteractive, 0, func(context.Context) (any, error) {
		return "replacement", nil
	})
	close(release)

	select {
	case <-promotedStarted:
	case <-time.After(time.Second):
		t.Fatal("interactive activation did not promote the queued child")
	}
	require.Equal(t, "child", (<-prefetch.result).value)
	require.Equal(t, "child", (<-interactive.result).value)
}

func TestExecutionSessionCancelsSpeculationAfterItsLastConsumerDetaches(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	call := session.submit(t.Context(), "intent", priorityIntent, 0, func(ctx context.Context) (any, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	})
	session.enableBackground()
	<-started
	call.Cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("detached intent work was not cancelled")
	}
}

func TestExecutionSessionKeepsPromotedWorkForInteractiveConsumer(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	started := make(chan struct{})
	release := make(chan struct{})
	run := func(ctx context.Context) (any, error) {
		close(started)
		select {
		case <-release:
			return "child", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	prefetch := session.submit(t.Context(), "child", priorityIntent, 0, run)
	session.enableBackground()
	<-started
	interactive := session.submit(t.Context(), "child", priorityInteractive, 0, run)
	prefetch.Cancel()
	close(release)
	require.Equal(t, "child", (<-interactive.result).value)
}

func TestExecutionSessionReleaseDoesNotCancelRunningWorkPromotedToInteractive(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	started := make(chan struct{})
	release := make(chan struct{})
	run := func(ctx context.Context) (any, error) {
		close(started)
		select {
		case <-release:
			return "child", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	prefetch := session.submit(t.Context(), "child", priorityIdlePrefetch, 4, run)
	session.enableBackground()
	<-started
	interactive := session.submit(t.Context(), "child", priorityInteractive, 0, run)
	prefetch.Cancel()
	session.cancelBackground()
	close(release)
	require.Equal(t, "child", (<-interactive.result).value)
}

func TestExecutionSessionRevisionCancelsOnlyStaleNavigationWork(t *testing.T) {
	session := newExecutionSession(3, time.Second)
	staleStarted := make(chan struct{})
	idleRelease := make(chan struct{})
	idle := session.submit(t.Context(), "idle", priorityIdlePrefetch, 0, func(ctx context.Context) (any, error) {
		select {
		case <-idleRelease:
			return "idle", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	stale := session.submit(t.Context(), "path-a", priorityInteractive, 0, func(ctx context.Context) (any, error) {
		close(staleStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}, 1)
	<-staleStarted
	session.advanceRevision(2)
	require.ErrorIs(t, (<-stale.result).err, context.Canceled)
	close(idleRelease)
	session.enableBackground()
	require.Equal(t, "idle", (<-idle.result).value, "parent idle warm-up must survive a sibling navigation change")
}

func TestExecutionSessionReleaseCancelsOnlySpeculativeWork(t *testing.T) {
	session := newExecutionSession(3, time.Second)
	foregroundRelease := make(chan struct{})
	foreground := session.submit(t.Context(), "root", priorityRootBase, 0, func(ctx context.Context) (any, error) {
		select {
		case <-foregroundRelease:
			return "root", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	backgroundStarted := make(chan struct{})
	backgroundCancelled := make(chan struct{})
	background := session.submit(t.Context(), "child", priorityIdlePrefetch, 0, func(ctx context.Context) (any, error) {
		close(backgroundStarted)
		<-ctx.Done()
		close(backgroundCancelled)
		return nil, ctx.Err()
	})
	session.enableBackground()
	<-backgroundStarted

	session.cancelBackground()
	select {
	case <-backgroundCancelled:
	case <-time.After(time.Second):
		t.Fatal("snapshot release did not cancel running speculative work")
	}
	require.ErrorIs(t, (<-background.result).err, context.Canceled)
	close(foregroundRelease)
	require.Equal(t, "root", (<-foreground.result).value, "snapshot release must not cancel foreground work")
}

func TestAgedPriorityAdvancesWithinItsClassWithoutCrossingForegroundBoundary(t *testing.T) {
	now := time.Now()
	root := &scheduledJob{priority: priorityRootBase + 9, queuedAt: now.Add(-20 * priorityAgingStep)}
	intent := &scheduledJob{priority: priorityIntent + 9, queuedAt: now.Add(-20 * priorityAgingStep)}
	idle := &scheduledJob{priority: priorityIdlePrefetch + 9, queuedAt: now.Add(-20 * priorityAgingStep)}

	require.Equal(t, priorityRootBase+9, agedPriority(root, now, false), "cold first-row exclusivity must not age away")
	require.Equal(t, priorityRootBase, agedPriority(root, now, true))
	require.Equal(t, priorityIntent, agedPriority(intent, now, false))
	require.Equal(t, priorityIdlePrefetch, agedPriority(idle, now, false))
}
