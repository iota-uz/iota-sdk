package services

import (
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
)

const (
	ClientResource = "crm.client"

	ClientCreated = "crm.client.created"
	ClientUpdated = "crm.client.updated"
	ClientDeleted = "crm.client.deleted"
)

// HistoryDefinitions are the business history event types of the crm module.
func HistoryDefinitions() []history.Definition {
	clientRead := permissions.ClientRead
	return history.Lifecycle("crm.client", "Clients.History", history.Resource{Name: ClientResource, Fields: []history.Field{
		{Name: "first_name", LabelKey: "Clients.Single.FirstName.Label", Type: history.ValueString},
		{Name: "last_name", LabelKey: "Clients.Single.LastName.Label", Type: history.ValueString},
		{Name: "middle_name", LabelKey: "Clients.Single.MiddleName.Label", Type: history.ValueString},
		{Name: "phone", LabelKey: "Clients.Single.Phone.Label", Type: history.ValueString, ReadPermission: &clientRead},
		{Name: "email", LabelKey: "Clients.Single.Email.Label", Type: history.ValueString, ReadPermission: &clientRead},
		{Name: "address", LabelKey: "Clients.Single.Address.Label", Type: history.ValueString, ReadPermission: &clientRead},
		{Name: "gender", LabelKey: "Clients.Single.Gender.Label", Type: history.ValueString},
		{Name: "date_of_birth", LabelKey: "Clients.Single.DateOfBirth.Label", Type: history.ValueDate, ReadPermission: &clientRead},
		{Name: "passport", LabelKey: "Clients.Single.PassportSeriesAndNumber.Label", Type: history.ValueString, ReadPermission: &clientRead},
		{Name: "pin", LabelKey: "Clients.Single.Pin.Label", Type: history.ValueString, ReadPermission: &clientRead},
		{Name: "comments", LabelKey: "Clients.Single.Comments.Label", Type: history.ValueString},
	}})
}

func clientSnapshot(c client.Client) history.Snapshot {
	s := history.Snapshot{
		"first_name":    c.FirstName(),
		"last_name":     c.LastName(),
		"middle_name":   c.MiddleName(),
		"phone":         nil,
		"email":         nil,
		"address":       c.Address(),
		"gender":        nil,
		"date_of_birth": nil,
		"passport":      nil,
		"pin":           nil,
		"comments":      c.Comments(),
	}
	if c.Phone() != nil {
		s["phone"] = c.Phone().Value()
	}
	if c.Email() != nil {
		s["email"] = c.Email().Value()
	}
	if c.Gender() != nil {
		s["gender"] = c.Gender().String()
	}
	if c.DateOfBirth() != nil {
		s["date_of_birth"] = history.Date(*c.DateOfBirth())
	}
	if c.Passport() != nil {
		s["passport"] = c.Passport().Identifier()
	}
	if c.Pin() != nil {
		s["pin"] = c.Pin().Value()
	}
	return s
}
