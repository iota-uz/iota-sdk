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
	selectUploadQuery = `SELECT id, hash, slug, path, name, size, type, source, mimetype, geopoint, created_at, updated_at, tenant_id FROM uploads`

	countUploadsQuery = `SELECT COUNT(*) FROM uploads`

	insertUploadQuery = `INSERT INTO uploads (hash, slug, path, name, size, type, source, mimetype, geopoint, created_at, updated_at, tenant_id)
                         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
                         RETURNING id`

	updatedUploadQuery = `UPDATE uploads
                          SET hash = $1,
														slug = $2,
                              path = $3,
                              name = $4,
                              size = $5,
                              type = $6,
                              source = $7,
                              mimetype = $8,
                              geopoint = $9,
                              updated_at = $10
                          WHERE id = $11 AND tenant_id = $12`

	deleteUploadQuery = `DELETE FROM uploads WHERE id = $1 AND tenant_id = $2`

	existsUploadQuery = `SELECT EXISTS(SELECT 1 FROM uploads WHERE id = $1 AND tenant_id = $2)`
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
			&dbUpload.Slug,
			&dbUpload.Path,
			&dbUpload.Name,
			&dbUpload.Size,
			&dbUpload.Type,
			&dbUpload.Source,
			&dbUpload.Mimetype,
			&dbUpload.GeoPoint,
			&dbUpload.CreatedAt,
			&dbUpload.UpdatedAt,
			&dbUpload.TenantID,
		); err != nil {
			return nil, err
		}
		domainUpload, err := ToDomainUpload(&dbUpload)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, domainUpload)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return uploads, nil
}

func (g *GormUploadRepository) GetPaginated(
	ctx context.Context, params *upload.FindParams,
) ([]upload.Upload, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	where, args := []string{"tenant_id = $1"}, []interface{}{tenantID.String()}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, params.ID)
	}
	if params.Hash != "" {
		where, args = append(where, fmt.Sprintf("hash = $%d", len(args)+1)), append(args, params.Hash)
	}
	if params.Slug != "" {
		where, args = append(where, fmt.Sprintf("slug = $%d", len(args)+1)), append(args, params.Slug)
	}
	if params.Type != "" {
		where, args = append(where, fmt.Sprintf("type = $%d", len(args)+1)), append(args, params.Type.String())
	}
	if params.Source != "" {
		where, args = append(where, fmt.Sprintf("source = $%d", len(args)+1)), append(args, params.Source)
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := pool.QueryRow(ctx, countUploadsQuery+" WHERE tenant_id = $1", tenantID.String()).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUploadRepository) GetAll(ctx context.Context) ([]upload.Upload, error) {
	return g.queryUploads(ctx, selectUploadQuery)
}

func (g *GormUploadRepository) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	uploads, err := g.queryUploads(ctx, selectUploadQuery+" WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, ErrUploadNotFound
	}
	return uploads[0], nil
}

func (g *GormUploadRepository) GetByIDs(ctx context.Context, ids []uint) ([]upload.Upload, error) {
	if len(ids) == 0 {
		return []upload.Upload{}, nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	// Deduplicate IDs for efficiency
	idMap := make(map[uint]struct{})
	uniqueIDs := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, exists := idMap[id]; !exists {
			idMap[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	uploads, err := g.queryUploads(ctx, repo.Join(selectUploadQuery, "WHERE id = ANY($1) AND tenant_id = $2"), uniqueIDs, tenantID)
	if err != nil {
		return nil, err
	}
	return uploads, nil
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

func (g *GormUploadRepository) GetBySlug(ctx context.Context, slug string) (upload.Upload, error) {
	uploads, err := g.GetPaginated(ctx, &upload.FindParams{
		Slug: slug,
	})
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, ErrUploadNotFound
	}
	return uploads[0], nil
}

func (g *GormUploadRepository) Exists(ctx context.Context, id uint) (bool, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return false, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, err
	}

	var exists bool
	if err := pool.QueryRow(ctx, existsUploadQuery, id, tenantID.String()).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (g *GormUploadRepository) Create(ctx context.Context, data upload.Upload) (upload.Upload, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	dbUpload := ToDBUpload(data)
	dbUpload.TenantID = tenantID.String()

	if err := tx.QueryRow(
		ctx,
		insertUploadQuery,
		dbUpload.Hash,
		dbUpload.Slug,
		dbUpload.Path,
		dbUpload.Name,
		dbUpload.Size,
		dbUpload.Type,
		dbUpload.Source,
		dbUpload.Mimetype,
		dbUpload.GeoPoint,
		dbUpload.CreatedAt,
		dbUpload.UpdatedAt,
		dbUpload.TenantID,
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	dbUpload := ToDBUpload(data)
	dbUpload.TenantID = tenantID.String()

	if _, err := tx.Exec(
		ctx,
		updatedUploadQuery,
		dbUpload.Hash,
		dbUpload.Slug,
		dbUpload.Path,
		dbUpload.Name,
		dbUpload.Size,
		dbUpload.Type,
		dbUpload.Source,
		dbUpload.Mimetype,
		dbUpload.GeoPoint,
		dbUpload.UpdatedAt,
		dbUpload.ID,
		dbUpload.TenantID,
	); err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) UpdateSource(ctx context.Context, id uint, source string) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, "UPDATE uploads SET source = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3", source, id, tenantID.String()); err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, deleteUploadQuery, id, tenantID.String()); err != nil {
		return err
	}
	return nil
}
