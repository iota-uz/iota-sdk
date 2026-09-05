package services_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUploadServiceCreatePrivateDoesNotReusePublicHash(t *testing.T) {
	// This test would be falsely green if CreatePrivate queried the global hash
	// but the mock happened to return not found.
	ctx := context.Background()
	repo := new(MockUploadRepository)
	storage := new(MockUploadStorage)
	service := services.NewUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()))
	content := []byte("%PDF-1.7\nprivate")
	privatePath := filepath.Join(t.TempDir(), ".private")
	slug := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	created := upload.NewWithID(7, uuid.New(), "hash", filepath.Join(privatePath, slug+".pdf"), "policy.pdf", slug, len(content), mimetype.Detect(content), upload.UploadTypeDocument, time.Now(), time.Now())

	repo.On("GetBySlug", ctx, slug).Return(nil, persistence.ErrUploadNotFound).Once()
	storage.On("Save", ctx, filepath.Join(privatePath, slug+".pdf"), content).Return(nil).Once()
	repo.On("Create", ctx, mock.MatchedBy(func(entity upload.Upload) bool {
		return entity.Path() == filepath.Join(privatePath, slug+".pdf")
	})).Return(created, nil).Once()

	result, err := service.CreatePrivate(ctx, &upload.CreateDTO{
		File: bytes.NewReader(content), Name: "policy.pdf", Size: len(content), Slug: slug,
	}, privatePath)
	require.NoError(t, err)
	require.Equal(t, created, result)
	repo.AssertNotCalled(t, "GetByHash", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestUploadServiceCreatePrivateRejectsSlugReplacement(t *testing.T) {
	// This test would be falsely green if the replacement path failed only
	// because storage or repository mutation mocks were incomplete.
	ctx := context.Background()
	repo := new(MockUploadRepository)
	storage := new(MockUploadStorage)
	service := services.NewUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()))
	privatePath := filepath.Join(t.TempDir(), ".private")
	slug := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	existing := upload.New("different-hash", filepath.Join(privatePath, slug+".pdf"), "old.pdf", slug, 3, mimetype.Detect([]byte("old")))
	repo.On("GetBySlug", ctx, slug).Return(existing, nil).Once()

	_, err := service.CreatePrivate(ctx, &upload.CreateDTO{
		File: bytes.NewReader([]byte("%PDF-new")), Name: "new.pdf", Size: 8, Slug: slug,
	}, privatePath)
	require.ErrorIs(t, err, services.ErrUploadSlugConflict)
	require.NotErrorIs(t, err, persistence.ErrUploadNotFound)
	repo.AssertNotCalled(t, "GetByHash", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	storage.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)
}

func TestUploadServiceCreatePrivateRejectsPublicSlugMatch(t *testing.T) {
	// This test would be falsely green if matching content were accepted without
	// checking that the existing upload belongs to the private namespace.
	ctx := context.Background()
	repo := new(MockUploadRepository)
	storage := new(MockUploadStorage)
	service := services.NewUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()))
	content := []byte("%PDF-same")
	privatePath := filepath.Join(t.TempDir(), ".private")
	slug := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	public := upload.New("635b3969c08c7cac341c01f017ab0e0a", filepath.Join(t.TempDir(), slug+".pdf"), "public.pdf", slug, len(content), mimetype.Detect(content))
	repo.On("GetBySlug", ctx, slug).Return(public, nil).Once()

	_, err := service.CreatePrivate(ctx, &upload.CreateDTO{
		File: bytes.NewReader(content), Name: "private.pdf", Size: len(content), Slug: slug,
	}, privatePath)
	require.ErrorIs(t, err, services.ErrUploadSlugConflict)
	storage.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)
}

func TestUploadServiceCreatePrivateRequiresNamespace(t *testing.T) {
	// This test would be falsely green if an empty path reached ToEntity and
	// another dependency happened to reject the upload first.
	service := services.NewUploadService(new(MockUploadRepository), new(MockUploadStorage), eventbus.NewEventPublisher(logrus.New()))
	_, err := service.CreatePrivate(context.Background(), &upload.CreateDTO{
		File: bytes.NewReader([]byte("%PDF")), Name: "private.pdf", Size: 4,
	}, "")
	require.ErrorIs(t, err, services.ErrPrivateUploadPathRequired)
}
