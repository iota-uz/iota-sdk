package role

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

type Type string
type Option func(r *role)

const (
	TypeUser   Type = "user"
	TypeSystem Type = "system"
)

func WithID(id uint) Option {
	return func(r *role) {
		r.id = id
	}
}

func WithType(type_ Type) Option {
	return func(r *role) {
		r.type_ = type_
	}
}

func WithPermissions(permissions []*permission.Permission) Option {
	return func(r *role) {
		r.permissions = permissions
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(r *role) {
		r.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(r *role) {
		r.updatedAt = t
	}
}

type Role interface {
	ID() uint
	Type() Type
	Name() string
	Description() string
	Permissions() []*permission.Permission
	CreatedAt() time.Time
	UpdatedAt() time.Time

	Can(perm *permission.Permission) bool
	CanUpdate() bool
	CanDelete() bool

	SetName(name string) Role
	SetDescription(description string) Role

	AddPermission(p *permission.Permission) Role
	SetPermissions(permissions []*permission.Permission) Role
}

func WithDescription(description string) Option {
	return func(r *role) {
		r.description = description
	}
}

func New(
	name string,
	opts ...Option,
) Role {
	r := &role{
		id:          0,
		type_:       TypeUser,
		name:        name,
		description: "",
		permissions: []*permission.Permission{},
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type role struct {
	id          uint
	type_       Type
	name        string
	description string
	permissions []*permission.Permission
	createdAt   time.Time
	updatedAt   time.Time
}

func (r *role) ID() uint {
	return r.id
}

func (r *role) Type() Type {
	return r.type_
}

func (r *role) Name() string {
	return r.name
}

func (r *role) Description() string {
	return r.description
}

func (r *role) Permissions() []*permission.Permission {
	return r.permissions
}

func (r *role) CreatedAt() time.Time {
	return r.createdAt
}

func (r *role) UpdatedAt() time.Time {
	return r.updatedAt
}

func (r *role) Can(perm *permission.Permission) bool {
	for _, p := range r.permissions {
		if p.Equals(*perm) {
			return true
		}
	}
	return false
}

func (r *role) CanUpdate() bool {
	return r.type_ != TypeSystem
}

func (r *role) CanDelete() bool {
	return r.type_ != TypeSystem
}

func (r *role) SetName(name string) Role {
	result := *r
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (r *role) SetDescription(description string) Role {
	result := *r
	result.description = description
	result.updatedAt = time.Now()
	return &result
}

func (r *role) AddPermission(p *permission.Permission) Role {
	result := *r
	result.permissions = append(result.permissions, p)
	result.updatedAt = time.Now()
	return &result
}

func (r *role) SetPermissions(permissions []*permission.Permission) Role {
	result := *r
	result.permissions = permissions
	result.updatedAt = time.Now()
	return &result
}
