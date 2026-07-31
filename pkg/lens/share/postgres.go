package share

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) ListViews(ctx context.Context, access Access, dashboardID string) ([]SavedView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, dashboard_id, name, scope, owner_user_id, state_url,
		       default_role_id, created_at, updated_at
		FROM lens_saved_views
		WHERE tenant_id = $1 AND dashboard_id = $2
		  AND (scope = 'team' OR owner_user_id = $3)
		ORDER BY default_role_id NULLS LAST, lower(name), id`, access.TenantID, dashboardID, int64(access.UserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	views := make([]SavedView, 0)
	for rows.Next() {
		view, scanErr := scanView(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		views = append(views, view)
	}
	return views, rows.Err()
}

func (s *PostgresStore) PutView(ctx context.Context, access Access, view SavedView) (SavedView, error) {
	view.TenantID = access.TenantID
	view.OwnerUserID = access.UserID
	if view.Scope == ViewScopeTeam && !access.ManageTeam || view.DefaultRoleID != nil && !access.ManageTeam {
		return SavedView{}, ErrForbidden
	}
	if view.DefaultRoleID != nil && !canAssignDefaultRole(access, *view.DefaultRoleID) {
		return SavedView{}, ErrForbidden
	}
	if err := view.Validate(); err != nil {
		return SavedView{}, invalidInput(err)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SavedView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if view.ID == uuid.Nil {
		view.ID = uuid.New()
	} else {
		var tenantID uuid.UUID
		var ownerID int64
		var scope ViewScope
		var dashboardID string
		if err := tx.QueryRow(ctx, `SELECT tenant_id, owner_user_id, scope, dashboard_id FROM lens_saved_views WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, view.ID, access.TenantID).
			Scan(&tenantID, &ownerID, &scope, &dashboardID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SavedView{}, ErrNotFound
			}
			return SavedView{}, err
		}
		if tenantID != access.TenantID ||
			uint(ownerID) != access.UserID && (scope != ViewScopeTeam || !access.ManageTeam) {
			return SavedView{}, ErrNotFound
		}
		if scope == ViewScopeTeam && !access.ManageTeam {
			return SavedView{}, ErrForbidden
		}
		if dashboardID != view.DashboardID {
			return SavedView{}, invalidInput(errors.New("saved view dashboard cannot be changed"))
		}
	}
	if view.DefaultRoleID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE lens_saved_views SET default_role_id = NULL, updated_at = now()
			WHERE tenant_id = $1 AND dashboard_id = $2 AND default_role_id = $3 AND id <> $4`,
			view.TenantID, view.DashboardID, int64(*view.DefaultRoleID), view.ID)
		if err != nil {
			return SavedView{}, err
		}
	}
	if view.Scope == ViewScopePersonal {
		if _, err := tx.Exec(ctx, `
			DELETE FROM lens_export_schedules
			WHERE tenant_id = $1 AND dashboard_id = $2 AND view_id = $3 AND owner_user_id <> $4`,
			view.TenantID, view.DashboardID, view.ID, int64(view.OwnerUserID)); err != nil {
			return SavedView{}, err
		}
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO lens_saved_views
			(id, tenant_id, dashboard_id, name, scope, owner_user_id, state_url, default_role_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			dashboard_id = EXCLUDED.dashboard_id, name = EXCLUDED.name, scope = EXCLUDED.scope,
			owner_user_id = EXCLUDED.owner_user_id, state_url = EXCLUDED.state_url,
			default_role_id = EXCLUDED.default_role_id, updated_at = now()
		RETURNING id, tenant_id, dashboard_id, name, scope, owner_user_id, state_url,
		          default_role_id, created_at, updated_at`,
		view.ID, view.TenantID, view.DashboardID, view.Name, view.Scope, int64(view.OwnerUserID), view.StateURL, uintPointerInt64(view.DefaultRoleID))
	stored, err := scanView(row)
	if err != nil {
		return SavedView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SavedView{}, err
	}
	return stored, nil
}

