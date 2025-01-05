package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
)

type GormUploadRepository struct{}

func NewUploadRepository() upload.Repository {
	return &GormUploadRepository{}
}

func (g *GormUploadRepository) GetPaginated(
	ctx context.Context, params *upload.FindParams,
) ([]*upload.Upload, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.Hash != "" {
		where, args = append(where, fmt.Sprintf("hash = $%d", len(args)+1)), append(args, params.Hash)
	}

	if params.Type != "" {
		where, args = append(where, fmt.Sprintf("mimetype = $%d", len(args)+1)), append(args, params.Type)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, hash, path, size, mimetype, created_at, updated_at FROM uploads
		WHERE `+strings.Join(where, " AND ")+`
		`+repo.FormatLimitOffset(params.Limit, params.Offset)+`
	`, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uploads := make([]*upload.Upload, 0)
	for rows.Next() {
		var upload models.Upload
		if err := rows.Scan(
			&upload.ID,
			&upload.Hash,
			&upload.Path,
			&upload.Mimetype,
			&upload.CreatedAt,
			&upload.UpdatedAt,
		); err != nil {
			return nil, err
		}
		uploads = append(uploads, ToDomainUpload(&upload))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return uploads, nil
}

func (g *GormUploadRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM uploads
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUploadRepository) GetAll(ctx context.Context) ([]*upload.Upload, error) {
	return g.GetPaginated(ctx, &upload.FindParams{
		Limit: 100000,
	})
}

func (g *GormUploadRepository) GetByID(ctx context.Context, id uint) (*upload.Upload, error) {
	uploads, err := g.GetPaginated(ctx, &upload.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, ErrUploadNotFound
	}
	return uploads[0], nil
}

func (g *GormUploadRepository) GetByHash(ctx context.Context, hash string) (*upload.Upload, error) {
	uploads, err := g.GetPaginated(ctx, &upload.FindParams{
		Hash: hash,
	})
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, ErrUploadNotFound
	}
	return uploads[0], nil
}

func (g *GormUploadRepository) Create(ctx context.Context, data *upload.Upload) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	upload := ToDBUpload(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO uploads (hash, path, size, mimetype) VALUES ($1, $2, $3, $4)
		RETURNING id
	`, upload.Hash, upload.Path, upload.Size, upload.Mimetype).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Update(ctx context.Context, data *upload.Upload) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	upload := ToDBUpload(data)
	if _, err := tx.Exec(ctx, `
		UPDATE uploads 
		SET
		hash = $1
		path = $2
		size = $3
		mimetype = $4
		WHERE id = $5
	`, upload.Hash, upload.Path, upload.Size, upload.Mimetype, upload.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM uploads where id = $1`, id); err != nil {
		return err
	}
	return nil
}
