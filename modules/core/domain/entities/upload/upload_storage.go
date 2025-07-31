package upload

import (
	"context"
)

type Storage interface {
	Open(ctx context.Context, fileName string) ([]byte, error)
	Save(ctx context.Context, fileName string, bytes []byte) error
	Rename(ctx context.Context, oldPath, newPath string) error
}