func (s *PostgresStore) DeleteView(ctx context.Context, access Access, id uuid.UUID) error {
	command, err := s.pool.Exec(ctx, `
		DELETE FROM lens_saved_views
		WHERE id = $1 AND tenant_id = $2 AND (owner_user_id = $3 OR (scope = 'team' AND $4))`,
		id, access.TenantID, int64(access.UserID), access.ManageTeam)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListSchedules(ctx context.Context, access Access, dashboardID string) ([]ExportSchedule, error) {
	if !access.ScheduleMail {
		return nil, ErrForbidden
	}
	rows, err := s.pool.Query(ctx, `
		SELECT schedules.id, schedules.tenant_id, schedules.owner_user_id, schedules.dashboard_id,
		       schedules.view_id, views.state_url, schedules.name, schedules.cron_expr,
		       schedules.timezone, schedules.recipients, schedules.enabled, schedules.next_run_at,
		       schedules.last_run_at, schedules.last_error, schedules.created_at, schedules.updated_at
		FROM lens_export_schedules AS schedules
		JOIN lens_saved_views AS views
		  ON views.id = schedules.view_id
		 AND views.tenant_id = schedules.tenant_id
		 AND views.dashboard_id = schedules.dashboard_id
		WHERE schedules.tenant_id = $1 AND schedules.owner_user_id = $2 AND schedules.dashboard_id = $3
		  AND (views.scope = 'team' OR views.owner_user_id = schedules.owner_user_id)
		ORDER BY schedules.next_run_at NULLS LAST, lower(schedules.name), schedules.id`, access.TenantID, int64(access.UserID), dashboardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schedules := make([]ExportSchedule, 0)
	for rows.Next() {
		schedule, scanErr := scanSchedule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
}

func (s *PostgresStore) PutSchedule(ctx context.Context, access Access, schedule ExportSchedule) (ExportSchedule, error) {
	if !access.ScheduleMail {
		return ExportSchedule{}, ErrForbidden
	}
	schedule.TenantID = access.TenantID
	schedule.OwnerUserID = access.UserID
	if err := schedule.Validate(); err != nil {
		return ExportSchedule{}, invalidInput(err)
	}
	if schedule.ID != uuid.Nil {
		var tenantID uuid.UUID
		var ownerID int64
		var dashboardID string
		if err := s.pool.QueryRow(ctx, `SELECT tenant_id, owner_user_id, dashboard_id FROM lens_export_schedules WHERE id = $1 AND tenant_id = $2`, schedule.ID, access.TenantID).
			Scan(&tenantID, &ownerID, &dashboardID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ExportSchedule{}, ErrNotFound
			}
			return ExportSchedule{}, err
		}
		if tenantID != access.TenantID || uint(ownerID) != access.UserID {
			return ExportSchedule{}, ErrNotFound
		}
		if dashboardID != schedule.DashboardID {
			return ExportSchedule{}, invalidInput(errors.New("schedule dashboard cannot be changed"))
		}
	}
	var stateURL string
	if err := s.pool.QueryRow(ctx, `
		SELECT state_url FROM lens_saved_views
			WHERE id = $1 AND tenant_id = $2 AND dashboard_id = $3
			  AND (scope = 'team' OR owner_user_id = $4)`,
		schedule.ViewID, access.TenantID, schedule.DashboardID, int64(access.UserID)).Scan(&stateURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExportSchedule{}, ErrNotFound
		}
		return ExportSchedule{}, err
	}
	schedule.StateURL = stateURL
	now := time.Now().UTC()
	if schedule.Enabled {
		schedule.NextRunAt, _ = schedule.Next(now)
	} else {
		schedule.NextRunAt = time.Time{}
	}
	if schedule.ID == uuid.Nil {
		schedule.ID = uuid.New()
	}
	stored, err := scanSchedule(s.pool.QueryRow(ctx, `
		INSERT INTO lens_export_schedules
			(id, tenant_id, owner_user_id, dashboard_id, view_id, name, cron_expr, timezone, recipients, enabled, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			dashboard_id = EXCLUDED.dashboard_id, view_id = EXCLUDED.view_id,
			name = EXCLUDED.name, cron_expr = EXCLUDED.cron_expr,
			timezone = EXCLUDED.timezone, recipients = EXCLUDED.recipients,
			enabled = EXCLUDED.enabled, next_run_at = EXCLUDED.next_run_at, updated_at = now()
		RETURNING id, tenant_id, owner_user_id, dashboard_id, view_id, ''::text, name, cron_expr,
		          timezone, recipients, enabled, next_run_at, last_run_at, last_error, created_at, updated_at`,
		schedule.ID, schedule.TenantID, int64(schedule.OwnerUserID), schedule.DashboardID, schedule.ViewID,
		schedule.Name, schedule.Cron, schedule.Timezone, schedule.Recipients, schedule.Enabled, nullableTime(schedule.NextRunAt)))
	if err != nil {
		return ExportSchedule{}, err
	}
	stored.StateURL = stateURL
	return stored, nil
}

func (s *PostgresStore) DeleteSchedule(ctx context.Context, access Access, id uuid.UUID) error {
	if !access.ScheduleMail {
		return ErrForbidden
	}
	command, err := s.pool.Exec(ctx, `
		DELETE FROM lens_export_schedules WHERE id = $1 AND tenant_id = $2 AND owner_user_id = $3`,
		id, access.TenantID, int64(access.UserID))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DueSchedules(ctx context.Context, now time.Time, limit int) ([]ExportSchedule, error) {
	if limit <= 0 {
		return nil, nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		WITH due AS (
			SELECT schedules.id, schedules.tenant_id, views.state_url
			FROM lens_export_schedules AS schedules
			JOIN lens_saved_views AS views
			  ON views.id = schedules.view_id
			 AND views.tenant_id = schedules.tenant_id
			 AND views.dashboard_id = schedules.dashboard_id
			WHERE schedules.enabled = TRUE AND schedules.next_run_at <= $1
			  AND (views.scope = 'team' OR views.owner_user_id = schedules.owner_user_id)
			ORDER BY schedules.next_run_at, schedules.id
			FOR UPDATE OF schedules SKIP LOCKED
			LIMIT $2
		)
		UPDATE lens_export_schedules AS schedules
		SET next_run_at = $3, updated_at = now()
		FROM due
		WHERE schedules.id = due.id AND schedules.tenant_id = due.tenant_id
		RETURNING schedules.id, schedules.tenant_id, schedules.owner_user_id, schedules.dashboard_id,
		          schedules.view_id, due.state_url, schedules.name, schedules.cron_expr, schedules.timezone, schedules.recipients,
		          schedules.enabled, schedules.next_run_at, schedules.last_run_at, schedules.last_error,
		          schedules.created_at, schedules.updated_at`,
		now.UTC(), limit, now.UTC().Add(scheduleClaimLease))
	if err != nil {
		return nil, err
	}
	schedules := make([]ExportSchedule, 0)
	for rows.Next() {
		schedule, scanErr := scanSchedule(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		schedules = append(schedules, schedule)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return schedules, nil
}

func (s *PostgresStore) FinishSchedule(
	ctx context.Context,
	tenantID uuid.UUID,
	id uuid.UUID,
	claimedUntil time.Time,
	ranAt time.Time,
	nextRun time.Time,
	runErr error,
) error {
	lastError := ""
	if runErr != nil {
		lastError = runErr.Error()
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE lens_export_schedules
		SET last_run_at = $4, next_run_at = $5, last_error = $6, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND next_run_at = $3`,
		id, tenantID, claimedUntil.UTC(), ranAt.UTC(), nextRun.UTC(), lastError)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrClaimLost
	}
	return nil
}

type scanner interface {
	Scan(...any) error
}

func scanView(row scanner) (SavedView, error) {
	var view SavedView
	var ownerID int64
	var defaultRoleID *int64
	if err := row.Scan(&view.ID, &view.TenantID, &view.DashboardID, &view.Name, &view.Scope, &ownerID,
		&view.StateURL, &defaultRoleID, &view.CreatedAt, &view.UpdatedAt); err != nil {
		return SavedView{}, err
	}
	view.OwnerUserID = uint(ownerID)
	if defaultRoleID != nil {
		value := uint(*defaultRoleID)
		view.DefaultRoleID = &value
	}
	return view, nil
}

func scanSchedule(row scanner) (ExportSchedule, error) {
	var schedule ExportSchedule
	var ownerID int64
	var nextRunAt, lastRunAt *time.Time
	if err := row.Scan(&schedule.ID, &schedule.TenantID, &ownerID, &schedule.DashboardID, &schedule.ViewID, &schedule.StateURL,
		&schedule.Name, &schedule.Cron, &schedule.Timezone, &schedule.Recipients, &schedule.Enabled,
		&nextRunAt, &lastRunAt, &schedule.LastError, &schedule.CreatedAt, &schedule.UpdatedAt); err != nil {
		return ExportSchedule{}, err
	}
	schedule.OwnerUserID = uint(ownerID)
	if nextRunAt != nil {
		schedule.NextRunAt = nextRunAt.UTC()
	}
	if lastRunAt != nil {
		schedule.LastRunAt = lastRunAt.UTC()
	}
	return schedule, nil
}

func uintPointerInt64(value *uint) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
