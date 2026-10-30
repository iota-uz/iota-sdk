package persistence

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RoutedStorage serves uploads from two backends at once: paths of the form
// "s3://<bucket>/<key>" go to the object store, every other path to the local
// filesystem. New uploads are written to the primary backend.
type RoutedStorage struct {
	fs         upload.Storage
	s3         upload.Storage
	s3Primary  bool
	presignTTL time.Duration
}

var (
	_ upload.Storage           = (*RoutedStorage)(nil)
	_ upload.StreamingStorage  = (*RoutedStorage)(nil)
	_ upload.DeletingStorage   = (*RoutedStorage)(nil)
	_ upload.PresigningStorage = (*RoutedStorage)(nil)
	_ upload.LocatingStorage   = (*RoutedStorage)(nil)
)

const defaultPresignTTL = 5 * time.Minute

// NewUploadStorage builds the routed upload storage from configuration. S3
// reads are enabled whenever S3 is configured, independent of the primary backend.
func NewUploadStorage(uploadsCfg *uploadsconfig.Config, storageCfg *uploadsconfig.StorageConfig) (*RoutedStorage, error) {
	const op serrors.Op = "persistence.NewUploadStorage"
	fsStorage, err := NewFSStorage(uploadsCfg)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if storageCfg == nil {
		storageCfg = &uploadsconfig.StorageConfig{}
	}
	if err := storageCfg.Validate(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	var s3Storage upload.Storage
	if storageCfg.S3Configured() {
		if s3Storage, err = NewS3Storage(storageCfg.S3); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	routed, err := NewRoutedStorage(fsStorage, s3Storage, storageCfg.Backend() == uploadsconfig.BackendS3)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	routed.presignTTL = storageCfg.S3.PresignTTL()
	return routed, nil
}

// NewRoutedStorage combines a filesystem storage with an optional S3 storage.
// s3 may be nil, in which case s3:// paths fail with upload.ErrS3NotConfigured.
func NewRoutedStorage(fs, s3 upload.Storage, s3Primary bool) (*RoutedStorage, error) {
	const op serrors.Op = "persistence.NewRoutedStorage"
	if fs == nil {
		return nil, serrors.Wrap(op, serrors.NewInvalid("filesystem upload storage is required"))
	}
	if s3Primary && s3 == nil {
		return nil, serrors.Wrap(op, upload.ErrS3NotConfigured)
	}
	if s3Primary {
		if _, ok := s3.(upload.LocatingStorage); !ok {
			return nil, serrors.Wrap(op, serrors.NewInvalid("primary s3 upload storage must report its storage location"))
		}
	}
	return &RoutedStorage{fs: fs, s3: s3, s3Primary: s3Primary, presignTTL: defaultPresignTTL}, nil
}

func (s *RoutedStorage) backend(fileName string) (upload.Storage, error) {
	if !upload.IsS3Path(fileName) {
		return s.fs, nil
	}
	if s.s3 == nil {
		return nil, upload.ErrS3NotConfigured
	}
	return s.s3, nil
}

// StorageLocation implements upload.LocatingStorage.
func (s *RoutedStorage) StorageLocation() string {
	if !s.s3Primary {
		return ""
	}
	return s.s3.(upload.LocatingStorage).StorageLocation()
}

func (s *RoutedStorage) Open(ctx context.Context, fileName string) ([]byte, error) {
	const op serrors.Op = "persistence.RoutedStorage.Open"
	backend, err := s.backend(fileName)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return backend.Open(ctx, fileName)
}

func (s *RoutedStorage) OpenReader(ctx context.Context, fileName string) (io.ReadCloser, upload.ObjectInfo, error) {
	const op serrors.Op = "persistence.RoutedStorage.OpenReader"
	backend, err := s.backend(fileName)
	if err != nil {
		return nil, upload.ObjectInfo{}, serrors.Wrap(op, err)
	}
	return OpenUploadReader(ctx, backend, fileName)
}

func (s *RoutedStorage) Save(ctx context.Context, fileName string, data []byte) error {
	const op serrors.Op = "persistence.RoutedStorage.Save"
	backend, err := s.backend(fileName)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return backend.Save(ctx, fileName, data)
}

func (s *RoutedStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	const op serrors.Op = "persistence.RoutedStorage.Rename"
	if upload.IsS3Path(oldPath) != upload.IsS3Path(newPath) {
		return serrors.Wrap(op, upload.ErrStorageBackendMismatch)
	}
	backend, err := s.backend(oldPath)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return backend.Rename(ctx, oldPath, newPath)
}

func (s *RoutedStorage) Delete(ctx context.Context, fileName string) error {
	const op serrors.Op = "persistence.RoutedStorage.Delete"
	backend, err := s.backend(fileName)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	deleter, ok := backend.(upload.DeletingStorage)
	if !ok {
		return serrors.Wrap(op, serrors.NewUnimplemented("upload storage cannot delete"))
	}
	return deleter.Delete(ctx, fileName)
}

// PresignGetURL uses UPLOAD_S3_PRESIGN_TTL_SECONDS when ttl is not positive.
func (s *RoutedStorage) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	const op serrors.Op = "persistence.RoutedStorage.PresignGetURL"
	if ttl <= 0 {
		ttl = s.presignTTL
	}
	backend, err := s.backend(fileName)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	presigner, ok := backend.(upload.PresigningStorage)
	if !ok {
		return "", serrors.Wrap(op, upload.ErrPresignNotSupported)
	}
	return presigner.PresignGetURL(ctx, fileName, ttl)
}

// OpenUploadReader streams from storage when it supports it and otherwise
// falls back to the buffered Open.
func OpenUploadReader(ctx context.Context, storage upload.Storage, fileName string) (io.ReadCloser, upload.ObjectInfo, error) {
	if streaming, ok := storage.(upload.StreamingStorage); ok {
		return streaming.OpenReader(ctx, fileName)
	}
	data, err := storage.Open(ctx, fileName)
	if err != nil {
		return nil, upload.ObjectInfo{}, err
	}
	return io.NopCloser(bytes.NewReader(data)), upload.ObjectInfo{Size: int64(len(data))}, nil
}
