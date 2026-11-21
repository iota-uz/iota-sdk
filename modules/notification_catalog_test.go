package modules_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// atRepositoryRoot runs the test from the repository root, where itf.Setup
// finds the migrations and locales.
func atRepositoryRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(".."))
	t.Cleanup(func() { require.NoError(t, os.Chdir(wd)) })
}

// Every native event the SDK modules publish is in the notification settings
// catalog (which the outbox bridge also lists its keys from), under its
// module, with the event's label as its name.
func TestEveryNativeEventIsInTheNotificationCatalog(t *testing.T) {
	atRepositoryRoot(t)
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))
	catalog, err := composition.Resolve[*notifications.Catalog](env.Container)
	require.NoError(t, err)

	exposed := map[string]notifications.Definition{}
	for _, d := range catalog.Definitions() {
		exposed[d.Key] = d
	}
	require.NotEmpty(t, set.Events())
	for _, ev := range set.Events() {
		d, ok := exposed[ev.ID()]
		require.True(t, ok, "%s is in the notification settings catalog", ev.ID())
		require.NotNil(t, d.Event, "%s is event-backed, so the outbox bridge delivers it", ev.ID())
		require.Equal(t, ev.ID(), d.Event.ID())
		require.NotEmpty(t, d.Module, ev.ID())
		meta := ev.(native.EventMetadata)
		for locale, label := range meta.Label() {
			require.Equal(t, label, d.Name[locale], "%s: the name of the notification is the label of the event", ev.ID())
		}
		require.NotEmpty(t, d.RecipientKeys, "%s can be delivered to its actor", ev.ID())
	}
}
