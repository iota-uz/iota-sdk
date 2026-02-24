package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresChatRepository_SaveMessage_WithCodeOutputs_RoundTrip(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Code Outputs"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	base := time.Now().Add(-10 * time.Minute)
	msgID := uuid.New()

	out1 := types.CodeInterpreterOutput{
		ID:        uuid.New(),
		MessageID: msgID,
		Name:      "chart.png",
		MimeType:  "image/png",
		URL:       "/files/chart.png",
		Size:      2048,
		CreatedAt: base.Add(1 * time.Second),
	}
	out2 := types.CodeInterpreterOutput{
		ID:        uuid.New(),
		MessageID: msgID,
		Name:      "data.csv",
		MimeType:  "text/csv",
		URL:       "/files/data.csv",
		Size:      1024,
		CreatedAt: base.Add(2 * time.Second),
	}

	msg := types.AssistantMessage(
		"Generated outputs",
		types.WithMessageID(msgID),
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base),
	)
	require.NoError(t, repo.SaveMessage(env.Ctx, msg))

	artifact1 := domain.NewArtifact(
		domain.WithArtifactID(out1.ID),
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactMessageID(&msgID),
		domain.WithArtifactType(domain.ArtifactTypeCodeOutput),
		domain.WithArtifactName(out1.Name),
		domain.WithArtifactMimeType(out1.MimeType),
		domain.WithArtifactURL(out1.URL),
		domain.WithArtifactSizeBytes(out1.Size),
		domain.WithArtifactCreatedAt(out1.CreatedAt),
	)
	artifact2 := domain.NewArtifact(
		domain.WithArtifactID(out2.ID),
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactMessageID(&msgID),
		domain.WithArtifactType(domain.ArtifactTypeCodeOutput),
		domain.WithArtifactName(out2.Name),
		domain.WithArtifactMimeType(out2.MimeType),
		domain.WithArtifactURL(out2.URL),
		domain.WithArtifactSizeBytes(out2.Size),
		domain.WithArtifactCreatedAt(out2.CreatedAt),
	)
	require.NoError(t, repo.SaveArtifact(env.Ctx, artifact1))
	require.NoError(t, repo.SaveArtifact(env.Ctx, artifact2))

	got, err := repo.GetMessage(env.Ctx, msgID)
	require.NoError(t, err)
	require.Len(t, got.CodeOutputs(), 2)

	// Code outputs are ordered by created_at ASC.
	assert.Equal(t, out1.ID, got.CodeOutputs()[0].ID)
	assert.Equal(t, out2.ID, got.CodeOutputs()[1].ID)
	assert.Equal(t, "chart.png", got.CodeOutputs()[0].Name)
	assert.Equal(t, "/files/data.csv", got.CodeOutputs()[1].URL)

	msgs, err := repo.GetSessionMessages(env.Ctx, session.ID(), domain.ListOptions{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].CodeOutputs(), 2)
	assert.Equal(t, out1.ID, msgs[0].CodeOutputs()[0].ID)
	assert.Equal(t, out2.ID, msgs[0].CodeOutputs()[1].ID)
}
