package native_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

type contactAdded struct {
	ID      int64 `json:"id,string" wire:"int64"`
	Contact struct {
		Phone string `json:"phone"`
	} `json:"contact"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

// The key grammar is shared with notification definitions: snake_case only,
// versions from .v1.
func TestNewEvent_KeyGrammar(t *testing.T) {
	for _, ok := range []string{"core.user.created.v1", "crm.client.updated.v2", "finance.debt_line.paid_off.v10"} {
		_, err := native.NewEvent[struct{}](ok, native.EventSpec{})
		require.NoError(t, err, ok)
		require.True(t, native.ValidEventKey(ok))
	}
	for _, bad := range []string{"core.group.memberAdded.v1", "core.user.created.v0", "core.user.created", "user.v1", "Core.user.created.v1", "core.user.created.v01", "core..user.v1"} {
		_, err := native.NewEvent[struct{}](bad, native.EventSpec{})
		require.Error(t, err, bad)
		require.False(t, native.ValidEventKey(bad), bad)
	}
}

func TestNewEvent_SensitivePathsExistInThePayload(t *testing.T) {
	ev, err := native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{
		Sensitive: map[string][]string{"contact.phone": {"Contact.Read"}, "tags.name": {"Tag.Read"}},
	})
	require.NoError(t, err)
	require.Equal(t, map[string][]string{"contact.phone": {"Contact.Read"}, "tags.name": {"Tag.Read"}}, ev.Sensitive())

	for _, path := range []string{"email", "contact.email", "id.value", "contact.phone.x"} {
		_, err := native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{Sensitive: map[string][]string{path: {"Contact.Read"}}})
		require.Error(t, err, path)
	}
	_, err = native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{Sensitive: map[string][]string{"contact.phone": nil}})
	require.Error(t, err, "a sensitive field names the permissions that reveal it")
}

// Texts and access metadata are not part of the contract.
func TestNewEvent_MetadataDoesNotChangeTheContract(t *testing.T) {
	plain, err := native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{Subject: "acme.contact"})
	require.NoError(t, err)
	described, err := native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{
		Subject:        "acme.contact",
		Label:          manifest.Label{"en": "Contact added", "ru": "Контакт добавлен"},
		Descriptions:   manifest.Label{"en": "A contact was added.", "ru": "Добавлен контакт."},
		ReadPermission: "Contact.Read",
		Sensitive:      map[string][]string{"contact.phone": {"Contact.Read"}},
		Category:       "acme.contacts",
	})
	require.NoError(t, err)
	require.Equal(t, plain.Resource().ContractHash, described.Resource().ContractHash)

	res := described.Resource()
	require.Equal(t, "A contact was added.", res.Description, "the manifest description falls back to the English text")
	require.Equal(t, manifest.Label{"en": "Contact added", "ru": "Контакт добавлен"}, res.Label)
	require.Equal(t, manifest.Label{"en": "A contact was added.", "ru": "Добавлен контакт."}, described.Descriptions())
	require.Equal(t, "Contact.Read", described.ReadPermission())
	require.Equal(t, "acme.contacts", described.Category())

	explicit, err := native.NewEvent[contactAdded]("acme.contact.added.v1", native.EventSpec{Description: "Manifest text", Descriptions: manifest.Label{"en": "Localized"}})
	require.NoError(t, err)
	require.Equal(t, "Manifest text", explicit.Resource().Description)

	// Returned metadata is a copy.
	described.Descriptions()["en"] = "changed"
	described.Sensitive()["contact.phone"][0] = "changed"
	require.Equal(t, "A contact was added.", described.Descriptions()["en"])
	require.Equal(t, []string{"Contact.Read"}, described.Sensitive()["contact.phone"])
}
