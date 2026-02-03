package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ArtifactType is a string to allow extension without code changes.
type ArtifactType string

const (
	ArtifactTypeCodeOutput ArtifactType = "code_output"
	ArtifactTypeChart      ArtifactType = "chart"
	ArtifactTypeExport     ArtifactType = "export"
)

// Artifact represents any generated output from a chat session.
type Artifact struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	SessionID   uuid.UUID
	MessageID   *uuid.UUID
	Type        ArtifactType
	Name        string
	Description string
	MimeType    string
	URL         string
	SizeBytes   int64
	Metadata    map[string]any
	CreatedAt   time.Time
}

// ArtifactOption is a functional option for creating artifacts.
type ArtifactOption func(*Artifact)

// NewArtifact creates a new artifact with the given options.
func NewArtifact(opts ...ArtifactOption) *Artifact {
	a := &Artifact{
		ID:        uuid.New(),
		Metadata:  make(map[string]any),
		CreatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithArtifactID sets the artifact ID.
func WithArtifactID(id uuid.UUID) ArtifactOption {
	return func(a *Artifact) {
		a.ID = id
	}
}

// WithArtifactTenantID sets the tenant ID.
func WithArtifactTenantID(tenantID uuid.UUID) ArtifactOption {
	return func(a *Artifact) {
		a.TenantID = tenantID
	}
}

// WithArtifactSessionID sets the session ID.
func WithArtifactSessionID(sessionID uuid.UUID) ArtifactOption {
	return func(a *Artifact) {
		a.SessionID = sessionID
	}
}

// WithArtifactMessageID sets the optional message ID.
func WithArtifactMessageID(messageID *uuid.UUID) ArtifactOption {
	return func(a *Artifact) {
		a.MessageID = messageID
	}
}

// WithArtifactType sets the artifact type.
func WithArtifactType(t ArtifactType) ArtifactOption {
	return func(a *Artifact) {
		a.Type = t
	}
}

// WithArtifactName sets the display name.
func WithArtifactName(name string) ArtifactOption {
	return func(a *Artifact) {
		a.Name = name
	}
}

// WithArtifactDescription sets the optional description.
func WithArtifactDescription(desc string) ArtifactOption {
	return func(a *Artifact) {
		a.Description = desc
	}
}

// WithArtifactMimeType sets the MIME type.
func WithArtifactMimeType(mimeType string) ArtifactOption {
	return func(a *Artifact) {
		a.MimeType = mimeType
	}
}

// WithArtifactURL sets the storage URL.
func WithArtifactURL(url string) ArtifactOption {
	return func(a *Artifact) {
		a.URL = url
	}
}

// WithArtifactSizeBytes sets the file size.
func WithArtifactSizeBytes(size int64) ArtifactOption {
	return func(a *Artifact) {
		a.SizeBytes = size
	}
}

// WithArtifactMetadata sets the type-specific metadata.
func WithArtifactMetadata(m map[string]any) ArtifactOption {
	return func(a *Artifact) {
		if m != nil {
			a.Metadata = m
		}
	}
}

// WithArtifactCreatedAt sets the created timestamp.
func WithArtifactCreatedAt(t time.Time) ArtifactOption {
	return func(a *Artifact) {
		a.CreatedAt = t
	}
}

// HasFile returns true if the artifact has an associated file (URL set).
func (a *Artifact) HasFile() bool {
	return a != nil && a.URL != ""
}

// IsPreviewable returns true for image/* MIME types or chart type.
func (a *Artifact) IsPreviewable() bool {
	if a == nil {
		return false
	}
	if a.Type == ArtifactTypeChart {
		return true
	}
	return strings.HasPrefix(a.MimeType, "image/")
}

// GetMetadataString returns a string value from metadata, or empty string.
func (a *Artifact) GetMetadataString(key string) string {
	if a == nil || a.Metadata == nil {
		return ""
	}
	v, ok := a.Metadata[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// GetMetadataInt returns an int value from metadata, or 0.
func (a *Artifact) GetMetadataInt(key string) int {
	if a == nil || a.Metadata == nil {
		return 0
	}
	v, ok := a.Metadata[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}
