// Package services provides this package.
package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var ErrUploadSlugConflict = errors.New("upload slug already belongs to different content")
var ErrPrivateUploadPathRequired = errors.New("private upload path is required")
var ErrPrivateUploadPathInvalid = errors.New("private upload path escapes namespace")

type UploadService struct {
	repo        upload.Repository
	storage     upload.Storage
	publisher   eventbus.EventBus
	uploadsPath string
}

// NewConfiguredUploadService uses the same upload root as the mounted file controller.
func NewConfiguredUploadService(repo upload.Repository, storage upload.Storage, publisher eventbus.EventBus, cfg *uploadsconfig.Config) *UploadService {
	service := NewUploadService(repo, storage, publisher)
	if cfg != nil {
		service.uploadsPath = cfg.Path
	}
	return service
}

func NewUploadService(
	repo upload.Repository,
	storage upload.Storage,
	publisher eventbus.EventBus,
) *UploadService {
	return &UploadService{
		repo:      repo,
		publisher: publisher,
		storage:   storage,
	}
}

func (s *UploadService) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) Exists(ctx context.Context, id uint) (bool, error) {
	return s.repo.Exists(ctx, id)
}

// IsAccessible checks if upload metadata exists in DB and backing storage content can be opened.
func (s *UploadService) IsAccessible(ctx context.Context, entity upload.Upload) bool {
	if entity == nil {
		return false
	}
	body, _, err := s.OpenReader(ctx, entity.Path())
	if err != nil {
		return false
	}
	_ = body.Close()
	return true
}

// Open reads a whole stored upload by its stored path (Upload.Path()), on
// whichever backend holds it. Prefer OpenReader for large files.
func (s *UploadService) Open(ctx context.Context, storedPath string) ([]byte, error) {
	const op serrors.Op = "UploadService.Open"
	data, err := s.storage.Open(ctx, storedPath)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return data, nil
}

// OpenReader streams a stored upload by its stored path (Upload.Path()):
// "s3://<bucket>/<key>" paths are read from S3, any other path from the local
// filesystem. It performs no authorization; callers check access first and
// must close the reader.
func (s *UploadService) OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error) {
	const op serrors.Op = "UploadService.OpenReader"
	body, info, err := persistence.OpenUploadReader(ctx, s.storage, storedPath)
	if err != nil {
		return nil, upload.ObjectInfo{}, serrors.Wrap(op, err)
	}
	return body, info, nil
}

// PresignGetURL returns a short-lived direct download URL for an S3-backed
// upload. A non-positive ttl uses UPLOAD_S3_PRESIGN_TTL_SECONDS. Filesystem
// paths return upload.ErrPresignNotSupported. It performs no authorization.
func (s *UploadService) PresignGetURL(ctx context.Context, storedPath string, ttl time.Duration) (string, error) {
	const op serrors.Op = "UploadService.PresignGetURL"
	presigner, ok := s.storage.(upload.PresigningStorage)
	if !ok {
		return "", serrors.Wrap(op, upload.ErrPresignNotSupported)
	}
	presigned, err := presigner.PresignGetURL(ctx, storedPath, ttl)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	return presigned, nil
}

func (s *UploadService) storageLocation() string {
	if locating, ok := s.storage.(upload.LocatingStorage); ok {
		return locating.StorageLocation()
	}
	return ""
}

func (s *UploadService) GetByHash(ctx context.Context, hash string) (upload.Upload, error) {
	return s.repo.GetByHash(ctx, hash)
}

func (s *UploadService) GetBySlug(ctx context.Context, slug string) (upload.Upload, error) {
	return s.repo.GetBySlug(ctx, slug)
}

func (s *UploadService) GetAll(ctx context.Context) ([]upload.Upload, error) {
	return s.repo.GetAll(ctx)
}

func (s *UploadService) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *UploadService) Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error) {
	if data.UploadsPath != "" || s.uploadsPath == "" {
		return s.create(ctx, data, true, true)
	}
	configured := *data
	configured.UploadsPath = s.uploadsPath
	return s.create(ctx, &configured, true, true)
}

// CreatePrivate stores an upload in a caller-provided non-public namespace.
// Hash deduplication is intentionally limited to the deterministic slug so a
// private upload can never resolve to content from the public namespace.
func (s *UploadService) CreatePrivate(ctx context.Context, data *upload.CreateDTO, privatePath string) (upload.Upload, error) {
	const op serrors.Op = "UploadService.CreatePrivate"
	if privatePath == "" {
		return nil, serrors.Wrap(op, ErrPrivateUploadPathRequired)
	}
	privateData := *data
	privateData.UploadsPath = privatePath
	return s.create(ctx, &privateData, false, false, privatePath)
}

