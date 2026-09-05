// Package persistence provides this package.
package persistence

import (
	"context"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type FSStorage struct{}

func NewFSStorage(cfg *uploadsconfig.Config) (*FSStorage, error) {
	const op serrors.Op = "persistence.NewFSStorage"
	uploadsPath := "static"
	if cfg != nil && cfg.Path != "" {
		uploadsPath = cfg.Path
	}
	workDir, err := os.Getwd()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	fullPath := uploadsPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(workDir, fullPath)
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, serrors.E(op, err)
	}
	if err := os.Chmod(fullPath, 0755); err != nil {
		return nil, serrors.E(op, err)
	}
	privatePath := filepath.Join(fullPath, uploadsconfig.PrivateDirectory)
	if err := os.MkdirAll(privatePath, 0700); err != nil {
		return nil, serrors.E(op, err)
	}
	if err := os.Chmod(privatePath, 0700); err != nil {
		return nil, serrors.E(op, err)
	}
	return &FSStorage{}, nil
}

func (s *FSStorage) Open(ctx context.Context, fileName string) ([]byte, error) {
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (s *FSStorage) Save(ctx context.Context, fileName string, bytes []byte) error {
	return os.WriteFile(fileName, bytes, 0644)
}

func (s *FSStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
