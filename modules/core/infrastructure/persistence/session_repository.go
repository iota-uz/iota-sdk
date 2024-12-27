package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
	"github.com/iota-agency/iota-sdk/pkg/utils/repo"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

type GormSessionRepository struct{}

func NewSessionRepository() session.Repository {
	return &GormSessionRepository{}
}

func (g *GormSessionRepository) GetPaginated(
	ctx context.Context, params *session.FindParams,
) ([]*session.Session, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.Token != "" {
		where, args = append(where, fmt.Sprintf("token = $%d", len(args)+1)), append(args, params.Token)
	}

	rows, err := pool.Query(ctx, `
		SELECT token, user_id, expires_at, ip, user_agent, created_at FROM sessions
		WHERE `+strings.Join(where, " AND ")+`
		`+repo.FormatLimitOffset(params.Limit, params.Offset)+`
	`, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]*session.Session, 0)
	for rows.Next() {
		var session models.Session
		if err := rows.Scan(
			&session.Token,
			&session.UserID,
			&session.ExpiresAt,
			&session.IP,
			&session.UserAgent,
			&session.CreatedAt,
		); err != nil {
			return nil, err
		}

		domainSession := toDomainSession(&session)
		sessions = append(sessions, domainSession)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (g *GormSessionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM sessions
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormSessionRepository) GetAll(ctx context.Context) ([]*session.Session, error) {
	return g.GetPaginated(ctx, &session.FindParams{
		Limit: 100000,
	})
}

func (g *GormSessionRepository) GetByToken(ctx context.Context, token string) (*session.Session, error) {
	sessions, err := g.GetPaginated(ctx, &session.FindParams{
		Token: token,
	})
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, ErrSessionNotFound
	}
	return sessions[0], nil
}

func (g *GormSessionRepository) Create(ctx context.Context, data *session.Session) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbSession := toDBSession(data)
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
	`, dbSession.Token, dbSession.UserID, dbSession.ExpiresAt, dbSession.IP, dbSession.UserAgent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (g *GormSessionRepository) Update(ctx context.Context, data *session.Session) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	dbSession := toDBSession(data)
	if _, err := tx.Exec(ctx, `
		UPDATE sessions
		SET expires_at = $1, ip = $2, user_agent = $3
		WHERE token = $4
	`, dbSession.ExpiresAt, dbSession.IP, dbSession.UserAgent, dbSession.Token); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (g *GormSessionRepository) Delete(ctx context.Context, token string) error {
	tx, ok := composables.UsePoolTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM sessions WHERE token = $1
	`, token); err != nil {
		return err
	}
	return nil
}
