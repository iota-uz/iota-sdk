package core_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type uploadMetadata struct {
	upload.Repository
	existing upload.Upload
	bySlug   bool
}

func (r *uploadMetadata) GetByHash(context.Context, string) (upload.Upload, error) {
	if r.existing != nil && !r.bySlug {
		return r.existing, nil
	}
	return nil, persistence.ErrUploadNotFound
}
func (r *uploadMetadata) GetBySlug(context.Context, string) (upload.Upload, error) {
	if r.existing != nil && r.bySlug {
		return r.existing, nil
	}
	return nil, persistence.ErrUploadNotFound
}
func (*uploadMetadata) Create(_ context.Context, entity upload.Upload) (upload.Upload, error) {
	return entity, nil
}
func (r *uploadMetadata) Update(_ context.Context, entity upload.Upload) error {
	r.existing = entity
	return nil
}

func TestConfiguredUploadServesDeduplicatedBytesFromOwnedDirectory(t *testing.T) {
	// Falsely green if deduplication is skipped or file serving is mocked.
	for _, bySlug := range []bool{false, true} {
		t.Run(map[bool]string{false: "hash", true: "slug"}[bySlug], func(t *testing.T) {
			directory, err := os.MkdirTemp(".", "upload-dedup-")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(directory)) })
			legacy := filepath.Join(directory, "baseline")
			owned := filepath.Join(directory, "owned")
			content := []byte("%PDF-1.7\nexisting baseline document")
			existing, _, err := (&upload.CreateDTO{File: bytes.NewReader(content), Name: "policy.pdf", Size: len(content), UploadsPath: legacy}).ToEntity()
			require.NoError(t, err)
			existing.SetID(42)
			require.NoError(t, os.MkdirAll(legacy, 0o755))
			require.NoError(t, os.WriteFile(existing.Path(), content, 0o600))
			metadata := &uploadMetadata{existing: existing, bySlug: bySlug}
			cfg := &uploadsconfig.Config{Path: owned}
			storage, err := persistence.NewFSStorage(cfg)
			require.NoError(t, err)
			service := services.NewConfiguredUploadService(metadata, storage, eventbus.NewEventPublisher(logrus.New()), cfg)
			router := mux.NewRouter()
			controllers.NewUploadController(service, cfg).Register(router)
			server := httptest.NewServer(router)
			defer server.Close()
			created, err := service.Create(t.Context(), &upload.CreateDTO{File: bytes.NewReader(content), Name: "policy.pdf", Size: len(content)})
			require.NoError(t, err)
			require.Equal(t, existing.ID(), created.ID())
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+created.PreviewURL(), nil)
			require.NoError(t, err)
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			require.Equal(t, http.StatusOK, response.StatusCode)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, content, body)
			baseline, err := os.ReadFile(existing.Path())
			require.NoError(t, err)
			require.Equal(t, content, baseline)
		})
	}
}

func TestConfiguredUploadServesCreatedBytesFromOwnedDirectory(t *testing.T) {
	// Falsely green if storage or the mounted file HTTP handler are replaced with a mock.
	directory, err := os.MkdirTemp(".", "upload-owned-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(directory)) })
	cfg := &uploadsconfig.Config{Path: filepath.Clean(directory)}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	service := services.NewConfiguredUploadService(&uploadMetadata{}, storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	router := mux.NewRouter()
	controllers.NewUploadController(service, cfg).Register(router)
	server := httptest.NewServer(router)
	defer server.Close()
	content := []byte("%PDF-1.7\nowned policy document")
	created, err := service.Create(context.Background(), &upload.CreateDTO{File: bytes.NewReader(content), Name: "policy.pdf", Size: len(content)})
	require.NoError(t, err)
	disk, err := os.ReadFile(filepath.Join(directory, filepath.Base(created.Path())))
	require.NoError(t, err)
	require.Equal(t, content, disk)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+created.PreviewURL(), nil)
	require.NoError(t, err)
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, content, body)
}
