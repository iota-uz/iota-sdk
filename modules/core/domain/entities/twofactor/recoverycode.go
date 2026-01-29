package twofactor

import (
	"time"

	"github.com/google/uuid"
)

// RecoveryCodeOption is a functional option for configuring RecoveryCode
type RecoveryCodeOption func(*recoveryCode)

// --- Option setters ---

func WithRecoveryCodeID(id uint) RecoveryCodeOption {
	return func(r *recoveryCode) {
		r.id = id
	}
}

func WithRecoveryCodeTenantID(tenantID uuid.UUID) RecoveryCodeOption {
	return func(r *recoveryCode) {
		r.tenantID = tenantID
	}
}

func WithRecoveryCodeUsedAt(usedAt *time.Time) RecoveryCodeOption {
	return func(r *recoveryCode) {
		r.usedAt = usedAt
	}
}

func WithRecoveryCodeCreatedAt(createdAt time.Time) RecoveryCodeOption {
	return func(r *recoveryCode) {
		r.createdAt = createdAt
	}
}

// --- Interface ---

// RecoveryCode represents a recovery/backup code for two-factor authentication
type RecoveryCode interface {
	ID() uint
	UserID() uint
	CodeHash() string
	UsedAt() *time.Time
	CreatedAt() time.Time
	TenantID() uuid.UUID

	IsUsed() bool
}

// --- Implementation ---

// NewRecoveryCode creates a new RecoveryCode with required fields
func NewRecoveryCode(userID uint, codeHash string, tenantID uuid.UUID, opts ...RecoveryCodeOption) RecoveryCode {
	r := &recoveryCode{
		userID:    userID,
		codeHash:  codeHash,
		tenantID:  tenantID,
		usedAt:    nil,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type recoveryCode struct {
	id        uint
	userID    uint
	codeHash  string
	usedAt    *time.Time
	createdAt time.Time
	tenantID  uuid.UUID
}

func (r *recoveryCode) ID() uint {
	return r.id
}

func (r *recoveryCode) UserID() uint {
	return r.userID
}

func (r *recoveryCode) CodeHash() string {
	return r.codeHash
}

func (r *recoveryCode) UsedAt() *time.Time {
	return r.usedAt
}

func (r *recoveryCode) CreatedAt() time.Time {
	return r.createdAt
}

func (r *recoveryCode) TenantID() uuid.UUID {
	return r.tenantID
}

func (r *recoveryCode) IsUsed() bool {
	return r.usedAt != nil
}

// --- DTOs ---

// CreateRecoveryCodeDTO represents the data required to create recovery codes
type CreateRecoveryCodeDTO struct {
	UserID    uint
	CodeHashes []string
	TenantID  uuid.UUID
}

// ToEntities converts the DTO to RecoveryCode entities
func (d *CreateRecoveryCodeDTO) ToEntities() []RecoveryCode {
	codes := make([]RecoveryCode, len(d.CodeHashes))
	for i, hash := range d.CodeHashes {
		codes[i] = NewRecoveryCode(d.UserID, hash, d.TenantID)
	}
	return codes
}
