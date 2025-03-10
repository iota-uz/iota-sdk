package group

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

// ---- Interface ----

type Option func(g *group)

type Group interface {
	ID() uuid.UUID
	Name() string
	Description() string
	Users() []user.User
	Roles() []role.Role
	CreatedAt() time.Time
	UpdatedAt() time.Time

	AddUser(u user.User) Group
	RemoveUser(u user.User) Group
	AssignRole(r role.Role) Group
	RemoveRole(r role.Role) Group
	SetName(name string) Group
	SetDescription(desc string) Group
}

// ---- Implementations ----

func WithID(id uuid.UUID) Option {
	return func(g *group) {
		g.id = id
	}
}

func WithDescription(desc string) Option {
	return func(g *group) {
		g.description = desc
	}
}

func WithRoles(roles []role.Role) Option {
	return func(g *group) {
		g.roles = roles
	}
}

func WithUsers(users []user.User) Option {
	return func(g *group) {
		g.users = users
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(g *group) {
		g.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(g *group) {
		g.updatedAt = t
	}
}

func New(name string, opts ...Option) Group {
	g := &group{
		id:        uuid.New(),
		name:      name,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(g)
	}
	return g
}

type group struct {
	id          uuid.UUID
	name        string
	description string
	roles       []role.Role
	users       []user.User
	createdAt   time.Time
	updatedAt   time.Time
}

func (g *group) ID() uuid.UUID {
	return g.id
}

func (g *group) Name() string {
	return g.name
}

func (g *group) Description() string {
	return g.description
}

func (g *group) Users() []user.User {
	return g.users
}

func (g *group) Roles() []role.Role {
	return g.roles
}

func (g *group) CreatedAt() time.Time {
	return g.createdAt
}

func (g *group) UpdatedAt() time.Time {
	return g.updatedAt
}

func (g *group) SetName(name string) Group {
	r := *g
	r.name = name
	r.updatedAt = g.updatedAt
	return &r
}

func (g *group) SetDescription(desc string) Group {
	r := *g
	r.description = desc
	r.updatedAt = g.updatedAt
	return &r
}

func (g *group) AssignRole(r role.Role) Group {
	res := *g
	res.roles = append(res.roles, r)
	res.updatedAt = time.Now()
	return &res
}

func (g *group) RemoveRole(r role.Role) Group {
	res := *g
	filteredRoles := make([]role.Role, 0, len(res.roles)-1)
	for _, v := range res.roles {
		if v.ID() == r.ID() {
			continue
		}
		filteredRoles = append(filteredRoles, v)
	}
	res.roles = filteredRoles
	res.updatedAt = time.Now()
	return &res
}

func (g *group) AddUser(u user.User) Group {
	r := *g
	r.users = append(r.users, u)
	r.updatedAt = time.Now()
	return &r
}

func (g *group) RemoveUser(u user.User) Group {
	r := *g
	filteredUsers := make([]user.User, 0, len(r.users)-1)
	for _, v := range r.users {
		if v.ID() == u.ID() {
			continue
		}
		filteredUsers = append(filteredUsers, v)
	}
	r.users = filteredUsers
	r.updatedAt = time.Now()
	return &r
}
