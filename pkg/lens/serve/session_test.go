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
	require.Equal(t, "foreground", (<-foreground).value)
	select {
	case <-backgroundStarted:
	case <-time.After(time.Second):
		t.Fatal("idle prefetch did not start after foreground completion")
	}
	require.Equal(t, "background", (<-background).value)
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
	require.Equal(t, "child", (<-prefetch).value)
	require.Equal(t, "child", (<-interactive).value)
}
