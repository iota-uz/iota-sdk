package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type uploadRepoMock struct {
	mock.Mock
}

func (m *uploadRepoMock) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) GetByIDs(ctx context.Context, ids []uint) ([]upload.Upload, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) GetByHash(ctx context.Context, hash string) (upload.Upload, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) GetBySlug(ctx context.Context, slug string) (upload.Upload, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) GetAll(ctx context.Context) ([]upload.Upload, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) Create(ctx context.Context, entity upload.Upload) (upload.Upload, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *uploadRepoMock) Update(ctx context.Context, entity upload.Upload) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *uploadRepoMock) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *uploadRepoMock) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *uploadRepoMock) Exists(ctx context.Context, id uint) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

type uploadStorageMock struct {
	mock.Mock
}

func (m *uploadStorageMock) Open(ctx context.Context, fileName string) ([]byte, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *uploadStorageMock) Save(ctx context.Context, fileName string, bytes []byte) error {
	args := m.Called(ctx, fileName, bytes)
	return args.Error(0)
}

func (m *uploadStorageMock) Rename(ctx context.Context, oldPath, newPath string) error {
	args := m.Called(ctx, oldPath, newPath)
	return args.Error(0)
}

func (m *uploadStorageMock) Delete(ctx context.Context, fileName string) error {
	args := m.Called(ctx, fileName)
	return args.Error(0)
}

func (m *uploadStorageMock) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	args := m.Called(ctx, fileName, ttl)
	return args.String(0), args.Error(1)
}

func TestUploadService_GetDownloadURLByPath(t *testing.T) {
	t.Parallel()

	repo := &uploadRepoMock{}
	storage := &uploadStorageMock{}
	svc := services.NewUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()))

	storage.On("PresignGetURL", mock.Anything, "uploads/a.txt", mock.MatchedBy(func(ttl time.Duration) bool {
		return ttl > 0
	})).Return("https://example.test/presigned", nil).Once()

	got, err := svc.GetDownloadURLByPath(context.Background(), "uploads/a.txt")
	require.NoError(t, err)
	require.Equal(t, "https://example.test/presigned", got)
	storage.AssertExpectations(t)
}

func TestUploadService_Delete_RemovesStorageObject(t *testing.T) {
	t.Parallel()

	repo := &uploadRepoMock{}
	storage := &uploadStorageMock{}
	svc := services.NewUploadService(repo, storage, eventbus.NewEventPublisher(logrus.New()))

	entity := upload.NewWithID(
		12,
		uuid.Nil,
		"hash",
		"uploads/doc.pdf",
		"doc.pdf",
		"slug",
		42,
		nil,
		upload.UploadTypeDocument,
		time.Now(),
		time.Now(),
	)

	repo.On("GetByID", mock.Anything, uint(12)).Return(entity, nil).Once()
	repo.On("Delete", mock.Anything, uint(12)).Return(nil).Once()
	storage.On("Delete", mock.Anything, "uploads/doc.pdf").Return(nil).Once()

	deleted, err := svc.Delete(context.Background(), 12)
	require.NoError(t, err)
	require.Equal(t, uint(12), deleted.ID())
	repo.AssertExpectations(t)
	storage.AssertExpectations(t)
}
