package upload

import (
	"context"
	"time"
)

type Storage interface {
	Open(ctx context.Context, fileName string) ([]byte, error)
	Save(ctx context.Context, fileName string, bytes []byte) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Delete(ctx context.Context, fileName string) error
	PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error)
}
