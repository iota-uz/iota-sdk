package notifications_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/notificationstest"
)

type tenantConnections map[uuid.UUID]bool

func (t tenantConnections) Connection(_ context.Context, tenantID uuid.UUID, spec notifications.ChannelSpec) (string, bool, error) {
	return "platform:" + spec.ConnectorKind, t[tenantID], nil
}

func linkedSpec(channel notifications.Channel) notifications.ChannelSpec {
	return notifications.ChannelSpec{
		Channel: channel, Order: 40, Address: notifications.AddressLinked, ConnectorKind: "telegram_bot",
		Label:   map[string]string{"en": "Telegram", "ru": "Телеграм"},
		LinkURL: func(context.Context, uuid.UUID, string, string) (string, error) { return "https://t.me/x", nil },
	}
}

func TestChannelValid_IsAFormatCheck(t *testing.T) {
	for _, c := range []notifications.Channel{"in_app", "email", "sms", "push", "telegram", "whatsapp", "ab"} {
		assert.True(t, c.Valid(), c)
	}
	for _, c := range []notifications.Channel{"Telegram", "", "a", "1sms", "tele-gram", "x234567890123456789012345678901234"} {
		assert.False(t, c.Valid(), c)
	}
	assert.True(t, notifications.ChannelEmail.External())
	assert.True(t, notifications.ChannelTelegram.External())
	assert.False(t, notifications.ChannelInApp.External())
}

func TestRegistry_BuiltinsAndContributedChannels(t *testing.T) {
	email := notificationstest.NewEmail()
	reg, err := notifications.NewChannelRegistry(email)
	require.NoError(t, err)
	telegram := notificationstest.NewAdapter(notifications.ChannelTelegram)
	require.NoError(t, reg.Register(linkedSpec(notifications.ChannelTelegram), telegram))
	require.Error(t, reg.Register(linkedSpec(notifications.ChannelTelegram), telegram), "a channel twice fails")
	require.Error(t, reg.Register(notifications.ChannelSpec{Channel: "Telegram2", Address: notifications.AddressLinked}, notificationstest.NewAdapter("Telegram2")), "a malformed name fails")
	require.Error(t, reg.Register(notifications.ChannelSpec{Channel: "whatsapp", Address: notifications.AddressLinked}, notificationstest.NewAdapter("whatsapp")), "a linked channel needs LinkURL")

	all := reg.All()
	order := make([]notifications.Channel, 0, len(all))
	for _, spec := range all {
		order = append(order, spec.Channel)
	}
	assert.Equal(t, []notifications.Channel{"in_app", "email", "sms", "push", "telegram"}, order)

	spec, adapter, ok := reg.Lookup(notifications.ChannelEmail)
	require.True(t, ok)
	assert.Same(t, email, adapter)
	assert.Equal(t, "Эл. почта", spec.LabelFor("ru"))
	_, adapter, ok = reg.Lookup(notifications.ChannelSMS)
	assert.True(t, ok)
	assert.Nil(t, adapter, "a built-in channel without a provider is registered without an adapter")
	_, _, ok = reg.Lookup("whatsapp")
	assert.False(t, ok)

	tg, _, _ := reg.Lookup(notifications.ChannelTelegram)
	assert.Equal(t, "Телеграм", tg.LabelFor("ru"))
	assert.Equal(t, "Telegram", tg.LabelFor("uz-Cyrl"), "a missing language falls back to English")
}

func TestRegistry_ForTenantHidesChannelsWithoutConnection(t *testing.T) {
	reg, err := notifications.NewChannelRegistry(notificationstest.NewEmail())
	require.NoError(t, err)
	reg.WithSource(func() ([]notifications.ChannelContribution, error) {
		return []notifications.ChannelContribution{{Spec: linkedSpec(notifications.ChannelTelegram), Adapter: notificationstest.NewAdapter(notifications.ChannelTelegram)}}, nil
	})
	with, without := uuid.New(), uuid.New()
	reg.WithConnections(tenantConnections{with: true})
	channels := func(tenant uuid.UUID) []notifications.Channel {
		specs, err := reg.ForTenant(context.Background(), tenant)
		require.NoError(t, err)
		var out []notifications.Channel
		for _, s := range specs {
			out = append(out, s.Channel)
		}
		return out
	}
	assert.Equal(t, []notifications.Channel{"in_app", "email", "telegram"}, channels(with))
	assert.Equal(t, []notifications.Channel{"in_app", "email"}, channels(without))
}

func TestRegistry_DuplicateContributionFailsLoad(t *testing.T) {
	reg, err := notifications.NewChannelRegistry()
	require.NoError(t, err)
	a := notificationstest.NewAdapter(notifications.ChannelTelegram)
	reg.WithSource(func() ([]notifications.ChannelContribution, error) {
		return []notifications.ChannelContribution{{Spec: linkedSpec(notifications.ChannelTelegram), Adapter: a}, {Spec: linkedSpec(notifications.ChannelTelegram), Adapter: a}}, nil
	})
	require.Error(t, reg.Load())
}

func TestRegistry_OptInChannels(t *testing.T) {
	reg, err := notifications.NewChannelRegistry()
	require.NoError(t, err)
	spec := linkedSpec("whatsapp")
	spec.OptIn = true
	require.NoError(t, reg.Register(spec, notificationstest.NewAdapter("whatsapp")))
	assert.Equal(t, []notifications.Channel{"whatsapp"}, reg.OptInChannels())
}

func TestSummaryFor_UsesTheSpec(t *testing.T) {
	reg, err := notifications.NewChannelRegistry()
	require.NoError(t, err)
	content := notifications.ChannelContent{Title: "T", Body: "B", Subject: "S", Text: "X"}
	title, body := notifications.SummaryFor(reg, notifications.ChannelEmail, content)
	assert.Equal(t, "S", title)
	assert.Equal(t, "X", body)
	title, body = notifications.SummaryFor(reg, notifications.ChannelSMS, content)
	assert.Empty(t, title)
	assert.Equal(t, "X", body)
}
