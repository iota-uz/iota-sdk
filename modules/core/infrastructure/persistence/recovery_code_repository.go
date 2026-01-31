package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrRecoveryCodeNotFound = errors.New("recovery code not found")
)

const (
	selectRecoveryCodeQuery = `
        SELECT id, user_id, code_hash, used_at, created_at, tenant_id
        FROM recovery_codes`

	insertRecoveryCodeQuery = `
        INSERT INTO recovery_codes (user_id, code_hash, created_at, tenant_id)
        VALUES`

	markRecoveryCodeUsedQuery = `
        UPDATE recovery_codes
        SET used_at = NOW()
        WHERE id = $1 AND tenant_id = $2`

	deleteRecoveryCodesQuery = `
        DELETE FROM recovery_codes
        WHERE user_id = $1 AND tenant_id = $2`

	countRemainingRecoveryCodesQuery = `
        SELECT COUNT(*)
        FROM recovery_codes
        WHERE user_id = $1 AND tenant_id = $2 AND used_at IS NULL`
)

type RecoveryCodeRepository struct {
	fieldMap map[twofactor.RecoveryCodeField]string
}

func NewRecoveryCodeRepository() twofactor.RecoveryCodeRepository {
	return &RecoveryCodeRepository{
		fieldMap: map[twofactor.RecoveryCodeField]string{
			twofactor.RecoveryCodeFieldCreatedAt: "recovery_codes.created_at",
			twofactor.RecoveryCodeFieldUsedAt:    "recovery_codes.used_at",
		},
	}
}

// Create stores new recovery code entities in the repository
// The codeHashes slice contains pre-hashed recovery codes for security
func (r *RecoveryCodeRepository) Create(ctx context.Context, userID uint, codeHashes []string) error {
	if len(codeHashes) == 0 {
		return nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	// Build batch insert values
	values := make([][]interface{}, 0, len(codeHashes))
	for _, hash := range codeHashes {
		values = append(values, []interface{}{userID, hash, time.Now(), tenantID.String()})
	}

	query, args := repo.BatchInsertQueryN(insertRecoveryCodeQuery, values)
	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to insert recovery codes")
	}

	return nil
}

// FindUnused retrieves all unused recovery codes for a given user
// Recovery codes are considered unused if UsedAt is nil
// Returns a slice of recovery codes and an error on failure
func (r *RecoveryCodeRepository) FindUnused(ctx context.Context, userID uint) ([]twofactor.RecoveryCode, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	query := repo.Join(
		selectRecoveryCodeQuery,
		"WHERE user_id = $1 AND tenant_id = $2 AND used_at IS NULL ORDER BY created_at ASC",
	)

	return r.queryRecoveryCodes(ctx, query, userID, tenantID.String())
}

// MarkUsed marks a recovery code as used by setting the UsedAt timestamp
// This should be called when a recovery code has been successfully used for authentication
func (r *RecoveryCodeRepository) MarkUsed(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	_, err = tx.Exec(ctx, markRecoveryCodeUsedQuery, id, tenantID.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to mark recovery code %d as used", id))
	}

	return nil
}

// DeleteAll removes all recovery codes associated with a given user
// This is useful when regenerating recovery codes or disabling 2FA
func (r *RecoveryCodeRepository) DeleteAll(ctx context.Context, userID uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	_, err = tx.Exec(ctx, deleteRecoveryCodesQuery, userID, tenantID.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete recovery codes for user %d", userID))
	}

	return nil
}

// CountRemaining returns the count of unused recovery codes for a given user
// This can be used to determine if the user should be prompted to generate new codes
func (r *RecoveryCodeRepository) CountRemaining(ctx context.Context, userID uint) (int, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	var count int
	err = tx.QueryRow(ctx, countRemainingRecoveryCodesQuery, userID, tenantID.String()).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("failed to count remaining recovery codes for user %d", userID))
	}

	return count, nil
}

// queryRecoveryCodes is a helper that executes a recovery code query and maps results to domain entities
func (r *RecoveryCodeRepository) queryRecoveryCodes(ctx context.Context, query string, args ...interface{}) ([]twofactor.RecoveryCode, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var codes []twofactor.RecoveryCode
	for rows.Next() {
		var rc models.RecoveryCode
		if err := rows.Scan(
			&rc.ID,
			&rc.UserID,
			&rc.CodeHash,
			&rc.UsedAt,
			&rc.CreatedAt,
			&rc.TenantID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan recovery code row")
		}

		// Map to domain entity
		domainCode := r.toDomainRecoveryCode(&rc)
		codes = append(codes, domainCode)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return codes, nil
}

// toDomainRecoveryCode converts a database model to a domain entity
func (r *RecoveryCodeRepository) toDomainRecoveryCode(dbCode *models.RecoveryCode) twofactor.RecoveryCode {
	opts := []twofactor.RecoveryCodeOption{
		twofactor.WithRecoveryCodeID(dbCode.ID),
		twofactor.WithRecoveryCodeCreatedAt(dbCode.CreatedAt),
		twofactor.WithRecoveryCodeTenantID(dbCode.ParsedTenantID()),
	}

	if dbCode.UsedAt != nil {
		opts = append(opts, twofactor.WithRecoveryCodeUsedAt(dbCode.UsedAt))
	}

	return twofactor.NewRecoveryCode(dbCode.UserID, dbCode.CodeHash, dbCode.ParsedTenantID(), opts...)
}
