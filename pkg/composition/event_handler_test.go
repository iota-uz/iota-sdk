package composition

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type fooEvent struct{ value int }

// subscriberService simulates a service registered via ProvideFunc whose
// method is subscribed to the event bus.
type subscriberService struct {
	received []int
}

func (s *subscriberService) OnFoo(e *fooEvent) {
	s.received = append(s.received, e.value)
}

func TestContributeEventHandler_AutoSubscribeAndUnsubscribe(t *testing.T) {
	bus := eventbus.NewEventPublisher(logrus.New())
	var seen []int

	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "consumer"},
		build: func(builder *Builder) error {
			ContributeEventHandler(builder, func(e *fooEvent) {
				seen = append(seen, e.value)
			})
			return nil
		},
	})
	require.NoError(t, err)

	// EventBus is auto-provided from BuildContext.eventPublisher.
	bctx := BuildContext{eventPublisher: bus}
	container, err := engine.Compile(bctx)
	require.NoError(t, err)

	require.Equal(t, 0, bus.SubscribersCount())
	require.NoError(t, Start(context.Background(), container))
	require.Equal(t, 1, bus.SubscribersCount())

	bus.Publish(&fooEvent{value: 7})
	bus.Publish(&fooEvent{value: 8})
	require.Equal(t, []int{7, 8}, seen)

	require.NoError(t, Stop(context.Background(), container))
	require.Equal(t, 0, bus.SubscribersCount())
}

func TestContributeEventHandlerFunc_ResolvesFromContainer(t *testing.T) {
	bus := eventbus.NewEventPublisher(logrus.New())

	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "consumer"},
		build: func(builder *Builder) error {
			// Service is provided via ProvideFunc so the container has to
			// construct it before the factory runs.
			ProvideFunc(builder, func() *subscriberService {
				return &subscriberService{}
			})
			ContributeEventHandlerFunc(builder, func(svc *subscriberService) any {
				return svc.OnFoo
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{eventPublisher: bus})
	require.NoError(t, err)

	// Until Start runs, no handler is attached.
	require.Equal(t, 0, bus.SubscribersCount())
	require.NoError(t, Start(context.Background(), container))
	require.Equal(t, 1, bus.SubscribersCount())

	// Publishing events must reach the method bound to the resolved service.
	bus.Publish(&fooEvent{value: 1})
	bus.Publish(&fooEvent{value: 2})

	svc, err := Resolve[*subscriberService](container)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, svc.received)

	// Stop unsubscribes so subsequent publishes do not reach the handler.
	require.NoError(t, Stop(context.Background(), container))
	require.Equal(t, 0, bus.SubscribersCount())
}

func TestContributeEventHandlerFunc_NilHandlerIsCompileError(t *testing.T) {
	bus := eventbus.NewEventPublisher(logrus.New())

	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "consumer"},
		build: func(builder *Builder) error {
			ProvideFunc(builder, func() *subscriberService {
				return &subscriberService{}
			})
			// Factory intentionally returns nil — a misconfigured wiring.
			// The container treats this as an explicit error and surfaces
			// it at engine compile time rather than silently dropping the
			// subscription.
			ContributeEventHandlerFunc(builder, func(*subscriberService) any {
				return nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{eventPublisher: bus})
	require.Error(t, err)
	require.Contains(t, err.Error(), "factory returned nil handler")
}
