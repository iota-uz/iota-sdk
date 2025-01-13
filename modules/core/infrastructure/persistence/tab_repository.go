package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrTabNotFound = errors.New("tab not found")
)

type GormTabRepository struct{}

func NewTabRepository() tab.Repository {
	return &GormTabRepository{}
}

func (g *GormTabRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM tabs
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormTabRepository) GetAll(ctx context.Context, params *tab.FindParams) ([]*tab.Tab, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.UserID != 0 {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, params.UserID)
	}

	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, href, user_id, position FROM tabs
		WHERE `+strings.Join(where, " AND "), args...)

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

func (g *GormTabRepository) GetUserTabs(ctx context.Context, userID uint) ([]*tab.Tab, error) {
	tabs, err := g.GetAll(ctx, &tab.FindParams{
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return tabs, nil
}

func (g *GormTabRepository) GetByID(ctx context.Context, id uint) (*tab.Tab, error) {
	tabs, err := g.GetAll(ctx, &tab.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(tabs) == 0 {
		return nil, ErrTabNotFound
	}
	return tabs[0], nil
}

func (g *GormTabRepository) Create(ctx context.Context, data *tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	tab := ToDBTab(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO tabs (href, user_id, position) VALUES ($1, $2, $3) RETURNING id
	`, tab.Href, tab.UserID, tab.Position).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormTabRepository) CreateMany(ctx context.Context, tabs []*tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	for _, data := range tabs {
		tab := ToDBTab(data)
		if err := tx.QueryRow(ctx, `
		INSERT INTO tabs (href, user_id, position) VALUES ($1, $2, $3) RETURNING id
	`, tab.Href, tab.UserID, tab.Position).Scan(&data.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormTabRepository) CreateOrUpdate(ctx context.Context, data *tab.Tab) error {
	u, err := g.GetByID(ctx, data.ID)
	if err != nil && !errors.Is(err, ErrTabNotFound) {
		return err
	}
	if u != nil {
		if err := g.Update(ctx, data); err != nil {
			return err
		}
	} else {
		if err := g.Create(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func (g *GormTabRepository) Update(ctx context.Context, data *tab.Tab) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	tab := ToDBTab(data)
	if _, err := tx.Exec(ctx, `
		UPDATE tabs
		SET href = $1, position = $2
		WHERE id = $3
	`, tab.Href, tab.Position, tab.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormTabRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tabs where id = $1`, id); err != nil {
		return err
	}
	return nil
}

func (g *GormTabRepository) DeleteUserTabs(ctx context.Context, userID uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tabs where user_id = $1`, userID); err != nil {
		return err
	}
	return nil
}
