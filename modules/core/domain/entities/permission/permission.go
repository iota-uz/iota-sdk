package permission

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

const (
	ModifierAll Modifier = "all"
	ModifierOwn Modifier = "own"
)

// Permission interface (public)
type Permission interface {
	ID() uuid.UUID
	Name() string
	Resource() Resource
	Action() Action
	Modifier() Modifier

	// Business logic
	Equals(Permission) bool
	Matches(resource Resource, action Action) bool
	IsValid() bool
}

// permission struct (private implementation)
type permission struct {
	id       uuid.UUID
	name     string
	resource Resource
	action   Action
	modifier Modifier
}

// Compile-time interface check
var _ Permission = (*permission)(nil)

// Constructor
func New(opts ...Option) Permission {
	p := &permission{
		id: uuid.New(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Option type
type Option func(*permission)

// Option functions
func WithID(id uuid.UUID) Option {
	return func(p *permission) { p.id = id }
}

func WithName(name string) Option {
	return func(p *permission) { p.name = name }
}

func WithResource(resource Resource) Option {
	return func(p *permission) { p.resource = resource }
}

func WithAction(action Action) Option {
	return func(p *permission) { p.action = action }
}

func WithModifier(modifier Modifier) Option {
	return func(p *permission) { p.modifier = modifier }
}

// MustCreate for constants (panic wrapper for invalid permissions)
func MustCreate(id uuid.UUID, name string, resource Resource, action Action, modifier Modifier) Permission {
	p := New(
		WithID(id),
		WithName(name),
		WithResource(resource),
		WithAction(action),
		WithModifier(modifier),
	)
	if !p.IsValid() {
		panic(fmt.Sprintf("invalid permission constant: %s", name))
	}
	return p
}

// Getters
func (p *permission) ID() uuid.UUID {
	return p.id
}

func (p *permission) Name() string {
	return p.name
}

func (p *permission) Resource() Resource {
	return p.resource
}

func (p *permission) Action() Action {
	return p.action
}

func (p *permission) Modifier() Modifier {
	return p.modifier
}

// Business logic methods
func (p *permission) Equals(p2 Permission) bool {
	if p.Modifier() == ModifierAll {
		return p.Resource() == p2.Resource() && p.Action() == p2.Action()
	}
	return p.Resource() == p2.Resource() && p.Action() == p2.Action() && p.Modifier() == p2.Modifier()
}

func (p *permission) Matches(resource Resource, action Action) bool {
	return p.resource == resource && p.action == action
}

func (p *permission) IsValid() bool {
	return p.id != uuid.Nil && p.name != "" && p.resource != "" && p.action != ""
}
