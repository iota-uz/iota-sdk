package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type FilesStore interface {
	Store(ctx context.Context, name, contentType string, data []byte) (map[string]any, error)
	Get(ctx context.Context, id string) (map[string]any, bool, error)
	Delete(ctx context.Context, id string) (bool, error)
}

type FilesStub struct {
	store FilesStore
}

type localFilesStore struct {
	baseDir string
	mu      sync.RWMutex
	files   map[string]map[string]fileRecord
}

type fileRecord struct {
	ID          string
	Name        string
	ContentType string
	Size        int
	Path        string
	CreatedAt   time.Time
}

func NewFilesStub() *FilesStub {
	return &FilesStub{store: newLocalFilesStore("")}
}

func NewFilesStubWithStore(store FilesStore) *FilesStub {
	if store == nil {
		store = newLocalFilesStore("")
	}
	return &FilesStub{store: store}
}

func newLocalFilesStore(baseDir string) *localFilesStore {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = filepath.Join(os.TempDir(), "iota-applet-engine-files")
	}
	return &localFilesStore{
		baseDir: baseDir,
		files:   make(map[string]map[string]fileRecord),
	}
}

func (s *FilesStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	methods := map[string]applets.Procedure[any, any]{
		"store": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				name, _ := payload["name"].(string)
				name = strings.TrimSpace(name)
				if name == "" {
					return nil, fmt.Errorf("name is required: %w", applets.ErrInvalid)
				}
				contentType, _ := payload["contentType"].(string)
				dataBase64, _ := payload["dataBase64"].(string)
				if strings.TrimSpace(dataBase64) == "" {
					return nil, fmt.Errorf("dataBase64 is required: %w", applets.ErrInvalid)
				}
				data, err := base64.StdEncoding.DecodeString(dataBase64)
				if err != nil {
					return nil, fmt.Errorf("invalid dataBase64: %w", applets.ErrInvalid)
				}
				return s.store.Store(ctx, name, contentType, data)
			},
		},
		"get": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := payload["id"].(string)
				id = strings.TrimSpace(id)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				value, found, err := s.store.Get(ctx, id)
				if err != nil {
					return nil, err
				}
				if !found {
					return nil, nil
				}
				return value, nil
			},
		},
		"delete": {
			Handler: func(ctx context.Context, params any) (any, error) {
				payload, ok := params.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
				}
				id, _ := payload["id"].(string)
				id = strings.TrimSpace(id)
				if id == "" {
					return nil, fmt.Errorf("id is required: %w", applets.ErrInvalid)
				}
				return s.store.Delete(ctx, id)
			},
		},
	}

	for op, procedure := range methods {
		router := applets.NewTypedRPCRouter()
		if err := applets.AddProcedure(router, fmt.Sprintf("%s.files.%s", appletName, op), procedure); err != nil {
			return err
		}
		for methodName, method := range router.Config().Methods {
			if err := registry.RegisterServerOnly(appletName, methodName, method, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *localFilesStore) Store(ctx context.Context, name, contentType string, data []byte) (map[string]any, error) {
	scope := scopeFromContext(ctx)
	tenantID, appletID := tenantAndAppletFromContext(ctx)
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

	record := fileRecord{
		ID:          id,
		Name:        safeName,
		ContentType: strings.TrimSpace(contentType),
		Size:        len(data),
		Path:        filePath,
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	scopeFiles := s.files[scope]
	if scopeFiles == nil {
		scopeFiles = make(map[string]fileRecord)
		s.files[scope] = scopeFiles
	}
	scopeFiles[id] = record
	return fileRecordResponse(record), nil
}

func (s *localFilesStore) Get(ctx context.Context, id string) (map[string]any, bool, error) {
	scope := scopeFromContext(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopeFiles := s.files[scope]
	if scopeFiles == nil {
		return nil, false, nil
	}
	record, ok := scopeFiles[id]
	if !ok {
		return nil, false, nil
	}
	return fileRecordResponse(record), true, nil
}

func (s *localFilesStore) Delete(ctx context.Context, id string) (bool, error) {
	scope := scopeFromContext(ctx)

	s.mu.Lock()
	scopeFiles := s.files[scope]
	if scopeFiles == nil {
		s.mu.Unlock()
		return false, nil
	}
	record, ok := scopeFiles[id]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	delete(scopeFiles, id)
	s.mu.Unlock()

	if err := os.Remove(record.Path); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("delete file: %w", err)
	}
	return true, nil
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, string(filepath.Separator), "_")
	base = strings.ReplaceAll(base, "..", "_")
	return strings.TrimSpace(base)
}

func fileRecordResponse(record fileRecord) map[string]any {
	return map[string]any{
		"id":          record.ID,
		"name":        record.Name,
		"contentType": record.ContentType,
		"size":        record.Size,
		"path":        record.Path,
		"createdAt":   record.CreatedAt.Format(time.RFC3339Nano),
	}
}
