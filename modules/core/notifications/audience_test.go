package notifications_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/stretchr/testify/require"
)

func TestDeriveAudienceDedupeKey(t *testing.T) {
	producer := notifications.Producer{InstanceID: "wf", Occurrence: "notify#1"}
	event := notifications.Event{Key: notifications.TestEventKey, Subject: notifications.SubjectReference{Type: "acme.policy", ID: "p-1"}}
	channels := []notifications.Channel{notifications.ChannelSMS}
	address := map[notifications.Channel]string{notifications.ChannelSMS: "+998900000001"}
	direct := notifications.Audience{Users: []uint{7}, Records: []notifications.ObjectRef{{Entity: "acme.customer", Key: "c-1"}}, Addresses: []map[notifications.Channel]string{address}}
	legacy := notifications.DeriveDedupeKey(producer, event, []notifications.Addressee{
		notifications.AddressAddressee(address), notifications.UserAddressee(7), notifications.RecordAddressee("acme.customer", "c-1"),
	}, channels)
	require.NotEmpty(t, legacy)
	require.Equal(t, legacy, notifications.DeriveAudienceDedupeKey(producer, event, direct, nil, channels), "explicit sends keep their dedupe identity")

	group := direct
	group.Groups = []uuid.UUID{uuid.MustParse("6f1c1a43-77b4-4a43-8c39-3c1b1f1b8a11")}
	require.NotEqual(t, legacy, notifications.DeriveAudienceDedupeKey(producer, event, group, nil, channels))
	source := &notifications.Event{Key: "acme.policy.created.v1", ID: "e-1"}
	withParticipant := notifications.Audience{Participants: []string{"created_user"}}
	first := notifications.DeriveAudienceDedupeKey(producer, event, withParticipant, source, channels)
	other := *source
	other.ID = "e-2"
	require.NotEqual(t, first, notifications.DeriveAudienceDedupeKey(producer, event, withParticipant, &other, channels), "participants of another source event are another audience")
	require.Empty(t, notifications.DeriveAudienceDedupeKey(notifications.Producer{}, event, direct, nil, channels))
}
