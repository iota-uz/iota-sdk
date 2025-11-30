package project

import (
	"time"
)

// Option is a functional option for configuring Project
type Option func(*project)

// --- Option setters ---

func WithID(id uint) Option {
	return func(p *project) {
		p.id = id
	}
}

func WithDescription(description string) Option {
	return func(p *project) {
		p.description = description
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *project) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *project) {
		p.updatedAt = updatedAt
	}
}

// --- Interface ---

// Project represents a project entity
type Project interface {
	ID() uint
	Name() string
	Description() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetName(name string) Project
	SetDescription(description string) Project
}

// --- Implementation ---

// New creates a new Project with required fields
func New(name string, opts ...Option) Project {
	p := &project{
		name:      name,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type project struct {
	id          uint
	name        string
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

func (p *project) ID() uint {
	return p.id
}

func (p *project) Name() string {
	return p.name
}

func (p *project) Description() string {
	return p.description
}

func (p *project) CreatedAt() time.Time {
	return p.createdAt
}

func (p *project) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *project) SetName(name string) Project {
	result := *p
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (p *project) SetDescription(description string) Project {
	result := *p
	result.description = description
	result.updatedAt = time.Now()
	return &result
}
