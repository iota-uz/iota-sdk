package persistence_test

import (
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGormUploadRepository_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t)

	uploadRepository := persistence.NewUploadRepository()

	t.Run("Create", func(t *testing.T) {
		// Create test data
		mime := mimetype.Lookup("image/jpeg")
		uploadData := upload.New(
			"test-hash",
			"uploads/test.jpg",
			"test.jpg",
			1024,
			mime,
		)

		// Create upload
		createdUpload, err := uploadRepository.Create(f.ctx, uploadData)
		require.NoError(t, err)
		assert.NotEqual(t, uint(0), createdUpload.ID())
		assert.Equal(t, "test-hash", createdUpload.Hash())
		assert.Equal(t, "uploads/test.jpg", createdUpload.Path())
		assert.Equal(t, "test.jpg", createdUpload.Name())
		assert.Equal(t, 1024, createdUpload.Size().Bytes())
		assert.Equal(t, "image/jpeg", createdUpload.Mimetype().String())
		assert.Equal(t, upload.UploadTypeImage, createdUpload.Type())
		assert.True(t, createdUpload.IsImage())
		assert.NotZero(t, createdUpload.CreatedAt())
		assert.NotZero(t, createdUpload.UpdatedAt())
	})

	t.Run("GetByID", func(t *testing.T) {
		// Create test data
		mime := mimetype.Lookup("application/pdf")
		uploadData := upload.New(
			"pdf-hash",
			"uploads/document.pdf",
			"document.pdf",
			2048,
			mime,
		)

		// Create upload
		createdUpload, err := uploadRepository.Create(f.ctx, uploadData)
		require.NoError(t, err)

		// Get upload by ID
		retrievedUpload, err := uploadRepository.GetByID(f.ctx, createdUpload.ID())
		require.NoError(t, err)
		assert.Equal(t, createdUpload.ID(), retrievedUpload.ID())
		assert.Equal(t, "pdf-hash", retrievedUpload.Hash())
		assert.Equal(t, "uploads/document.pdf", retrievedUpload.Path())
		assert.Equal(t, "document.pdf", retrievedUpload.Name())
		assert.Equal(t, 2048, retrievedUpload.Size().Bytes())
		assert.Equal(t, "application/pdf", retrievedUpload.Mimetype().String())
		assert.Equal(t, upload.UploadTypeDocument, retrievedUpload.Type())
		assert.False(t, retrievedUpload.IsImage())
	})

	t.Run("GetByHash", func(t *testing.T) {
		// Create test data with unique hash
		uniqueHash := "unique-hash-" + time.Now().Format("20060102150405")
		mime := mimetype.Lookup("text/plain")
		uploadData := upload.New(
			uniqueHash,
			"uploads/text.txt",
			"text.txt",
			512,
			mime,
		)

		// Create upload
		_, err := uploadRepository.Create(f.ctx, uploadData)
		require.NoError(t, err)

		// Get upload by hash
		retrievedUpload, err := uploadRepository.GetByHash(f.ctx, uniqueHash)
		require.NoError(t, err)
		assert.Equal(t, uniqueHash, retrievedUpload.Hash())
		assert.Equal(t, "uploads/text.txt", retrievedUpload.Path())
		assert.Equal(t, "text.txt", retrievedUpload.Name())
		assert.Equal(t, 512, retrievedUpload.Size().Bytes())
		assert.Equal(t, "text/plain", retrievedUpload.Mimetype().String())
		assert.Equal(t, upload.UploadTypeDocument, retrievedUpload.Type())
	})

	t.Run("GetAll", func(t *testing.T) {
		// Get all uploads
		uploads, err := uploadRepository.GetAll(f.ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, uploads)
	})

	t.Run("Count", func(t *testing.T) {
		// Count uploads
		count, err := uploadRepository.Count(f.ctx)
		require.NoError(t, err)
		assert.NotZero(t, count)
	})

	t.Run("Update", func(t *testing.T) {
		// Create test data
		mime := mimetype.Lookup("image/png")
		uploadData := upload.New(
			"update-hash",
			"uploads/update.png",
			"update.png",
			4096,
			mime,
		)

		// Create upload
		createdUpload, err := uploadRepository.Create(f.ctx, uploadData)
		require.NoError(t, err)

		// Create a new upload with the same ID for update
		updatedMime := mimetype.Lookup("image/png")
		updatedUpload := upload.NewWithID(
			createdUpload.ID(),
			"updated-hash",
			"uploads/updated.png",
			"updated.png",
			8192,
			updatedMime,
			upload.UploadTypeImage,
			createdUpload.CreatedAt(),
			time.Now(),
		)

		// Update upload
		err = uploadRepository.Update(f.ctx, updatedUpload)
		require.NoError(t, err)

		// Get updated upload
		retrievedUpload, err := uploadRepository.GetByID(f.ctx, createdUpload.ID())
		require.NoError(t, err)
		assert.Equal(t, "updated-hash", retrievedUpload.Hash())
		assert.Equal(t, "uploads/updated.png", retrievedUpload.Path())
		assert.Equal(t, "updated.png", retrievedUpload.Name())
		assert.Equal(t, 8192, retrievedUpload.Size().Bytes())
	})

	t.Run("GetPaginated", func(t *testing.T) {
		// Create multiple test uploads
		for i := 0; i < 5; i++ {
			mime := mimetype.Lookup("image/jpeg")
			uploadData := upload.New(
				"page-hash-"+time.Now().Format("20060102150405")+"-"+string(rune(i+48)),
				"uploads/page"+string(rune(i+48))+".jpg",
				"page"+string(rune(i+48))+".jpg",
				1024*(i+1),
				mime,
			)
			_, err := uploadRepository.Create(f.ctx, uploadData)
			require.NoError(t, err)
		}

		// Test pagination
		params := &upload.FindParams{
			Limit:  3,
			Offset: 0,
			SortBy: upload.SortBy{
				Fields: []upload.SortByField{{
					Field:     upload.FieldSize,
					Ascending: true,
				}},
			},
		}

		uploads, err := uploadRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(uploads), 3)

		// Verify sorting by size (ascending)
		for i := 0; i < len(uploads)-1; i++ {
			assert.LessOrEqual(t, uploads[i].Size().Bytes(), uploads[i+1].Size().Bytes())
		}

		// Test with different sort order
		params.SortBy.Fields[0].Ascending = false
		uploadsDesc, err := uploadRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(uploadsDesc), 3)

		// Verify sorting by size (descending)
		for i := 0; i < len(uploadsDesc)-1; i++ {
			assert.GreaterOrEqual(t, uploadsDesc[i].Size().Bytes(), uploadsDesc[i+1].Size().Bytes())
		}

		// Test filtering by mimetype
		params = &upload.FindParams{
			Mimetype: mimetype.Lookup("image/jpeg"),
			Limit:    10,
			SortBy: upload.SortBy{
				Fields: []upload.SortByField{{
					Field:     upload.FieldName,
					Ascending: true,
				}},
			},
		}

		imageUploads, err := uploadRepository.GetPaginated(f.ctx, params)
		require.NoError(t, err)
		for _, u := range imageUploads {
			assert.True(t, u.IsImage())
			assert.Equal(t, "image/jpeg", u.Mimetype().String())
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Create test data
		mime := mimetype.Lookup("image/gif")
		uploadData := upload.New(
			"delete-hash",
			"uploads/delete.gif",
			"delete.gif",
			2048,
			mime,
		)

		// Create upload
		createdUpload, err := uploadRepository.Create(f.ctx, uploadData)
		require.NoError(t, err)

		// Delete upload
		err = uploadRepository.Delete(f.ctx, createdUpload.ID())
		require.NoError(t, err)

		// Try to get deleted upload
		_, err = uploadRepository.GetByID(f.ctx, createdUpload.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrUploadNotFound)
	})

	t.Run("NotFound", func(t *testing.T) {
		// Try to get non-existent upload
		_, err := uploadRepository.GetByID(f.ctx, 99999)
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrUploadNotFound)

		// Try to get upload with non-existent hash
		_, err = uploadRepository.GetByHash(f.ctx, "non-existent-hash")
		require.Error(t, err)
		require.ErrorIs(t, err, persistence.ErrUploadNotFound)
	})
}
