package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewAttachment(t *testing.T) {
	t.Parallel()

	t.Run("creates attachment with defaults", func(t *testing.T) {
		att := NewAttachment()

		if att.ID() == uuid.Nil {
			t.Error("Expected non-nil ID")
		}
		if att.MessageID() != uuid.Nil {
			t.Error("Expected nil MessageID by default")
		}
		if att.FileName() != "" {
			t.Error("Expected empty FileName by default")
		}
		if att.MimeType() != "" {
			t.Error("Expected empty MimeType by default")
		}
		if att.SizeBytes() != 0 {
			t.Error("Expected 0 SizeBytes by default")
		}
		if att.FilePath() != "" {
			t.Error("Expected empty FilePath by default")
		}
		if att.CreatedAt().IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
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

		if att.ID() != customID {
			t.Errorf("Expected ID %s, got %s", customID, att.ID())
		}
		if att.MessageID() != msgID {
			t.Errorf("Expected MessageID %s, got %s", msgID, att.MessageID())
		}
		if att.FileName() != "test.png" {
			t.Errorf("Expected FileName 'test.png', got '%s'", att.FileName())
		}
		if att.MimeType() != "image/png" {
			t.Errorf("Expected MimeType 'image/png', got '%s'", att.MimeType())
		}
		if att.SizeBytes() != 2048 {
			t.Errorf("Expected SizeBytes 2048, got %d", att.SizeBytes())
		}
		if att.FilePath() != "/path/to/file" {
			t.Errorf("Expected FilePath '/path/to/file', got '%s'", att.FilePath())
		}
		if !att.CreatedAt().Equal(customTime) {
			t.Errorf("Expected CreatedAt %v, got %v", customTime, att.CreatedAt())
		}
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
		att := NewAttachment(WithMimeType(tt.mimeType))
		if att.IsImage() != tt.expected {
			t.Errorf("IsImage() for %q: expected %v, got %v", tt.mimeType, tt.expected, att.IsImage())
		}
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
		att := NewAttachment(WithMimeType(tt.mimeType))
		if att.IsDocument() != tt.expected {
			t.Errorf("IsDocument() for %q: expected %v, got %v", tt.mimeType, tt.expected, att.IsDocument())
		}
	}
}

func TestAttachment_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// This test ensures Attachment is an interface and the implementation satisfies it
	var _ Attachment = NewAttachment()
}
