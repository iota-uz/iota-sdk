package twofactor

import (
	"time"

	"github.com/google/uuid"
	tf "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// Option is a functional option for configuring OTP
type OTPOption func(*otp)

// --- Option setters ---

func WithOTPID(id uint) OTPOption {
	return func(o *otp) {
		o.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) OTPOption {
	return func(o *otp) {
		o.tenantID = tenantID
	}
}

func WithUserID(userID uint) OTPOption {
	return func(o *otp) {
		o.userID = userID
	}
}

func WithUsedAt(usedAt *time.Time) OTPOption {
	return func(o *otp) {
		o.usedAt = usedAt
	}
}

func WithAttempts(attempts int) OTPOption {
	return func(o *otp) {
		o.attempts = attempts
	}
}

func WithCreatedAt(createdAt time.Time) OTPOption {
	return func(o *otp) {
		o.createdAt = createdAt
	}
}

// --- Interface ---

// OTP represents a one-time password entity
type OTP interface {
	ID() uint
	Identifier() string
	CodeHash() string
	Channel() tf.OTPChannel
	ExpiresAt() time.Time
	UsedAt() *time.Time
	Attempts() int
	CreatedAt() time.Time
	TenantID() uuid.UUID
	UserID() uint

	IsExpired() bool
	IsUsed() bool
}

// --- Implementation ---

// NewOTP creates a new OTP with required fields
func NewOTP(identifier, codeHash string, channel tf.OTPChannel, expiresAt time.Time, tenantID uuid.UUID, userID uint, opts ...OTPOption) OTP {
	o := &otp{
		identifier: identifier,
		codeHash:   codeHash,
		channel:    channel,
		expiresAt:  expiresAt,
		tenantID:   tenantID,
		userID:     userID,
		attempts:   0,
		usedAt:     nil,
		createdAt:  time.Now(),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type otp struct {
	id         uint
	identifier string
	codeHash   string
	channel    tf.OTPChannel
	expiresAt  time.Time
	usedAt     *time.Time
	attempts   int
	createdAt  time.Time
	tenantID   uuid.UUID
	userID     uint
}

func (o *otp) ID() uint {
	return o.id
}

func (o *otp) Identifier() string {
	return o.identifier
}

func (o *otp) CodeHash() string {
	return o.codeHash
}

func (o *otp) Channel() tf.OTPChannel {
	return o.channel
}

func (o *otp) ExpiresAt() time.Time {
	return o.expiresAt
}

func (o *otp) UsedAt() *time.Time {
	return o.usedAt
}

func (o *otp) Attempts() int {
	return o.attempts
}

func (o *otp) CreatedAt() time.Time {
	return o.createdAt
}

func (o *otp) TenantID() uuid.UUID {
	return o.tenantID
}

func (o *otp) UserID() uint {
	return o.userID
}

func (o *otp) IsExpired() bool {
	return o.expiresAt.Before(time.Now())
}

func (o *otp) IsUsed() bool {
	return o.usedAt != nil
}

// --- DTOs ---

// CreateOTPDTO represents the data required to create an OTP
type CreateOTPDTO struct {
	Identifier string
	CodeHash   string
	Channel    tf.OTPChannel
	ExpiresAt  time.Time
	TenantID   uuid.UUID
	UserID     uint
}

// ToEntity converts the DTO to an OTP entity
func (d *CreateOTPDTO) ToEntity() OTP {
	return NewOTP(d.Identifier, d.CodeHash, d.Channel, d.ExpiresAt, d.TenantID, d.UserID)
}
