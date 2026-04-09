package composition

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type fooEvent struct{ value int }

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
