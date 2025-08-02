package cache

import (
	"context"
	"errors"
)

var (
	ErrKeyNotFound = errors.New("chat thread not found")
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

