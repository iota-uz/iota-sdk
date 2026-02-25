package domain

import (
	"errors"
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

func (r SessionMemberRole) ValidAccessRole() bool {
	switch r {
	case SessionMemberRoleNone, SessionMemberRoleOwner, SessionMemberRoleEditor, SessionMemberRoleViewer, SessionMemberRoleReadAll:
		return true
	default:
		return false
	}
}

func (r SessionMemberRole) ValidMemberRole() bool {
	return r == SessionMemberRoleEditor || r == SessionMemberRoleViewer
}

var (
	ErrInvalidSessionMemberRole = errors.New("invalid session member role")
	ErrInvalidSessionAccess     = errors.New("invalid session access")
	ErrInvalidSessionUser       = errors.New("invalid session user")
	ErrInvalidSessionMember     = errors.New("invalid session member")
	ErrInvalidSessionSummary    = errors.New("invalid session summary")
	ErrInvalidMemberUpsert      = errors.New("invalid member upsert")
	ErrInvalidMemberRemoval     = errors.New("invalid member removal")
	ErrSessionAccessDenied      = errors.New("session access denied")
)

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

func NewSessionMemberRole(raw string) (SessionMemberRole, error) {
	role := ParseSessionMemberRole(raw)
	if !role.ValidMemberRole() {
		return SessionMemberRoleNone, ErrInvalidSessionMemberRole
	}
	return role, nil
}

// NewSessionAccess builds access flags for a role/source pair.
func NewSessionAccess(role SessionMemberRole, source SessionAccessSource) (SessionAccess, error) {
	if !role.ValidAccessRole() || !source.Valid() {
		return SessionAccess{}, ErrInvalidSessionAccess
	}

	switch role {
	case SessionMemberRoleOwner:
		if source != SessionAccessSourceOwner {
			return SessionAccess{}, ErrInvalidSessionAccess
		}
	case SessionMemberRoleEditor, SessionMemberRoleViewer:
		if source != SessionAccessSourceMember {
			return SessionAccess{}, ErrInvalidSessionAccess
		}
	case SessionMemberRoleReadAll:
		if source != SessionAccessSourcePermission {
			return SessionAccess{}, ErrInvalidSessionAccess
		}
	case SessionMemberRoleNone:
		if source != SessionAccessSourceNone {
			return SessionAccess{}, ErrInvalidSessionAccess
		}
	}

	access := SessionAccess{
		Role:   role,
		Source: source,
	}

	switch role {
	case SessionMemberRoleOwner:
		access.CanRead = true
		access.CanWrite = true
		access.CanManageMembers = true
	case SessionMemberRoleEditor:
		access.CanRead = true
		access.CanWrite = true
	case SessionMemberRoleViewer:
		access.CanRead = true
	case SessionMemberRoleReadAll:
		access.CanRead = true
	case SessionMemberRoleNone:
	}

	return access, nil
}

// SessionAccessSource tells how access was granted.
type SessionAccessSource string

const (
	SessionAccessSourceNone       SessionAccessSource = "none"
	SessionAccessSourceOwner      SessionAccessSource = "owner"
	SessionAccessSourceMember     SessionAccessSource = "member"
	SessionAccessSourcePermission SessionAccessSource = "permission"
)

func ParseSessionAccessSource(raw string) SessionAccessSource {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner":
		return SessionAccessSourceOwner
	case "member":
		return SessionAccessSourceMember
	case "permission":
		return SessionAccessSourcePermission
	default:
		return SessionAccessSourceNone
	}
}

func (s SessionAccessSource) Valid() bool {
	switch s {
	case SessionAccessSourceNone, SessionAccessSourceOwner, SessionAccessSourceMember, SessionAccessSourcePermission:
		return true
	default:
		return false
	}
}

// SessionUser is a lightweight user identity projection for chat/session APIs.
type SessionUser struct {
	ID        int64
	FirstName string
	LastName  string
}

