package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactRepoStub struct {
	domain.ChatRepository
	saved         []domain.Artifact
	tenantIDsSeen []uuid.UUID
}

func (r *artifactRepoStub) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	r.saved = append(r.saved, artifact)
	r.tenantIDsSeen = append(r.tenantIDsSeen, tenantID)
	return nil
}

func TestArtifactHandler_SubscribedToolCompleteEvent_PersistsArtifact(t *testing.T) {
	repo := &artifactRepoStub{}
	bus := hooks.NewEventBus()
	handler := NewArtifactHandler(repo)
	bus.Subscribe(handler, string(hooks.EventToolComplete))

	tenantID := uuid.New()
	sessionID := uuid.New()
	userMessageID := uuid.New()
	ctx := bichatservices.WithArtifactMessageID(context.Background(), userMessageID)

	err := bus.Publish(ctx, events.NewToolCompleteEvent(
		sessionID,
		tenantID,
		"export_query_to_excel",
		`{"query":"SELECT * FROM users"}`,
		"call-1",
		`{"url":"https://example.com/report.xlsx","filename":"report.xlsx","row_count":42,"size":2048}`,
		187,
	))
	require.NoError(t, err)
	require.Len(t, repo.saved, 1)
	require.Len(t, repo.tenantIDsSeen, 1)

	artifact := repo.saved[0]
	assert.Equal(t, domain.ArtifactTypeExport, artifact.Type())
	assert.Equal(t, sessionID, artifact.SessionID())
	assert.Equal(t, tenantID, artifact.TenantID())
	assert.Equal(t, "report.xlsx", artifact.Name())
	assert.Equal(t, "https://example.com/report.xlsx", artifact.URL())
	assert.Equal(t, int64(2048), artifact.SizeBytes())
	assert.Equal(t, 42, artifact.GetMetadataInt("row_count"))
	require.NotNil(t, artifact.MessageID())
	assert.Equal(t, userMessageID, *artifact.MessageID())
	assert.Equal(t, tenantID, repo.tenantIDsSeen[0])
}
