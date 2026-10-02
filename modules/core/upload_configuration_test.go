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

type uploadMetadata struct{ upload.Repository }

func (*uploadMetadata) GetByHash(context.Context, string) (upload.Upload, error) {
	return nil, persistence.ErrUploadNotFound
}
func (*uploadMetadata) GetBySlug(context.Context, string) (upload.Upload, error) {
	return nil, persistence.ErrUploadNotFound
}
func (*uploadMetadata) Create(_ context.Context, entity upload.Upload) (upload.Upload, error) {
	return entity, nil
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
	response, err := http.Get(server.URL + created.PreviewURL())
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, content, body)
}
