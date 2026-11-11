package crm

import (
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// ClientsRoute is the public route ID of the CRM clients page; a package page
// may replace it through a screen binding.
const ClientsRoute = "crm.clients"

// ClientsPath is the path of the CRM clients page.
const ClientsPath = "/crm/clients"

// Natives is the CRM module's native registration for the extension runtime:
// the services' entity, list, events and contracts (services.Natives) and the
// clients page route. Applications compose it into their native.Set
// (modules.Natives).
func Natives(s *native.Set) error {
	if err := services.Natives(s); err != nil {
		return err
	}
	return s.Add(native.Route(ClientsRoute, ClientsPath))
}
