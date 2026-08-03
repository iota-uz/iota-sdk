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

func TestExecutionSessionPreemptsRunningSpeculationForVisibleWork(t *testing.T) {
	session := newExecutionSession(2, time.Second)
	rootRelease := make(chan struct{})
	root := session.submit(t.Context(), "root", priorityRootBase, 0, func(context.Context) (any, error) {
		<-rootRelease
		return "root", nil
	})
	backgroundStarted := make(chan struct{})
	background := session.submit(t.Context(), "prefetch", priorityIdlePrefetch, 0, func(ctx context.Context) (any, error) {
		close(backgroundStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	session.enableBackground()
	<-backgroundStarted

	visibleStarted := make(chan struct{})
	visible := session.submit(t.Context(), "visible", priorityRootBase, 1, func(context.Context) (any, error) {
		close(visibleStarted)
		return "visible", nil
	})
	require.ErrorIs(t, (<-background.result).err, context.Canceled)
	select {
	case <-visibleStarted:
	case <-time.After(time.Second):
		t.Fatal("visible work did not take the speculative slot")
	}
	require.Equal(t, "visible", (<-visible.result).value)
	close(rootRelease)
	require.Equal(t, "root", (<-root.result).value)
}

func TestExecuteDerivedSurfacePromotesIntentInsteadOfDuplicatingDrawerWork(t *testing.T) {
	handlers := &Handlers{sessions: NewSessionRegistry(), workTimeout: time.Second, observer: noopObserver{}}
	started := make(chan struct{})
	release := make(chan struct{})
	var runs int
	run := func(context.Context) (any, error) {
		runs++
		close(started)
		<-release
		return "drawer", nil
	}
	session := handlers.session("snapshot")
	session.enableBackground()
	intentResult := make(chan any, 1)
	go func() {
		value, _ := handlers.ExecuteDerivedSurface(t.Context(), "snapshot", "metric:loss", DerivedSurfaceIntent, run)
		intentResult <- value
	}()
	<-started
	interactiveResult := make(chan any, 1)
	go func() {
		value, _ := handlers.ExecuteDerivedSurface(t.Context(), "snapshot", "metric:loss", DerivedSurfaceInteractive, run)
		interactiveResult <- value
	}()
	deadline := time.Now().Add(time.Second)
	for {
		session.mu.Lock()
		joined := len(session.jobs["derived:snapshot:metric:loss"].waiters) == 2
		session.mu.Unlock()
		if joined {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("interactive drawer did not join intent work")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	require.Equal(t, "drawer", <-intentResult)
	require.Equal(t, "drawer", <-interactiveResult)
	require.Equal(t, 1, runs)
}

func TestExecuteExportSurfaceJoinsSameSnapshotThroughputWork(t *testing.T) {
	handlers := &Handlers{sessions: NewSessionRegistry(), workTimeout: time.Second, observer: noopObserver{}}
	started := make(chan struct{})
	release := make(chan struct{})
	var runs int
	run := func(context.Context) (any, error) {
		runs++
		close(started)
		<-release
		return "workbook-source", nil
	}
	firstResult := make(chan any, 1)
	go func() {
		value, _ := handlers.ExecuteExportSurface(t.Context(), "snapshot", "dashboard", run)
		firstResult <- value
	}()
	<-started
	secondResult := make(chan any, 1)
	go func() {
		value, _ := handlers.ExecuteExportSurface(t.Context(), "snapshot", "dashboard", run)
		secondResult <- value
	}()

	session := handlers.session("snapshot")
	deadline := time.Now().Add(time.Second)
	for {
		session.mu.Lock()
		joined := len(session.jobs["export:snapshot:dashboard"].waiters) == 2
		session.mu.Unlock()
		if joined {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second export did not join the snapshot export graph")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	require.Equal(t, "workbook-source", <-firstResult)
	require.Equal(t, "workbook-source", <-secondResult)
	require.Equal(t, 1, runs)
}

func TestDerivedSnapshotSharesParentRegistrySession(t *testing.T) {
	registry := NewSessionRegistry()
	parent := &Handlers{sessions: registry, workTimeout: time.Second, observer: noopObserver{}}
	child := &Handlers{sessions: registry, workTimeout: time.Second, observer: noopObserver{}}
	parentSession := parent.session("parent")
	require.NoError(t, child.BindExecutionSession("child", "parent"))
	require.Same(t, parentSession, child.session("child"))

	child.releaseSession("child")
	require.Same(t, parentSession, parent.session("parent"), "releasing a drawer must not release its parent dashboard")
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

func TestPanelViewportRankReordersOnlyRowsBelowColdHero(t *testing.T) {
	priorities := panelPriorities{
		"hero":   {priority: priorityRootBase, order: 0},
		"second": {priority: priorityRootBase + 1, order: 1},
		"third":  {priority: priorityRootBase + 2, order: 2},
	}
	visible := 0
	offscreen := 2

	heroPriority, _ := priorities.forRequest(PanelRequest{PanelID: "hero", ViewportRank: &offscreen})
	secondPriority, _ := priorities.forRequest(PanelRequest{PanelID: "second", ViewportRank: &offscreen})
	thirdPriority, _ := priorities.forRequest(PanelRequest{PanelID: "third", ViewportRank: &visible})

	require.Equal(t, priorityRootBase, heroPriority, "viewport must not weaken cold first-row exclusivity")
	require.Equal(t, priorityRootBase+3, secondPriority)
	require.Equal(t, priorityRootBase+1, thirdPriority, "visible lower content should precede offscreen siblings")
}
