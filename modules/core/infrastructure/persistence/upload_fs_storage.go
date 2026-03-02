// Package persistence provides this package.
package persistence

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

func (s *FSStorage) Open(ctx context.Context, fileName string) ([]byte, error) {
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (s *FSStorage) Save(ctx context.Context, fileName string, bytes []byte) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(fileName), 0777); err != nil {
		return err
	}
	return os.WriteFile(fileName, bytes, 0644)
}

func (s *FSStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(newPath), 0777); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

func (s *FSStorage) Delete(ctx context.Context, fileName string) error {
	_ = ctx
	if err := os.Remove(fileName); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *FSStorage) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	_ = ctx
	_ = ttl

	cleaned := strings.TrimSpace(fileName)
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = path.Clean("/" + cleaned)
	if cleaned == "/" {
		return "", fmt.Errorf("invalid file path")
	}
	return cleaned, nil
}
