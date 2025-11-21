package authlog

import (
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for configuring AuthLog
type Option func(*authLog)

// --- Option setters ---

func WithID(id uint) Option {
	return func(a *authLog) {
		a.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(a *authLog) {
		a.tenantID = tenantID
	}
}

func WithUserID(userID uint) Option {
	return func(a *authLog) {
		a.userID = userID
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(a *authLog) {
		a.createdAt = createdAt
	}
}

func WithIP(ip string) Option {
	return func(a *authLog) {
		a.ip = ip
	}
}

func WithUserAgent(userAgent string) Option {
	return func(a *authLog) {
		a.userAgent = userAgent
	}
}

// --- Interface ---

// AuthLog represents an authentication log entry
type AuthLog interface {
	ID() uint
	TenantID() uuid.UUID
	UserID() uint
	IP() string
	UserAgent() string
	CreatedAt() time.Time
}

// --- Implementation ---

// New creates a new AuthLog with required fields
func New(ip, userAgent string, opts ...Option) AuthLog {
	a := &authLog{
		ip:        ip,
		userAgent: userAgent,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type authLog struct {
	id        uint
	tenantID  uuid.UUID
	userID    uint
	ip        string
	userAgent string
	createdAt time.Time
}

func (a *authLog) ID() uint {
	return a.id
}

func (a *authLog) TenantID() uuid.UUID {
	return a.tenantID
}

func (a *authLog) UserID() uint {
	return a.userID
}

func (a *authLog) IP() string {
	return a.ip
}

func (a *authLog) UserAgent() string {
	return a.userAgent
}

func (a *authLog) CreatedAt() time.Time {
	return a.createdAt
}

// AuthenticationLog is deprecated, use AuthLog interface instead
// Kept for backward compatibility
type AuthenticationLog struct {
	ID        uint
	TenantID  uuid.UUID
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}
