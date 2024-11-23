package persistence

import (
	"context"
	"os"
	"path/filepath"

	"github.com/iota-agency/iota-sdk/pkg/configuration"
)

type FSStorage struct{}

func NewFSStorage() (*FSStorage, error) {
	conf := configuration.Use()
	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(workDir, conf.UploadsPath)
	if err := os.MkdirAll(fullPath, 0777); err != nil {
		return nil, err
	}
	return &FSStorage{}, nil
}

func (s *FSStorage) Save(ctx context.Context, fileName string, bytes []byte) error {
	conf := configuration.Use()
	if err := os.WriteFile(filepath.Join(conf.UploadsPath, fileName), bytes, 0644); err != nil {
		return err
	}
	return nil
}
