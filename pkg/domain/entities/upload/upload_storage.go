package upload

import (
	"context"
)

type Storage interface {
	Save(ctx context.Context, fileName string, bytes []byte) error
}
