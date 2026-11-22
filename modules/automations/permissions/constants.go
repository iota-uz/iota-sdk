// Package permissions holds the RBAC permissions of the automations module.
package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceAutomation permission.Resource = "automation"
	ResourceConnection permission.Resource = "connection"
)

// Actions of automations: Author edits graphs, Enable turns versions on and off, Operate acts on runs
// (retry, cancel), Manage administers the tenant's connections.
const (
	ActionAuthor  permission.Action = "author"
	ActionEnable  permission.Action = "enable"
	ActionOperate permission.Action = "operate"
	ActionManage  permission.Action = "manage"
)

var (
	// AutomationsRead opens the automations pages.
	AutomationsRead = permission.MustCreate(
		uuid.MustParse("aae40f4c-f964-42f8-8c8a-e7c593ef6fcc"),
		"Automations.Read", ResourceAutomation, permission.ActionRead, permission.ModifierAll,
	)
	// AutomationsAuthor edits automation graphs; it also lists the tenant's connections, which an author
	// picks in a node.
	AutomationsAuthor = permission.MustCreate(
		uuid.MustParse("bd75bb7b-e27c-4651-85c8-15c6b53e33ec"),
		"Automations.Author", ResourceAutomation, ActionAuthor, permission.ModifierAll,
	)
	// AutomationsEnable enables and disables automation versions.
	AutomationsEnable = permission.MustCreate(
		uuid.MustParse("9fd09821-6778-4db4-beb0-0f4ddb76422b"),
		"Automations.Enable", ResourceAutomation, ActionEnable, permission.ModifierAll,
	)
	// AutomationsOperate acts on runs.
	AutomationsOperate = permission.MustCreate(
		uuid.MustParse("3c78b78b-fa9b-48de-a076-c6d7010e9e13"),
		"Automations.Operate", ResourceAutomation, ActionOperate, permission.ModifierAll,
	)
	// ConnectionsManage opens the integrations page and every action on it.
	ConnectionsManage = permission.MustCreate(
		uuid.MustParse("ea7cde5d-e133-44f4-9fa9-9dd32b4df37f"),
		"Connections.Manage", ResourceConnection, ActionManage, permission.ModifierAll,
	)
)

// Permissions are every permission of the module.
var Permissions = []permission.Permission{
	AutomationsRead,
	AutomationsAuthor,
	AutomationsEnable,
	AutomationsOperate,
	ConnectionsManage,
}
