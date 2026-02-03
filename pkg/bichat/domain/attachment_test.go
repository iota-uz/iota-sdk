package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAttachment(t *testing.T) {
	t.Parallel()

	t.Run("creates attachment with defaults", func(t *testing.T) {
		att := NewAttachment()

		require.NotEqual(t, uuid.Nil, att.ID(), "expected non-nil ID")
		assert.Equal(t, uuid.Nil, att.MessageID(), "expected nil MessageID by default")
		assert.Empty(t, att.FileName(), "expected empty FileName by default")
		assert.Empty(t, att.MimeType(), "expected empty MimeType by default")
		assert.Equal(t, int64(0), att.SizeBytes(), "expected 0 SizeBytes by default")
		assert.Empty(t, att.FilePath(), "expected empty FilePath by default")
		assert.False(t, att.CreatedAt().IsZero(), "expected CreatedAt to be set")
	})

	t.Run("creates attachment with options", func(t *testing.T) {
		msgID := uuid.New()
		customID := uuid.New()
		customTime := time.Now().Add(-time.Hour)

		att := NewAttachment(
			WithAttachmentID(customID),
			WithAttachmentMessageID(msgID),
			WithFileName("test.png"),
			WithMimeType("image/png"),
			WithSizeBytes(2048),
			WithFilePath("/path/to/file"),
			WithAttachmentCreatedAt(customTime),
		)

		require.Equal(t, customID, att.ID())
		require.Equal(t, msgID, att.MessageID())
		assert.Equal(t, "test.png", att.FileName())
		assert.Equal(t, "image/png", att.MimeType())
		assert.Equal(t, int64(2048), att.SizeBytes())
		assert.Equal(t, "/path/to/file", att.FilePath())
		assert.True(t, att.CreatedAt().Equal(customTime), "expected CreatedAt to match custom time")
	})
}

func TestAttachment_IsImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		expected bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/jpg", true},
		{"image/gif", true},
		{"image/webp", true},
		{"application/pdf", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			att := NewAttachment(WithMimeType(tt.mimeType))
			assert.Equal(t, tt.expected, att.IsImage(), "IsImage() for mimeType %q", tt.mimeType)
		})
	}
}

func TestAttachment_IsDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mimeType string
		expected bool
	}{
		{"application/pdf", true},
		{"application/msword", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/vnd.ms-excel", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"image/png", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			att := NewAttachment(WithMimeType(tt.mimeType))
			assert.Equal(t, tt.expected, att.IsDocument(), "IsDocument() for mimeType %q", tt.mimeType)
		})
	}
}

func TestNewAttachment_ReturnsConcreteType(t *testing.T) {
	t.Parallel()

	att := NewAttachment()
	require.NotNil(t, att)
	require.NotEqual(t, uuid.Nil, att.ID())
}