func NewSessionUser(id int64, firstName, lastName string) (SessionUser, error) {
	if id <= 0 {
		return SessionUser{}, ErrInvalidSessionUser
	}
	return SessionUser{
		ID:        id,
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
	}, nil
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

func (a SessionAccess) Require(requireWrite bool, requireManageMembers bool) error {
	if !a.CanRead || (requireWrite && !a.CanWrite) || (requireManageMembers && !a.CanManageMembers) {
		return ErrSessionAccessDenied
	}
	return nil
}

func (a SessionAccess) GrantReadAll() (SessionAccess, error) {
	if a.CanRead {
		return a, nil
	}
	return NewSessionAccess(SessionMemberRoleReadAll, SessionAccessSourcePermission)
}

type SessionSummarySpec struct {
	Session     Session
	Owner       SessionUser
	Access      SessionAccess
	MemberCount int
}

// SessionSummary is an enriched session row for chat APIs.
type SessionSummary struct {
	Session     Session
	Owner       SessionUser
	Access      SessionAccess
	MemberCount int
	IsGroup     bool
}

func NewSessionSummary(spec SessionSummarySpec) (SessionSummary, error) {
	if spec.Session == nil {
		return SessionSummary{}, ErrInvalidSessionSummary
	}
	owner, err := NewSessionUser(spec.Owner.ID, spec.Owner.FirstName, spec.Owner.LastName)
	if err != nil {
		return SessionSummary{}, err
	}
	access, err := NewSessionAccess(spec.Access.Role, spec.Access.Source)
	if err != nil {
		return SessionSummary{}, err
	}
	if spec.MemberCount < 1 {
		return SessionSummary{}, ErrInvalidSessionSummary
	}
	return SessionSummary{
		Session:     spec.Session,
		Owner:       owner,
		Access:      access,
		MemberCount: spec.MemberCount,
		IsGroup:     spec.MemberCount > 1,
	}, nil
}

type SessionMemberSpec struct {
	SessionID uuid.UUID
	User      SessionUser
	Role      SessionMemberRole
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionMember is a participant in a shared/group session.
type SessionMember struct {
	SessionID uuid.UUID
	User      SessionUser
	Role      SessionMemberRole
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewSessionMember(spec SessionMemberSpec) (SessionMember, error) {
	if spec.SessionID == uuid.Nil {
		return SessionMember{}, ErrInvalidSessionMember
	}
	user, err := NewSessionUser(spec.User.ID, spec.User.FirstName, spec.User.LastName)
	if err != nil {
		return SessionMember{}, err
	}
	if !spec.Role.ValidMemberRole() {
		return SessionMember{}, ErrInvalidSessionMemberRole
	}
	createdAt := spec.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := spec.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	if updatedAt.Before(createdAt) {
		return SessionMember{}, ErrInvalidSessionMember
	}
	return SessionMember{
		SessionID: spec.SessionID,
		User:      user,
		Role:      spec.Role,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (m SessionMember) ChangeRole(role SessionMemberRole, now time.Time) (SessionMember, error) {
	if !role.ValidMemberRole() {
		return SessionMember{}, ErrInvalidSessionMemberRole
	}
	updatedAt := now
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	if updatedAt.Before(m.CreatedAt) {
		return SessionMember{}, ErrInvalidSessionMember
	}
	next := m
	next.Role = role
	next.UpdatedAt = updatedAt
	return next, nil
}

type SessionMemberUpsertSpec struct {
	SessionID uuid.UUID
	UserID    int64
	Role      SessionMemberRole
}

type SessionMemberUpsert struct {
	sessionID uuid.UUID
	userID    int64
	role      SessionMemberRole
}

func NewSessionMemberUpsert(spec SessionMemberUpsertSpec) (SessionMemberUpsert, error) {
	if spec.SessionID == uuid.Nil || spec.UserID <= 0 || !spec.Role.ValidMemberRole() {
		return SessionMemberUpsert{}, ErrInvalidMemberUpsert
	}
	return SessionMemberUpsert{
		sessionID: spec.SessionID,
		userID:    spec.UserID,
		role:      spec.Role,
	}, nil
}

func (c SessionMemberUpsert) SessionID() uuid.UUID    { return c.sessionID }
func (c SessionMemberUpsert) UserID() int64           { return c.userID }
func (c SessionMemberUpsert) Role() SessionMemberRole { return c.role }

type SessionMemberRemovalSpec struct {
	SessionID uuid.UUID
	UserID    int64
}

type SessionMemberRemoval struct {
	sessionID uuid.UUID
	userID    int64
}

func NewSessionMemberRemoval(spec SessionMemberRemovalSpec) (SessionMemberRemoval, error) {
	if spec.SessionID == uuid.Nil || spec.UserID <= 0 {
		return SessionMemberRemoval{}, ErrInvalidMemberRemoval
	}
	return SessionMemberRemoval{
		sessionID: spec.SessionID,
		userID:    spec.UserID,
	}, nil
}

func (c SessionMemberRemoval) SessionID() uuid.UUID { return c.sessionID }
func (c SessionMemberRemoval) UserID() int64        { return c.userID }
