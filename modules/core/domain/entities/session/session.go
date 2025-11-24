package session

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// Option is a functional option for configuring Session
type Option func(*session)

// --- Option setters ---

func WithExpiresAt(expiresAt time.Time) Option {
	return func(s *session) {
		s.expiresAt = expiresAt
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(s *session) {
		s.createdAt = createdAt
	}
}

// --- Interface ---

// Session represents a user session
type Session interface {
	Token() string
	UserID() uint
	TenantID() uuid.UUID
	IP() string
	UserAgent() string
	ExpiresAt() time.Time
	CreatedAt() time.Time

	IsExpired() bool
}

// --- Implementation ---

// New creates a new Session with required fields
func New(token string, userID uint, tenantID uuid.UUID, ip, userAgent string, opts ...Option) Session {
	s := &session{
		token:     token,
		userID:    userID,
		tenantID:  tenantID,
		ip:        ip,
		userAgent: userAgent,
		expiresAt: time.Now().Add(configuration.Use().SessionDuration),
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type session struct {
	token     string
	userID    uint
	tenantID  uuid.UUID
	ip        string
	userAgent string
	expiresAt time.Time
	createdAt time.Time
}

func (s *session) Token() string {
	return s.token
}

func (s *session) UserID() uint {
	return s.userID
}

func (s *session) TenantID() uuid.UUID {
	return s.tenantID
}

func (s *session) IP() string {
	return s.ip
}

func (s *session) UserAgent() string {
	return s.userAgent
}

func (s *session) ExpiresAt() time.Time {
	return s.expiresAt
}

func (s *session) CreatedAt() time.Time {
	return s.createdAt
}

func (s *session) IsExpired() bool {
	return s.expiresAt.Before(time.Now())
}

// --- DTOs ---

type CreateDTO struct {
	Token     string
	UserID    uint
	TenantID  uuid.UUID
	IP        string
	UserAgent string
}

func (d *CreateDTO) ToEntity() Session {
	return New(d.Token, d.UserID, d.TenantID, d.IP, d.UserAgent)
}
