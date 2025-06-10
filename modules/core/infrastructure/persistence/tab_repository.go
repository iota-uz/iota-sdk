package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrTabNotFound = errors.New("tab not found")
)

const (
	selectTabsQuery     = `SELECT id, href, user_id, position, tenant_id FROM tabs`
	countTabsQuery      = `SELECT COUNT(*) as count FROM tabs`
	insertTabsQuery     = `INSERT INTO tabs (href, user_id, position, tenant_id) VALUES ($1, $2, $3, $4) RETURNING id`
	updateTabsQuery     = `UPDATE tabs SET href = $1, position = $2 WHERE id = $3`
	deleteTabsQuery     = `DELETE FROM tabs WHERE id = $1`
	deleteUserTabsQuery = `DELETE FROM tabs WHERE user_id = $1`
)

type tabRepository struct{}

func NewTabRepository() tab.Repository {
	return &tabRepository{}
}

func (g *tabRepository) queryTabs(ctx context.Context, query string, args ...interface{}) ([]*tab.Tab, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, query, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tabs := make([]*tab.Tab, 0)
	for rows.Next() {
		var tab models.Tab
		if err := rows.Scan(
			&tab.ID,
			&tab.Href,
			&tab.UserID,
			&tab.Position,
			&tab.TenantID,
		); err != nil {
			return nil, err
		}

		domainTab, err := ToDomainTab(&tab)
		if err != nil {
			return nil, err
		}
		tabs = append(tabs, domainTab)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tabs, nil
}

func (g *tabRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	var count int64
	if err := pool.QueryRow(ctx, countTabsQuery+" WHERE tenant_id = $1", tenant.ID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *tabRepository) GetAll(ctx context.Context, params *tab.FindParams) ([]*tab.Tab, error) {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where, args := []string{"tenant_id = $1"}, []interface{}{tenant.ID}
	if params.UserID != 0 {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, params.UserID)
	}

	return g.queryTabs(ctx, repo.Join(selectTabsQuery, repo.JoinWhere(where...)), args...)
}

func (g *tabRepository) GetUserTabs(ctx context.Context, userID uint) ([]*tab.Tab, error) {
	tabs, err := g.GetAll(ctx, &tab.FindParams{
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return tabs, nil
}

func (g *tabRepository) GetByID(ctx context.Context, id uint) (*tab.Tab, error) {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tabs, err := g.queryTabs(ctx, repo.Join(selectTabsQuery, "WHERE id = $1 AND tenant_id = $2"), id, tenant.ID)
	if err != nil {
		return nil, err
	}
	if len(tabs) == 0 {
		return nil, ErrTabNotFound
	}
	return tabs[0], nil
}

func (g *tabRepository) Create(ctx context.Context, data *tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tab := ToDBTab(data)

	if err := tx.QueryRow(
		ctx,
		insertTabsQuery,
		tab.Href,
		tab.UserID,
		tab.Position,
		tab.TenantID,
	).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *tabRepository) CreateMany(ctx context.Context, tabs []*tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	for _, data := range tabs {
		tab := ToDBTab(data)

		if err := tx.QueryRow(
			ctx,
			insertTabsQuery,
			tab.Href,
			tab.UserID,
			tab.Position,
			tab.TenantID,
		).Scan(&data.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *tabRepository) CreateOrUpdate(ctx context.Context, data *tab.Tab) error {
	matches, err := g.queryTabs(
		ctx,
		selectTabsQuery+" WHERE user_id = $1 AND href = $2",
		data.UserID,
		data.Href,
	)
	if err != nil {
		return err
	}
	if len(matches) > 1 {
		return errors.New("multiple tabs found")
	}
	if len(matches) == 1 {
		data.ID = matches[0].ID
		return g.Update(ctx, data)
	}
	return g.Create(ctx, data)
}

func (g *tabRepository) Update(ctx context.Context, data *tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tab := ToDBTab(data)
	if _, err := tx.Exec(
		ctx,
		updateTabsQuery,
		tab.Href,
		tab.Position,
		tab.ID,
	); err != nil {
		return err
	}
	return nil
}

func (g *tabRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, deleteTabsQuery, id); err != nil {
		return err
	}
	return nil
}

func (g *tabRepository) DeleteUserTabs(ctx context.Context, userID uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, deleteUserTabsQuery, userID); err != nil {
		return err
	}
	return nil
}
