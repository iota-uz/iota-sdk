// Package persistence provides this package.
package persistence

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
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
		return nil, serrors.Wrap(op, err)
	}
	fullPath := uploadsPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(workDir, fullPath)
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if err := os.Chmod(fullPath, 0755); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	privatePath := filepath.Join(fullPath, uploadsconfig.PrivateDirectory)
	if err := os.MkdirAll(privatePath, 0700); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if err := os.Chmod(privatePath, 0700); err != nil {
		return nil, serrors.Wrap(op, err)
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

func (s *FSStorage) OpenReader(ctx context.Context, fileName string) (io.ReadCloser, upload.ObjectInfo, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, upload.ObjectInfo{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, upload.ObjectInfo{}, err
	}
	if stat.IsDir() {
		_ = file.Close()
		return nil, upload.ObjectInfo{}, &fs.PathError{Op: "open", Path: fileName, Err: fs.ErrNotExist}
	}
	return file, upload.ObjectInfo{
		Size:        stat.Size(),
		ContentType: mime.TypeByExtension(filepath.Ext(fileName)),
		ModTime:     stat.ModTime(),
	}, nil
}

func (s *FSStorage) Delete(ctx context.Context, fileName string) error {
	if err := os.Remove(fileName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
