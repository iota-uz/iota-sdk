package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionMemberRole describes a participant role in a session.
type SessionMemberRole string

const (
	SessionMemberRoleNone    SessionMemberRole = "NONE"
	SessionMemberRoleOwner   SessionMemberRole = "OWNER"
	SessionMemberRoleEditor  SessionMemberRole = "EDITOR"
	SessionMemberRoleViewer  SessionMemberRole = "VIEWER"
	SessionMemberRoleReadAll SessionMemberRole = "READ_ALL"
)

func (r SessionMemberRole) String() string { return string(r) }

func (r SessionMemberRole) ValidMemberRole() bool {
	switch r {
	case SessionMemberRoleEditor, SessionMemberRoleViewer:
		return true
	default:
		return false
	}
}

func ParseSessionMemberRole(raw string) SessionMemberRole {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "OWNER":
		return SessionMemberRoleOwner
	case "EDITOR":
		return SessionMemberRoleEditor
	case "VIEWER":
		return SessionMemberRoleViewer
	case "READ_ALL", "READALL":
		return SessionMemberRoleReadAll
	default:
		return SessionMemberRoleNone
	}
}

// SessionAccessSource tells how access was granted.
type SessionAccessSource string

const (
	SessionAccessSourceNone       SessionAccessSource = "none"
	SessionAccessSourceOwner      SessionAccessSource = "owner"
	SessionAccessSourceMember     SessionAccessSource = "member"
	SessionAccessSourcePermission SessionAccessSource = "permission"
)

// SessionUser is a lightweight user identity projection for chat/session APIs.
type SessionUser struct {
	ID        int64
	FirstName string
	LastName  string
}

// Initials returns up to two initials from FirstName/LastName.
func (u SessionUser) Initials() string {
	first := strings.TrimSpace(u.FirstName)
	last := strings.TrimSpace(u.LastName)
	out := ""
	if first != "" {
		out += strings.ToUpper(string([]rune(first)[0]))
	}
	if last != "" {
		out += strings.ToUpper(string([]rune(last)[0]))
	}
	if out == "" {
		return "U"
	}
	return out
}

// SessionAccess describes resolved permissions for a user in a session.
type SessionAccess struct {
	Role             SessionMemberRole
	Source           SessionAccessSource
	CanRead          bool
	CanWrite         bool
	CanManageMembers bool
}

// SessionSummary is an enriched session row for chat APIs.
type SessionSummary struct {
	Session     Session
	Owner       SessionUser
	Access      SessionAccess
	MemberCount int
	IsGroup     bool
}

// SessionMember is a participant in a shared/group session.
type SessionMember struct {
	SessionID uuid.UUID
	User      SessionUser
	Role      SessionMemberRole
	CreatedAt time.Time
	UpdatedAt time.Time
}