// pathWithin compares filesystem layouts, so S3 keys match the same roots as local paths.
func pathWithin(root, candidate string) bool {
	if upload.IsS3Path(candidate) {
		root = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(root)), "/")
		candidate = upload.LayoutPath(candidate)
	}
	rel, err := filepath.Rel(filepath.Clean(filepath.FromSlash(root)), filepath.Clean(filepath.FromSlash(candidate)))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *UploadService) create(ctx context.Context, data *upload.CreateDTO, deduplicateByHash, replaceSlug bool, requiredPath ...string) (upload.Upload, error) {
	const op serrors.Op = "UploadService.create"
	if location := s.storageLocation(); location != "" && data.StorageLocation == "" {
		located := *data
		located.StorageLocation = location
		data = &located
	}
	entity, bytes, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	if len(requiredPath) > 0 && !pathWithin(requiredPath[0], entity.Path()) {
		return nil, serrors.Wrap(op, ErrPrivateUploadPathInvalid)
	}

	up, err := s.repo.GetBySlug(ctx, entity.Slug())
	if err != nil && !errors.Is(err, persistence.ErrUploadNotFound) {
		return nil, err
	}

	if up == nil && deduplicateByHash {
		up, err = s.repo.GetByHash(ctx, entity.Hash())
		if err != nil && !errors.Is(err, persistence.ErrUploadNotFound) {
			return nil, err
		}
	}
	if up != nil {
		if len(requiredPath) > 0 && !pathWithin(requiredPath[0], up.Path()) {
			return nil, serrors.Wrap(op, fmt.Errorf("%w: %s", ErrUploadSlugConflict, entity.Slug()))
		}
		if up.Hash() != entity.Hash() {
			if !replaceSlug {
				return nil, serrors.Wrap(op, fmt.Errorf("%w: %s", ErrUploadSlugConflict, entity.Slug()))
			}
			existing, err := s.repo.GetByHash(ctx, entity.Hash())
			if err != nil && !errors.Is(err, persistence.ErrUploadNotFound) {
				return nil, err
			}
			if existing != nil {
				if err := s.moveSlugToExisting(ctx, up, existing, entity.Slug()); err != nil {
					return nil, err
				}
				up = existing
			} else {
				up.SetName(entity.Name())
				entity.SetID(up.ID())
				if err := s.storage.Save(ctx, entity.Path(), bytes); err != nil {
					return nil, err
				}
				if err := s.repo.Update(ctx, entity); err != nil {
					return nil, err
				}
				up = entity
			}
		}
		if data.UploadsPath != "" && !pathWithin(data.UploadsPath, up.Path()) {
			rooted := upload.NewWithID(up.ID(), up.TenantID(), up.Hash(), upload.LocatePath(data.StorageLocation, filepath.Join(data.UploadsPath, filepath.Base(upload.LayoutPath(up.Path())))), up.Name(), up.Slug(), up.Size().Bytes(), up.Mimetype(), up.Type(), up.CreatedAt(), up.UpdatedAt(), upload.WithUploadsPath(data.UploadsPath))
			rooted.SetGeoPoint(up.GeoPoint())
			if err := s.storage.Save(ctx, rooted.Path(), bytes); err != nil {
				return nil, err
			}
			if err := s.repo.Update(ctx, rooted); err != nil {
				return nil, err
			}
			up = rooted
		}
		return up, nil
	}
	if err := s.storage.Save(ctx, entity.Path(), bytes); err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	createdEvent, err := upload.NewCreatedEvent(ctx, *data, createdEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

// moveSlugToExisting gives slug to existing and parks holder under its own hash.
// Objects move before the rows and every step is undone on failure, so a failed
// step never leaves a row pointing at an object that has moved.
func (s *UploadService) moveSlugToExisting(ctx context.Context, holder, existing upload.Upload, slug string) error {
	const op serrors.Op = "UploadService.moveSlugToExisting"
	holderPath, existingPath := holder.Path(), existing.Path()
	holderSlug, existingSlug := holder.Slug(), existing.Slug()
	holder.SetSlug(holder.Hash())
	existing.SetSlug(slug)
	holderTarget, existingTarget := holder.Path(), existing.Path()
	undoCtx := context.WithoutCancel(ctx)
	restoreEntities := func() {
		holder.SetSlug(holderSlug)
		existing.SetSlug(existingSlug)
	}

	if err := s.storage.Rename(ctx, holderPath, holderTarget); err != nil {
		restoreEntities()
		return serrors.Wrap(op, err)
	}
	if err := s.storage.Rename(ctx, existingPath, existingTarget); err != nil {
		restoreEntities()
		return serrors.Wrap(op, errors.Join(err, s.storage.Rename(undoCtx, holderTarget, holderPath)))
	}
	undoRenames := func() error {
		return errors.Join(
			s.storage.Rename(undoCtx, existingTarget, existingPath),
			s.storage.Rename(undoCtx, holderTarget, holderPath),
		)
	}
	if err := s.repo.Update(ctx, holder); err != nil {
		restoreEntities()
		return serrors.Wrap(op, errors.Join(err, undoRenames()))
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		restoreEntities()
		return serrors.Wrap(op, errors.Join(err, s.repo.Update(undoCtx, holder), undoRenames()))
	}
	return nil
}

func (s *UploadService) CreateMany(ctx context.Context, data []*upload.CreateDTO) ([]upload.Upload, error) {
	entities := make([]upload.Upload, 0, len(data))
	for _, d := range data {
		entity, err := s.Create(ctx, d)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (s *UploadService) Delete(ctx context.Context, id uint) (upload.Upload, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := upload.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
