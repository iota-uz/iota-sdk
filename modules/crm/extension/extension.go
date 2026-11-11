// Package extension declares the extension points the CRM module offers to
// packages of the extension runtime: the route of the client view page, which a
// package page can replace, and the slot of its sidebar, which any package can
// contribute to. They are contributed to the application's native.Set by the
// CRM module (crm.Natives); the module's pages use the IDs below.
package extension

import (
	"strings"

	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

const (
	// RouteClientView is the native client view page (BasePath + "/:id").
	// A package page implementing it replaces the screen once an
	// administrator binds it to the screen slot.
	RouteClientView = "crm.routes.clientView"
	// SlotClientSidebar is the sidebar of the client view page and of the
	// client drawer; its contributions receive ClientSidebarProps.
	SlotClientSidebar = "crm.slots.clientSidebar"
)

// ClientSidebarProps are the props every contribution to SlotClientSidebar
// receives; the slot declaration types them the same way.
type ClientSidebarProps struct {
	ClientID int `json:"clientId"`
}

// ClientViewPath is the path pattern of the client view page for the clients
// base path (default "/crm/clients").
func ClientViewPath(basePath string) string {
	return strings.TrimRight(basePath, "/") + "/:id"
}

// Items are the native contributions of the module's extension points: the
// typed client view route (under the clients page path) and the sidebar slot.
// The CRM contributor (crm.Natives) adds them to the application's native.Set.
func Items(clientsPath string) []native.Item {
	return []native.Item{
		native.RouteWith(RouteClientView, ClientViewPath(clientsPath), native.RouteSpec{
			Params: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
				{Name: "id", Type: manifest.Type{Kind: manifest.KindInt}},
			}},
			// A package page replacing the screen inherits the native requirement.
			Permission: permissions.ClientRead.Name(),
		}),
		native.Declare(manifest.Resource{
			ID: SlotClientSidebar, Kind: manifest.ResSlot,
			Props: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
				{Name: "clientId", Type: manifest.Type{Kind: manifest.KindInt}},
			}},
			// The host page needs the same permission to be viewed at all.
			Permission: permissions.ClientRead.Name(),
		}),
	}
}
