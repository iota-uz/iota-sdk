package services_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/exportconfig"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"net/url"
	"time"
)

// Mock dependencies
type MockUploadRepository struct {
	mock.Mock
}

func (m *MockUploadRepository) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *MockUploadRepository) GetByHash(ctx context.Context, hash string) (upload.Upload, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *MockUploadRepository) GetAll(ctx context.Context) ([]upload.Upload, error) {
	args := m.Called(ctx)
	return args.Get(0).([]upload.Upload), args.Error(1)
}

func (m *MockUploadRepository) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]upload.Upload), args.Error(1)
}

func (m *MockUploadRepository) Create(ctx context.Context, entity upload.Upload) (upload.Upload, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(upload.Upload), args.Error(1)
}

func (m *MockUploadRepository) Update(ctx context.Context, entity upload.Upload) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockUploadRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUploadRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockUploadRepository) Exists(ctx context.Context, id uint) (bool, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(bool), args.Error(1)
}

type MockUploadStorage struct {
	mock.Mock
}

func (m *MockUploadStorage) Save(ctx context.Context, path string, data []byte) error {
	args := m.Called(ctx, path, data)
	return args.Error(0)
}

func (m *MockUploadStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockUploadStorage) Get(ctx context.Context, path string) ([]byte, error) {
	args := m.Called(ctx, path)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockUploadStorage) Open(ctx context.Context, path string) ([]byte, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

type MockUpload struct {
	mock.Mock
}

func (m *MockUpload) ID() uint {
	return m.Called().Get(0).(uint)
}

func (m *MockUpload) TenantID() uuid.UUID {
	return m.Called().Get(0).(uuid.UUID)
}

func (m *MockUpload) Type() upload.UploadType {
	return m.Called().Get(0).(upload.UploadType)
}

func (m *MockUpload) Hash() string {
	return m.Called().String(0)
}

func (m *MockUpload) Path() string {
	return m.Called().String(0)
}

func (m *MockUpload) Name() string {
	return m.Called().String(0)
}

func (m *MockUpload) Size() upload.Size {
	return m.Called().Get(0).(upload.Size)
}

func (m *MockUpload) IsImage() bool {
	return m.Called().Bool(0)
}

func (m *MockUpload) PreviewURL() string {
	return m.Called().String(0)
}

func (m *MockUpload) URL() *url.URL {
	args := m.Called()
	return args.Get(0).(*url.URL)
}

func (m *MockUpload) Mimetype() *mimetype.MIME {
	args := m.Called()
	return args.Get(0).(*mimetype.MIME)
}

func (m *MockUpload) CreatedAt() time.Time {
	return m.Called().Get(0).(time.Time)
}

func (m *MockUpload) UpdatedAt() time.Time {
	return m.Called().Get(0).(time.Time)
}

// TestExcelExportService tests
func TestExcelExportService_ExportFromDataSource(t *testing.T) {
	// Create mocks
	mockRepo := new(MockUploadRepository)
	mockStorage := new(MockUploadStorage)
	logger := logrus.New()
	mockEventBus := eventbus.NewEventPublisher(logger)

	// Create upload service
	uploadService := services.NewUploadService(mockRepo, mockStorage, mockEventBus)

	// Create Excel export service (with nil DB since we're using custom datasource)
	excelService := services.NewExcelExportService(nil, uploadService)

	// Create mock datasource
	headers := []string{"id", "name", "email"}
	rows := [][]interface{}{
		{1, "John Doe", "john@example.com"},
		{2, "Jane Smith", "jane@example.com"},
	}
	datasource := &mockDataSource{
		headers: headers,
		rows:    rows,
	}

	// Setup mock expectations
	mockUpload := new(MockUpload)
	mockUpload.On("ID").Return(uint(1))
	mockUpload.On("Name").Return("users.xlsx")
	mockUpload.On("Hash").Return("abc123")
	mockUpload.On("Path").Return("uploads/abc123.xlsx")

	mockRepo.On("GetByHash", mock.Anything, mock.Anything).Return(nil, persistence.ErrUploadNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(mockUpload, nil)
	mockStorage.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Test export
	ctx := context.Background()
	config := exportconfig.New(exportconfig.WithFilename("users"))
	result, err := excelService.ExportFromDataSource(ctx, datasource, config)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(1), result.ID())
	assert.Equal(t, "users.xlsx", result.Name())

	// Verify all expectations were met
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// mockDataSource for testing
type mockDataSource struct {
	headers []string
	rows    [][]interface{}
	index   int
}

func (m *mockDataSource) GetHeaders() []string {
	return m.headers
}

func (m *mockDataSource) GetSheetName() string {
	return "TestSheet"
}

func (m *mockDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	return func() ([]interface{}, error) {
		if m.index >= len(m.rows) {
			return nil, nil
		}
		row := m.rows[m.index]
		m.index++
		return row, nil
	}, nil
}

func TestExcelExportService_ExportFromDataSourceWithOptions(t *testing.T) {
	// Create mocks
	mockRepo := new(MockUploadRepository)
	mockStorage := new(MockUploadStorage)
	logger := logrus.New()
	mockEventBus := eventbus.NewEventPublisher(logger)

	// Create services
	uploadService := services.NewUploadService(mockRepo, mockStorage, mockEventBus)
	excelService := services.NewExcelExportService(nil, uploadService)

	// Create mock datasource
	headers := []string{"id", "name", "score"}
	rows := [][]interface{}{
		{1, "John", 95.5},
		{2, "Jane", 87.3},
	}
	datasource := &mockDataSource{
		headers: headers,
		rows:    rows,
	}

	// Configure export options
	exportOpts := &excel.ExportOptions{
		IncludeHeaders: true,
		AutoFilter:     true,
		FreezeHeader:   true,
		MaxRows:        100,
	}

	styleOpts := excel.DefaultStyleOptions()

	// Setup mock expectations
	mockUpload := new(MockUpload)
	mockUpload.On("ID").Return(uint(2))
	mockUpload.On("Name").Return("scores.xlsx")

	mockRepo.On("GetByHash", mock.Anything, mock.Anything).Return(nil, persistence.ErrUploadNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(mockUpload, nil)
	mockStorage.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Test export with options
	ctx := context.Background()
	config := exportconfig.New(
		exportconfig.WithFilename("scores"),
		exportconfig.WithExportOptions(exportOpts),
		exportconfig.WithStyleOptions(styleOpts),
	)
	result, err := excelService.ExportFromDataSource(ctx, datasource, config)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(2), result.ID())
	assert.Equal(t, "scores.xlsx", result.Name())

	// Verify expectations
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestExcelExportService_ExportFromDataSource_EmptyFilename(t *testing.T) {
	// Create mocks
	mockRepo := new(MockUploadRepository)
	mockStorage := new(MockUploadStorage)
	logger := logrus.New()
	mockEventBus := eventbus.NewEventPublisher(logger)

	// Create services
	uploadService := services.NewUploadService(mockRepo, mockStorage, mockEventBus)
	excelService := services.NewExcelExportService(nil, uploadService)

	// Create mock datasource
	datasource := &mockDataSource{
		headers: []string{"id"},
		rows:    [][]interface{}{{1}},
	}

	// Setup mock expectations
	mockUpload := new(MockUpload)
	mockUpload.On("ID").Return(uint(3))
	mockUpload.On("Name").Return(mock.Anything)

	mockRepo.On("GetByHash", mock.Anything, mock.Anything).Return(nil, persistence.ErrUploadNotFound)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(mockUpload, nil)
	mockStorage.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Test export with empty filename
	ctx := context.Background()
	config := exportconfig.New() // No filename provided, will use default
	result, err := excelService.ExportFromDataSource(ctx, datasource, config)

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify the created upload has a generated filename
	createCall := mockRepo.Calls[1] // Second call is Create
	createdEntity := createCall.Arguments[1]
	assert.Contains(t, createdEntity.(upload.Upload).Name(), "export_")
	assert.Contains(t, createdEntity.(upload.Upload).Name(), ".xlsx")
}

func TestExcelExportService_ExportError(t *testing.T) {
	// Create mocks
	mockRepo := new(MockUploadRepository)
	mockStorage := new(MockUploadStorage)
	logger := logrus.New()
	mockEventBus := eventbus.NewEventPublisher(logger)

	// Create services
	uploadService := services.NewUploadService(mockRepo, mockStorage, mockEventBus)
	excelService := services.NewExcelExportService(nil, uploadService)

	// Create a datasource that returns error
	datasource := &errorDataSource{}

	// Test export with error
	ctx := context.Background()
	config := exportconfig.New(exportconfig.WithFilename("test"))
	_, err := excelService.ExportFromDataSource(ctx, datasource, config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export to Excel")
}

// errorDataSource is a DataSource that returns errors
type errorDataSource struct{}

func (e *errorDataSource) GetHeaders() []string {
	return []string{}
}

func (e *errorDataSource) GetSheetName() string {
	return "Error"
}

func (e *errorDataSource) GetRows(ctx context.Context) (func() ([]interface{}, error), error) {
	return nil, fmt.Errorf("datasource error")
}
