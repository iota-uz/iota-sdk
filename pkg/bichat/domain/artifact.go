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
	ArtifactTypeTable      ArtifactType = "table"
	ArtifactTypeExport     ArtifactType = "export"
	ArtifactTypeAttachment ArtifactType = "attachment"
)

type ArtifactStatus string

const (
	ArtifactStatusPendingUpload ArtifactStatus = "pending_upload"
	ArtifactStatusAvailable     ArtifactStatus = "available"
	ArtifactStatusFailed        ArtifactStatus = "failed"
	ArtifactStatusDeleted       ArtifactStatus = "deleted"
)

// Artifact represents any generated output from a chat session.
// Interface following the same design as other aggregates (e.g. Session).
type Artifact interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	SessionID() uuid.UUID
	MessageID() *uuid.UUID
	UploadID() *int64
	Type() ArtifactType
	Name() string
	Description() string
	MimeType() string
	URL() string
	SizeBytes() int64
	Metadata() map[string]any
	Status() ArtifactStatus
	IdempotencyKey() string
	CreatedAt() time.Time

	HasFile() bool
	IsPreviewable() bool
	GetMetadataString(key string) string
	GetMetadataInt(key string) int
}

type artifact struct {
	id           uuid.UUID
	tenantID     uuid.UUID
	sessionID    uuid.UUID
	messageID    *uuid.UUID
	uploadID     *int64
	artifactType ArtifactType
	name         string
	description  string
	mimeType     string
	url          string
	sizeBytes    int64
	metadata     map[string]any
	status       ArtifactStatus
	idempotency  string
	createdAt    time.Time
}

// ArtifactOption is a functional option for creating artifacts.
type ArtifactOption func(*artifact)

// NewArtifact creates a new artifact with the given options.
func NewArtifact(opts ...ArtifactOption) Artifact {
	a := &artifact{
		id:          uuid.New(),
		metadata:    make(map[string]any),
		status:      ArtifactStatusAvailable,
		idempotency: "",
		createdAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithArtifactID sets the artifact ID.
func WithArtifactID(id uuid.UUID) ArtifactOption {
	return func(a *artifact) {
		a.id = id
	}
}

// WithArtifactTenantID sets the tenant ID.
func WithArtifactTenantID(tenantID uuid.UUID) ArtifactOption {
	return func(a *artifact) {
		a.tenantID = tenantID
	}
}

// WithArtifactSessionID sets the session ID.
func WithArtifactSessionID(sessionID uuid.UUID) ArtifactOption {
	return func(a *artifact) {
		a.sessionID = sessionID
	}
}

// WithArtifactMessageID sets the optional message ID.
func WithArtifactMessageID(messageID *uuid.UUID) ArtifactOption {
	return func(a *artifact) {
		a.messageID = messageID
	}
}

// WithArtifactUploadID sets the optional upload ID.
func WithArtifactUploadID(uploadID int64) ArtifactOption {
	return func(a *artifact) {
		a.uploadID = &uploadID
	}
}

// WithArtifactType sets the artifact type.
func WithArtifactType(t ArtifactType) ArtifactOption {
	return func(a *artifact) {
		a.artifactType = t
	}
}

// WithArtifactName sets the display name.
func WithArtifactName(name string) ArtifactOption {
	return func(a *artifact) {
		a.name = name
	}
}

// WithArtifactDescription sets the optional description.
func WithArtifactDescription(desc string) ArtifactOption {
	return func(a *artifact) {
		a.description = desc
	}
}

// WithArtifactMimeType sets the MIME type.
func WithArtifactMimeType(mimeType string) ArtifactOption {
	return func(a *artifact) {
		a.mimeType = mimeType
	}
}

// WithArtifactURL sets the storage URL.
func WithArtifactURL(url string) ArtifactOption {
	return func(a *artifact) {
		a.url = url
	}
}

// WithArtifactSizeBytes sets the file size.
func WithArtifactSizeBytes(size int64) ArtifactOption {
	return func(a *artifact) {
		a.sizeBytes = size
	}
}

// WithArtifactMetadata sets the type-specific metadata.
func WithArtifactMetadata(m map[string]any) ArtifactOption {
	return func(a *artifact) {
		if m != nil {
			a.metadata = m
		}
	}
}

func WithArtifactStatus(status ArtifactStatus) ArtifactOption {
	return func(a *artifact) {
		a.status = status
	}
}

func WithArtifactIdempotencyKey(key string) ArtifactOption {
	return func(a *artifact) {
		a.idempotency = strings.TrimSpace(key)
	}
}

// WithArtifactCreatedAt sets the created timestamp.
func WithArtifactCreatedAt(t time.Time) ArtifactOption {
	return func(a *artifact) {
		a.createdAt = t
	}
}

// Getter methods implementing the Artifact interface
func (a *artifact) ID() uuid.UUID {
	return a.id
}

func (a *artifact) TenantID() uuid.UUID {
	return a.tenantID
}

func (a *artifact) SessionID() uuid.UUID {
	return a.sessionID
}

func (a *artifact) MessageID() *uuid.UUID {
	return a.messageID
}

func (a *artifact) UploadID() *int64 {
	return a.uploadID
}

func (a *artifact) Type() ArtifactType {
	return a.artifactType
}

func (a *artifact) Name() string {
	return a.name
}

func (a *artifact) Description() string {
	return a.description
}

func (a *artifact) MimeType() string {
	return a.mimeType
}

func (a *artifact) URL() string {
	return a.url
}

func (a *artifact) SizeBytes() int64 {
	return a.sizeBytes
}

func (a *artifact) Metadata() map[string]any {
	return a.metadata
}

func (a *artifact) Status() ArtifactStatus {
	return a.status
}

func (a *artifact) IdempotencyKey() string {
	return a.idempotency
}

func (a *artifact) CreatedAt() time.Time {
	return a.createdAt
}

// HasFile returns true if the artifact has an associated file (URL set).
func (a *artifact) HasFile() bool {
	return a != nil && a.url != ""
}

// IsPreviewable returns true for image/* MIME types or chart type.
func (a *artifact) IsPreviewable() bool {
	if a == nil {
		return false
	}
	if a.artifactType == ArtifactTypeChart {
		return true
	}
	return strings.HasPrefix(a.mimeType, "image/")
}

// GetMetadataString returns a string value from metadata, or empty string.
func (a *artifact) GetMetadataString(key string) string {
	if a == nil || a.metadata == nil {
		return ""
	}
	v, ok := a.metadata[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// GetMetadataInt returns an int value from metadata, or 0.
func (a *artifact) GetMetadataInt(key string) int {
	if a == nil || a.metadata == nil {
		return 0
	}
	v, ok := a.metadata[key]
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
