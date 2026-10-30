package upload

import (
	"context"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// S3PathScheme prefixes stored upload paths that live in an S3-compatible bucket.
const S3PathScheme = "s3://"

var (
	ErrS3NotConfigured        = errors.New("upload is stored in S3 but S3 storage is not configured")
	ErrInvalidS3Path          = errors.New("invalid s3 upload path")
	ErrPresignNotSupported    = errors.New("storage cannot presign upload path")
	ErrStorageBackendMismatch = errors.New("upload paths belong to different storage backends")
)

// Storage is the byte-oriented upload storage contract.
type Storage interface {
	Open(ctx context.Context, fileName string) ([]byte, error)
	Save(ctx context.Context, fileName string, bytes []byte) error
	Rename(ctx context.Context, oldPath, newPath string) error
}

// ObjectInfo describes a stored object opened for streaming. Zero values mean unknown.
type ObjectInfo struct {
	Size        int64
	ContentType string
	ModTime     time.Time
}

// StreamingStorage opens a stored object without buffering it in memory.
// Callers must close the returned reader.
type StreamingStorage interface {
	OpenReader(ctx context.Context, fileName string) (io.ReadCloser, ObjectInfo, error)
}

// DeletingStorage removes a stored object. Deleting a missing object is not an error.
type DeletingStorage interface {
	Delete(ctx context.Context, fileName string) error
}

// PresigningStorage issues a short-lived URL that grants read access to one object.
type PresigningStorage interface {
	PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error)
}

// LocatingStorage reports where new uploads are written: "" for the local
// filesystem, or a location prefix such as "s3://<bucket>".
type LocatingStorage interface {
	StorageLocation() string
}

// LocatePath places a filesystem-layout path (e.g. "static/<slug>.pdf") under
// a storage location; an empty location keeps the path unchanged.
func LocatePath(location, localPath string) string {
	if location == "" {
		return localPath
	}
	return strings.TrimSuffix(location, "/") + "/" + strings.TrimPrefix(filepath.ToSlash(localPath), "/")
}

// S3Path builds the stored path "s3://<bucket>/<key>".
func S3Path(bucket, key string) string {
	return S3PathScheme + bucket + "/" + strings.TrimPrefix(key, "/")
}

// IsS3Path reports whether p uses the s3:// scheme.
func IsS3Path(p string) bool {
	return strings.HasPrefix(p, S3PathScheme)
}

// ParseS3Path splits "s3://<bucket>/<key>" into a bucket and a cleaned key.
func ParseS3Path(p string) (string, string, error) {
	if !IsS3Path(p) {
		return "", "", ErrInvalidS3Path
	}
	bucket, rawKey, ok := strings.Cut(strings.TrimPrefix(p, S3PathScheme), "/")
	if !ok || bucket == "" {
		return "", "", ErrInvalidS3Path
	}
	key := strings.TrimPrefix(path.Clean("/"+rawKey), "/")
	if key == "" || key != rawKey {
		return "", "", ErrInvalidS3Path
	}
	return bucket, key, nil
}

// LayoutPath returns the filesystem-layout part of a stored path: the object
// key for S3 paths and the path itself otherwise.
func LayoutPath(p string) string {
	if _, key, err := ParseS3Path(p); err == nil {
		return key
	}
	return p
}
