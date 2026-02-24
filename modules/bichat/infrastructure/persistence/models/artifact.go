package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

var (
	ErrNilArtifactModel = errors.New("artifact model is nil")
	ErrNilArtifact      = errors.New("artifact is nil")
)

// ArtifactModel is the database model for bichat.artifacts.
type ArtifactModel struct {
	ID          string
	TenantID    string
	SessionID   string
	MessageID   *string
	UploadID    *int64
	Type        string
	Name        string
	Description *string
	MimeType    *string
	URL         *string
	SizeBytes   int64
	Metadata    []byte
	Status      string
	Idempotency *string
	CreatedAt   time.Time
}

// ToDomain converts the model to a domain Artifact.
func (m *ArtifactModel) ToDomain() (domain.Artifact, error) {
	if m == nil {
		return nil, ErrNilArtifactModel
	}
	id, err := uuid.Parse(m.ID)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(m.TenantID)
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.Parse(m.SessionID)
	if err != nil {
		return nil, err
	}
	var messageID *uuid.UUID
	if m.MessageID != nil && *m.MessageID != "" {
		parsed, err := uuid.Parse(*m.MessageID)
		if err != nil {
			return nil, err
		}
		messageID = &parsed
	}
	metadata := make(map[string]any)
	if len(m.Metadata) > 0 {
		if err := json.Unmarshal(m.Metadata, &metadata); err != nil {
			return nil, err
		}
	}
	opts := []domain.ArtifactOption{
		domain.WithArtifactID(id),
		domain.WithArtifactTenantID(tenantID),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactType(domain.ArtifactType(m.Type)),
		domain.WithArtifactName(m.Name),
		domain.WithArtifactMetadata(metadata),
		domain.WithArtifactSizeBytes(m.SizeBytes),
		domain.WithArtifactStatus(domain.ArtifactStatus(m.Status)),
		domain.WithArtifactCreatedAt(m.CreatedAt),
	}
	if m.Idempotency != nil {
		opts = append(opts, domain.WithArtifactIdempotencyKey(*m.Idempotency))
	}
	if messageID != nil {
		opts = append(opts, domain.WithArtifactMessageID(messageID))
	}
	if m.UploadID != nil {
		opts = append(opts, domain.WithArtifactUploadID(*m.UploadID))
	}
	if m.Description != nil {
		opts = append(opts, domain.WithArtifactDescription(*m.Description))
	}
	if m.MimeType != nil {
		opts = append(opts, domain.WithArtifactMimeType(*m.MimeType))
	}
	if m.URL != nil {
		opts = append(opts, domain.WithArtifactURL(*m.URL))
	}
	return domain.NewArtifact(opts...), nil
}

// ArtifactModelFromDomain converts a domain Artifact to the DB model.
func ArtifactModelFromDomain(a domain.Artifact) (*ArtifactModel, error) {
	if a == nil {
		return nil, ErrNilArtifact
	}
	m := &ArtifactModel{
		ID:        a.ID().String(),
		TenantID:  a.TenantID().String(),
		SessionID: a.SessionID().String(),
		Type:      string(a.Type()),
		Name:      a.Name(),
		SizeBytes: a.SizeBytes(),
		Status:    string(a.Status()),
		CreatedAt: a.CreatedAt(),
	}
	if key := a.IdempotencyKey(); key != "" {
		m.Idempotency = &key
	}
	if a.MessageID() != nil {
		s := a.MessageID().String()
		m.MessageID = &s
	}
	if a.UploadID() != nil {
		uploadID := *a.UploadID()
		m.UploadID = &uploadID
	}
	if a.Description() != "" {
		desc := a.Description()
		m.Description = &desc
	}
	if a.MimeType() != "" {
		mimeType := a.MimeType()
		m.MimeType = &mimeType
	}
	if a.URL() != "" {
		url := a.URL()
		m.URL = &url
	}
	if len(a.Metadata()) > 0 {
		metadata, err := json.Marshal(a.Metadata())
		if err != nil {
			return nil, err
		}
		m.Metadata = metadata
	}
	return m, nil
}
