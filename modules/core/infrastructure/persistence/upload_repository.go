package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
)

const (
	selectUploadQuery = `SELECT id, hash, path, name, size, type, mimetype, created_at, updated_at FROM uploads`

	countUploadsQuery = `SELECT COUNT(*) FROM uploads`

	insertUploadQuery = `INSERT INTO uploads (hash, path, name, size, type, mimetype, created_at, updated_at)
                         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
                         RETURNING id`

	updatedUploadQuery = `UPDATE uploads
                          SET hash = $1,
                              path = $2,
                              name = $3,
                              size = $4,
                              type = $5,
                              mimetype = $6,
                              updated_at = $7
                          WHERE id = $8`

	deleteUploadQuery = `DELETE FROM uploads WHERE id = $1`
)

type GormUploadRepository struct {
	fieldMap map[upload.Field]string
}

func NewUploadRepository() upload.Repository {
	return &GormUploadRepository{
		fieldMap: map[upload.Field]string{
			upload.FieldSize:      "size",
			upload.FieldName:      "name",
			upload.FieldCreatedAt: "created_at",
			upload.FieldUpdatedAt: "updated_at",
		},
	}
}

func (g *GormUploadRepository) queryUploads(
	ctx context.Context,
	query string,
	args ...interface{},
) ([]upload.Upload, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uploads := make([]upload.Upload, 0)
	for rows.Next() {
		var dbUpload models.Upload
		if err := rows.Scan(
			&dbUpload.ID,
			&dbUpload.Hash,
			&dbUpload.Path,
			&dbUpload.Name,
			&dbUpload.Size,
			&dbUpload.Type,
			&dbUpload.Mimetype,
			&dbUpload.CreatedAt,
			&dbUpload.UpdatedAt,
		); err != nil {
			return nil, err
		}
		uploads = append(uploads, ToDomainUpload(&dbUpload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return uploads, nil
}

func (g *GormUploadRepository) GetPaginated(
	ctx context.Context, params *upload.FindParams,
) ([]upload.Upload, error) {
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}
	if params.Hash != "" {
		where, args = append(where, fmt.Sprintf("hash = $%d", len(args)+1)), append(args, params.Hash)
	}
	if params.Type != "" {
		where, args = append(where, fmt.Sprintf("type = $%d", len(args)+1)), append(args, params.Type.String())
	}
	if params.Mimetype != nil {
		where, args = append(where, fmt.Sprintf("mimetype = $%d", len(args)+1)), append(args, params.Mimetype.String())
	}

	query := repo.Join(
		selectUploadQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryUploads(ctx, query, args...)
}

func (g *GormUploadRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, countUploadsQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUploadRepository) GetAll(ctx context.Context) ([]upload.Upload, error) {
	return g.queryUploads(ctx, selectUploadQuery)
}

func (g *GormUploadRepository) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
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

func (g *GormUploadRepository) GetByHash(ctx context.Context, hash string) (upload.Upload, error) {
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

func (g *GormUploadRepository) Create(ctx context.Context, data upload.Upload) (upload.Upload, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbUpload := ToDBUpload(data)
	if err := tx.QueryRow(
		ctx,
		insertUploadQuery,
		dbUpload.Hash,
		dbUpload.Path,
		dbUpload.Name,
		dbUpload.Size,
		dbUpload.Type,
		dbUpload.Mimetype,
		dbUpload.CreatedAt,
		dbUpload.UpdatedAt,
	).Scan(&dbUpload.ID); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, dbUpload.ID)
}

func (g *GormUploadRepository) Update(ctx context.Context, data upload.Upload) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	dbUpload := ToDBUpload(data)
	if _, err := tx.Exec(
		ctx,
		updatedUploadQuery,
		dbUpload.Hash,
		dbUpload.Path,
		dbUpload.Name,
		dbUpload.Size,
		dbUpload.Type,
		dbUpload.Mimetype,
		dbUpload.UpdatedAt,
		dbUpload.ID,
	); err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteUploadQuery, id); err != nil {
		return err
	}
	return nil
}
