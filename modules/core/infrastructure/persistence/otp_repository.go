package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	tf "github.com/iota-uz/iota-sdk/pkg/twofactor"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrOTPNotFound = errors.New("OTP not found")
)

const (
	selectOTPQuery = `
        SELECT id, identifier, code_hash, channel, expires_at, used_at, attempts, created_at, tenant_id, user_id
        FROM otps`

	insertOTPQuery = `
        INSERT INTO otps (identifier, code_hash, channel, expires_at, used_at, attempts, created_at, tenant_id, user_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	incrementAttemptsQuery = `
        UPDATE otps
        SET attempts = attempts + 1
        WHERE id = $1 AND tenant_id = $2`

	markOTPUsedQuery = `
        UPDATE otps
        SET used_at = NOW()
        WHERE id = $1 AND tenant_id = $2`

	deleteExpiredQuery = `
        DELETE FROM otps
        WHERE expires_at < NOW() AND used_at IS NULL AND tenant_id = $1`
)

type OTPRepository struct {
	fieldMap map[twofactor.OTPField]string
}

func NewOTPRepository() twofactor.OTPRepository {
	return &OTPRepository{
		fieldMap: map[twofactor.OTPField]string{
			twofactor.OTPFieldExpiresAt: "otps.expires_at",
			twofactor.OTPFieldCreatedAt: "otps.created_at",
			twofactor.OTPFieldAttempts:  "otps.attempts",
		},
	}
}

// Create stores a new OTP entity in the repository
func (o *OTPRepository) Create(ctx context.Context, otp twofactor.OTP) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	_, err = tx.Exec(ctx, insertOTPQuery,
		otp.Identifier(),
		otp.CodeHash(),
		string(otp.Channel()),
		otp.ExpiresAt(),
		otp.UsedAt(),
		otp.Attempts(),
		otp.CreatedAt(),
		tenantID.String(),
		otp.UserID(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to insert OTP")
	}

	return nil
}

// FindByIdentifier retrieves an OTP by its identifier (phone number, email address, etc.)
// Returns the OTP if found, or an error if not found or on query failure
func (o *OTPRepository) FindByIdentifier(ctx context.Context, identifier string) (twofactor.OTP, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	query := repo.Join(
		selectOTPQuery,
		"WHERE identifier = $1 AND used_at IS NULL AND tenant_id = $2 ORDER BY created_at DESC LIMIT 1",
	)

	otps, err := o.queryOTPs(ctx, query, identifier, tenantID.String())
	if err != nil {
		return nil, err
	}

	if len(otps) == 0 {
		return nil, ErrOTPNotFound
	}

	return otps[0], nil
}

// IncrementAttempts increments the failed attempt counter for an OTP
func (o *OTPRepository) IncrementAttempts(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	_, err = tx.Exec(ctx, incrementAttemptsQuery, id, tenantID.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to increment attempts for OTP %d", id))
	}

	return nil
}

// MarkUsed marks an OTP as used by setting the UsedAt timestamp
func (o *OTPRepository) MarkUsed(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	_, err = tx.Exec(ctx, markOTPUsedQuery, id, tenantID.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to mark OTP %d as used", id))
	}

	return nil
}

// DeleteExpired removes all OTP records that have expired
func (o *OTPRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	result, err := tx.Exec(ctx, deleteExpiredQuery, tenantID.String())
	if err != nil {
		return 0, errors.Wrap(err, "failed to delete expired OTPs")
	}

	rowsAffected := result.RowsAffected()
	return rowsAffected, nil
}

// queryOTPs is a helper that executes an OTP query and maps results to domain entities
func (o *OTPRepository) queryOTPs(ctx context.Context, query string, args ...interface{}) ([]twofactor.OTP, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var otps []twofactor.OTP
	for rows.Next() {
		var otp models.OTP
		if err := rows.Scan(
			&otp.ID,
			&otp.Identifier,
			&otp.CodeHash,
			&otp.Channel,
			&otp.ExpiresAt,
			&otp.UsedAt,
			&otp.Attempts,
			&otp.CreatedAt,
			&otp.TenantID,
			&otp.UserID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan OTP row")
		}

		// Map to domain entity
		domainOTP := o.toDomainOTP(&otp)
		otps = append(otps, domainOTP)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return otps, nil
}

// toDomainOTP converts a database model to a domain entity
func (o *OTPRepository) toDomainOTP(dbOTP *models.OTP) twofactor.OTP {
	opts := []twofactor.OTPOption{
		twofactor.WithOTPID(dbOTP.ID),
		twofactor.WithTenantID(dbOTP.ParsedTenantID()),
		twofactor.WithUserID(dbOTP.UserID),
		twofactor.WithAttempts(dbOTP.Attempts),
		twofactor.WithCreatedAt(dbOTP.CreatedAt),
	}

	if dbOTP.UsedAt != nil {
		opts = append(opts, twofactor.WithUsedAt(dbOTP.UsedAt))
	}

	return twofactor.NewOTP(
		dbOTP.Identifier,
		dbOTP.CodeHash,
		tf.OTPChannel(dbOTP.Channel),
		dbOTP.ExpiresAt,
		dbOTP.ParsedTenantID(),
		dbOTP.UserID,
		opts...,
	)
}
