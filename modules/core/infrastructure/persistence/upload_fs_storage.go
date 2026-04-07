// Package persistence provides this package.
package persistence

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type FSStorage struct{}

func NewFSStorage() (*FSStorage, error) {
	const op serrors.Op = "persistence.NewFSStorage"

	conf := configuration.Use()
	workDir, err := os.Getwd()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	fullPath := filepath.Join(workDir, conf.UploadsPath)
	if err := os.MkdirAll(fullPath, 0o755); err != nil {
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
	const op serrors.Op = "persistence.FSStorage.Save"
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(fileName), 0o755); err != nil {
		return serrors.E(op, err)
	}
	if err := os.WriteFile(fileName, bytes, 0o644); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *FSStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	const op serrors.Op = "persistence.FSStorage.Rename"
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return serrors.E(op, err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *FSStorage) Delete(ctx context.Context, fileName string) error {
	const op serrors.Op = "persistence.FSStorage.Delete"
	_ = ctx
	if err := os.Remove(fileName); err != nil && !os.IsNotExist(err) {
		return serrors.E(op, err)
	}
	return nil
}

func (s *FSStorage) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	const op serrors.Op = "persistence.FSStorage.PresignGetURL"

	_ = ctx
	_ = ttl

	conf := configuration.Use()
	cleaned := strings.TrimSpace(fileName)
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = path.Clean("/" + cleaned)
	if cleaned == "/" {
		return "", serrors.E(op, serrors.Invalid, fmt.Errorf("invalid file path"))
	}

	return (&url.URL{
		Scheme: conf.Scheme(),
		Host:   conf.Domain,
		Path:   cleaned,
	}).String(), nil
}
