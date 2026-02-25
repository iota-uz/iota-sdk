package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionModelRoundTrip(t *testing.T) {
	t.Parallel()

	parentID := uuid.New()
	prevRespID := "resp-123"
	createdAt := time.Now().UTC().Add(-time.Hour)
	updatedAt := time.Now().UTC()

	session, err := domain.NewSession(domain.SessionSpec{
		ID:                    uuid.New(),
		TenantID:              uuid.New(),
		OwnerUserID:           42,
		Title:                 "Revenue Insights",
		ParentSessionID:       &parentID,
		LLMPreviousResponseID: &prevRespID,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	})
	require.NoError(t, err)

	model, err := SessionModelFromDomain(session)
	require.NoError(t, err)

	restored, err := model.ToDomain()
	require.NoError(t, err)

	assert.Equal(t, session.ID(), restored.ID())
	assert.Equal(t, session.TenantID(), restored.TenantID())
	assert.Equal(t, session.UserID(), restored.UserID())
	assert.Equal(t, session.Title(), restored.Title())
	assert.Equal(t, session.Status(), restored.Status())
	assert.Equal(t, session.Pinned(), restored.Pinned())
	require.NotNil(t, restored.ParentSessionID())
	assert.Equal(t, *session.ParentSessionID(), *restored.ParentSessionID())
	require.NotNil(t, restored.LLMPreviousResponseID())
	assert.Equal(t, *session.LLMPreviousResponseID(), *restored.LLMPreviousResponseID())
	assert.Equal(t, session.CreatedAt(), restored.CreatedAt())
	assert.Equal(t, session.UpdatedAt(), restored.UpdatedAt())
}

func TestGenerationRunModelRoundTrip(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC().Add(-time.Minute)
	lastUpdatedAt := time.Now().UTC()

	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		ID:             uuid.New(),
		SessionID:      uuid.New(),
		TenantID:       uuid.New(),
		UserID:         7,
		Status:         domain.GenerationRunStatusStreaming,
		PartialContent: "partial response",
		PartialMetadata: map[string]any{
			"cursor": float64(12),
			"meta": map[string]any{
				"source": "test",
			},
		},
		StartedAt:     startedAt,
		LastUpdatedAt: lastUpdatedAt,
	})
	require.NoError(t, err)

	model, err := GenerationRunModelFromDomain(run)
	require.NoError(t, err)

	restored, err := model.ToDomain()
	require.NoError(t, err)

	assert.Equal(t, run.ID(), restored.ID())
	assert.Equal(t, run.SessionID(), restored.SessionID())
	assert.Equal(t, run.TenantID(), restored.TenantID())
	assert.Equal(t, run.UserID(), restored.UserID())
	assert.Equal(t, run.Status(), restored.Status())
	assert.Equal(t, run.PartialContent(), restored.PartialContent())
	assert.Equal(t, run.PartialMetadata(), restored.PartialMetadata())
	assert.Equal(t, run.StartedAt(), restored.StartedAt())
	assert.Equal(t, run.LastUpdatedAt(), restored.LastUpdatedAt())
}
