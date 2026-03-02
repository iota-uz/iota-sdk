// Package models provides this package.
package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

var (
	ErrNilGenerationRunModel = errors.New("generation run model is nil")
	ErrNilGenerationRun      = errors.New("generation run is nil")
)

// GenerationRunModel is the database model for bichat.generation_runs.
type GenerationRunModel struct {
	ID             uuid.UUID
	SessionID      uuid.UUID
	TenantID       uuid.UUID
	UserID         int64
	Status         string
	PartialContent string
	PartialMeta    []byte
	StartedAt      time.Time
	LastUpdatedAt  time.Time
}

// ToDomain converts the model to a domain GenerationRun.
func (m *GenerationRunModel) ToDomain() (domain.GenerationRun, error) {
	if m == nil {
		return nil, ErrNilGenerationRunModel
	}

	meta := make(map[string]any)
	if len(m.PartialMeta) > 0 {
		if err := json.Unmarshal(m.PartialMeta, &meta); err != nil {
			return nil, err
		}
	}

	return domain.RehydrateGenerationRun(domain.GenerationRunSpec{
		ID:              m.ID,
		SessionID:       m.SessionID,
		TenantID:        m.TenantID,
		UserID:          m.UserID,
		Status:          domain.GenerationRunStatus(m.Status),
		PartialContent:  m.PartialContent,
		PartialMetadata: meta,
		StartedAt:       m.StartedAt,
		LastUpdatedAt:   m.LastUpdatedAt,
	})
}

// GenerationRunModelFromDomain converts a domain GenerationRun to DB model.
func GenerationRunModelFromDomain(run domain.GenerationRun) (*GenerationRunModel, error) {
	if run == nil {
		return nil, ErrNilGenerationRun
	}

	meta, err := json.Marshal(run.PartialMetadata())
	if err != nil {
		return nil, err
	}

	return &GenerationRunModel{
		ID:             run.ID(),
		SessionID:      run.SessionID(),
		TenantID:       run.TenantID(),
		UserID:         run.UserID(),
		Status:         string(run.Status()),
		PartialContent: run.PartialContent(),
		PartialMeta:    meta,
		StartedAt:      run.StartedAt(),
		LastUpdatedAt:  run.LastUpdatedAt(),
	}, nil
}
