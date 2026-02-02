package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

// ArtifactModel is the database model for bichat.artifacts.
type ArtifactModel struct {
	ID          string
	TenantID    string
	SessionID   string
	MessageID   *string
	Type        string
	Name        string
	Description *string
	MimeType    *string
	URL         *string
	SizeBytes   int64
	Metadata    []byte
	CreatedAt   time.Time
}

// ToDomain converts the model to a domain Artifact.
func (m *ArtifactModel) ToDomain() (*domain.Artifact, error) {
	if m == nil {
		return nil, nil
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
	a := &domain.Artifact{
		ID:        id,
		TenantID:  tenantID,
		SessionID: sessionID,
		MessageID: messageID,
		Type:      domain.ArtifactType(m.Type),
		Name:      m.Name,
		Metadata:  metadata,
		SizeBytes: m.SizeBytes,
		CreatedAt: m.CreatedAt,
	}
	if m.Description != nil {
		a.Description = *m.Description
	}
	if m.MimeType != nil {
		a.MimeType = *m.MimeType
	}
	if m.URL != nil {
		a.URL = *m.URL
	}
	return a, nil
}

// ArtifactModelFromDomain converts a domain Artifact to the DB model.
func ArtifactModelFromDomain(a *domain.Artifact) (*ArtifactModel, error) {
	if a == nil {
		return nil, nil
	}
	m := &ArtifactModel{
		ID:        a.ID.String(),
		TenantID:  a.TenantID.String(),
		SessionID: a.SessionID.String(),
		Type:      string(a.Type),
		Name:      a.Name,
		SizeBytes: a.SizeBytes,
		CreatedAt: a.CreatedAt,
	}
	if a.MessageID != nil {
		s := a.MessageID.String()
		m.MessageID = &s
	}
	if a.Description != "" {
		m.Description = &a.Description
	}
	if a.MimeType != "" {
		m.MimeType = &a.MimeType
	}
	if a.URL != "" {
		m.URL = &a.URL
	}
	if len(a.Metadata) > 0 {
		metadata, err := json.Marshal(a.Metadata)
		if err != nil {
			return nil, err
		}
		m.Metadata = metadata
	}
	return m, nil
}
