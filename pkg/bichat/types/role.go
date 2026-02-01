package types

// Role represents the role of a message sender in a conversation.
type Role string

const (
	// RoleSystem represents a system message that sets context or instructions
	RoleSystem Role = "system"

	// RoleUser represents a message from the user
	RoleUser Role = "user"

	// RoleAssistant represents a message from the AI assistant
	RoleAssistant Role = "assistant"

	// RoleTool represents a message from a tool execution
	RoleTool Role = "tool"
)

// IsSystem returns true if the role is RoleSystem.
func (r Role) IsSystem() bool {
	return r == RoleSystem
}

// IsUser returns true if the role is RoleUser.
func (r Role) IsUser() bool {
	return r == RoleUser
}

// IsAssistant returns true if the role is RoleAssistant.
func (r Role) IsAssistant() bool {
	return r == RoleAssistant
}

// IsTool returns true if the role is RoleTool.
func (r Role) IsTool() bool {
	return r == RoleTool
}

// Valid returns true if the role is one of the defined role constants.
func (r Role) Valid() bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}
