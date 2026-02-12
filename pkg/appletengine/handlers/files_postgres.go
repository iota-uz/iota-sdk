package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresFilesStore struct {
	pool    *pgxpool.Pool
	baseDir string
}

func NewPostgresFilesStore(pool *pgxpool.Pool, baseDir string) (*PostgresFilesStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = filepath.Join(os.TempDir(), "iota-applet-engine-files")
	}
	return &PostgresFilesStore{
		pool:    pool,
		baseDir: baseDir,
	}, nil
}

func (s *PostgresFilesStore) Store(ctx context.Context, name, contentType string, data []byte) (map[string]any, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres files.store: %w", err)
	}
	id := uuid.NewString()
	safeName := sanitizeFileName(name)
	if safeName == "" {
		safeName = "file.bin"
	}
	fileDir := filepath.Join(s.baseDir, tenantID, appletID)
	if err := os.MkdirAll(fileDir, 0o755); err != nil {
		return nil, fmt.Errorf("create files directory: %w", err)
	}
	filePath := filepath.Join(fileDir, id+"-"+safeName)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO applet_engine_files(tenant_id, applet_id, file_id, file_name, content_type, size_bytes, storage_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`, tenantID, appletID, id, safeName, strings.TrimSpace(contentType), len(data), filePath)
	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		// Clean up orphaned file on DB failure
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("postgres files.store: %w", err)
	}

	return map[string]any{
		"id":          id,
		"name":        safeName,
		"contentType": strings.TrimSpace(contentType),
		"size":        len(data),
		"path":        filePath,
		"createdAt":   createdAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (s *PostgresFilesStore) Get(ctx context.Context, id string) (map[string]any, bool, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("postgres files.get: %w", err)
	}
	row := s.pool.QueryRow(ctx, `
		SELECT file_name, content_type, size_bytes, storage_path, created_at
		FROM applet_engine_files
		WHERE tenant_id = $1 AND applet_id = $2 AND file_id = $3
	`, tenantID, appletID, id)
	var (
		fileName    string
		contentType string
		sizeBytes   int
		storagePath string
		createdAt   time.Time
	)
	if err := row.Scan(&fileName, &contentType, &sizeBytes, &storagePath, &createdAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("postgres files.get: %w", err)
	}
	return map[string]any{
		"id":          id,
		"name":        fileName,
		"contentType": contentType,
		"size":        sizeBytes,
		"path":        storagePath,
		"createdAt":   createdAt.UTC().Format(time.RFC3339Nano),
	}, true, nil
}

func (s *PostgresFilesStore) Delete(ctx context.Context, id string) (bool, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return false, fmt.Errorf("postgres files.delete: %w", err)
	}
	row := s.pool.QueryRow(ctx, `
		DELETE FROM applet_engine_files
		WHERE tenant_id = $1 AND applet_id = $2 AND file_id = $3
		RETURNING storage_path
	`, tenantID, appletID, id)
	var storagePath string
	if err := row.Scan(&storagePath); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("postgres files.delete: %w", err)
	}
	if err := os.Remove(storagePath); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("delete file from storage: %w", err)
	}
	return true, nil
}
