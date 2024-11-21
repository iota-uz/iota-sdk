package persistence

import (
	"context"
	"os"
	"path/filepath"
)

type FSStorage struct {
	basePath string
}

func NewFSStorage(basePath string) (*FSStorage, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(workDir, basePath)
	return &FSStorage{
		basePath: fullPath,
	}, nil
}

func (s *FSStorage) Save(ctx context.Context, fileName string, bytes []byte) error {
	if err := os.WriteFile(filepath.Join(s.basePath, fileName), bytes, os.ModeAppend); err != nil {
		return err
	}
	return nil
}
