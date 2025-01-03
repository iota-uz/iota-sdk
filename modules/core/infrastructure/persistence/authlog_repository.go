package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrAuthlogNotFound = errors.New("auth log not found")
)

type GormAuthLogRepository struct{}

func NewAuthLogRepository() authlog.Repository {
	return &GormAuthLogRepository{}
}

func (g *GormAuthLogRepository) GetPaginated(
	ctx context.Context, params *authlog.FindParams,
) ([]*authlog.AuthenticationLog, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = ANY($%d)", len(args)+1)), append(args, params.ID)
	}

	if params.UserID != 0 {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, params.UserID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, user_id, ip, user_agent, created_at
		FROM authentication_logs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]*authlog.AuthenticationLog, 0)
	for rows.Next() {
		var log models.AuthenticationLog
		if err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.IP,
			&log.UserAgent,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, toDomainAuthenticationLog(&log))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func (g *GormAuthLogRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM authentication_logs
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormAuthLogRepository) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error) {
	return g.GetPaginated(ctx, &authlog.FindParams{
		Limit: 100000,
	})
}

func (g *GormAuthLogRepository) GetByID(ctx context.Context, id uint) (*authlog.AuthenticationLog, error) {
	logs, err := g.GetPaginated(ctx, &authlog.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return nil, ErrAuthlogNotFound
	}
	return logs[0], nil
}

func (g *GormAuthLogRepository) Create(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBAuthenticationLog(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO authentication_logs (user_id, ip, user_agent) VALUES ($1, $2, $3)
	`, dbRow.UserID, dbRow.IP, dbRow.UserAgent).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Update(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbRow := toDBAuthenticationLog(data)
	if _, err := tx.Exec(ctx, `
		UPDATE authentication_logs
		SET ip = $1, user_agent = $2
		WHERE id = $3
	`, dbRow.IP, dbRow.UserAgent, dbRow.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM authentication_logs WHERE id = $1
	`, id); err != nil {
		return err
	}
	return nil
}
