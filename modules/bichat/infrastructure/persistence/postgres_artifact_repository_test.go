package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresChatRepository_SaveAndGetArtifact(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Artifact Session"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	createdAt := time.Now().Add(-1 * time.Minute)
	artifact := domain.NewArtifact(
		domain.WithArtifactID(uuid.New()),
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Sales Chart"),
		domain.WithArtifactDescription("Monthly sales"),
		domain.WithArtifactMimeType("application/json"),
		domain.WithArtifactURL("/files/sales-chart.json"),
		domain.WithArtifactSizeBytes(42),
		domain.WithArtifactMetadata(map[string]any{"spec": "vega-lite"}),
		domain.WithArtifactCreatedAt(createdAt),
	)
	require.NoError(t, repo.SaveArtifact(env.Ctx, artifact))

	got, err := repo.GetArtifact(env.Ctx, artifact.ID())
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, artifact.ID(), got.ID())
	assert.Equal(t, env.Tenant.ID, got.TenantID())
	assert.Equal(t, session.ID(), got.SessionID())
	assert.Equal(t, domain.ArtifactTypeChart, got.Type())
	assert.Equal(t, "Sales Chart", got.Name())
	assert.Equal(t, "Monthly sales", got.Description())
	assert.Equal(t, "application/json", got.MimeType())
	assert.Equal(t, "/files/sales-chart.json", got.URL())
	assert.Equal(t, int64(42), got.SizeBytes())
	assert.Equal(t, "vega-lite", got.GetMetadataString("spec"))
	assert.WithinDuration(t, createdAt, got.CreatedAt(), time.Second)
}

func TestPostgresChatRepository_GetArtifact_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	_, err := repo.GetArtifact(env.Ctx, uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrArtifactNotFound)
}

func TestPostgresChatRepository_GetSessionArtifacts_OrderAndTypeFilter(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Artifacts Listing"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	base := time.Now().Add(-1 * time.Hour)
	a1 := domain.NewArtifact(
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Chart A"),
		domain.WithArtifactCreatedAt(base),
	)
	a2 := domain.NewArtifact(
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeExport),
		domain.WithArtifactName("Export B"),
		domain.WithArtifactCreatedAt(base.Add(1*time.Minute)),
	)
	a3 := domain.NewArtifact(
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Chart C"),
		domain.WithArtifactCreatedAt(base.Add(2*time.Minute)),
	)
	require.NoError(t, repo.SaveArtifact(env.Ctx, a1))
	require.NoError(t, repo.SaveArtifact(env.Ctx, a2))
	require.NoError(t, repo.SaveArtifact(env.Ctx, a3))

	all, err := repo.GetSessionArtifacts(env.Ctx, session.ID(), domain.ListOptions{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, all, 3)

	assert.Equal(t, a3.ID(), all[0].ID(), "expected DESC ordering by created_at")
	assert.Equal(t, a2.ID(), all[1].ID(), "expected DESC ordering by created_at")
	assert.Equal(t, a1.ID(), all[2].ID(), "expected DESC ordering by created_at")

	charts, err := repo.GetSessionArtifacts(env.Ctx, session.ID(), domain.ListOptions{
		Limit:  10,
		Offset: 0,
		Types:  []domain.ArtifactType{domain.ArtifactTypeChart},
	})
	require.NoError(t, err)
	require.Len(t, charts, 2)
	assert.Equal(t, domain.ArtifactTypeChart, charts[0].Type())
	assert.Equal(t, domain.ArtifactTypeChart, charts[1].Type())
}

func TestPostgresChatRepository_UpdateAndDeleteArtifact(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Artifact Updates"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	artifact := domain.NewArtifact(
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeExport),
		domain.WithArtifactName("Old Name"),
		domain.WithArtifactDescription("Old Description"),
	)
	require.NoError(t, repo.SaveArtifact(env.Ctx, artifact))

	require.NoError(t, repo.UpdateArtifact(env.Ctx, artifact.ID(), "New Name", "New Description"))

	got, err := repo.GetArtifact(env.Ctx, artifact.ID())
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name())
	assert.Equal(t, "New Description", got.Description())

	require.NoError(t, repo.DeleteArtifact(env.Ctx, artifact.ID()))
	_, err = repo.GetArtifact(env.Ctx, artifact.ID())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrArtifactNotFound)
}

func TestPostgresChatRepository_TenantIsolation_Artifacts(t *testing.T) {
	t.Parallel()
	envA := setupTest(t)
	envB := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	sessionA := domain.NewSession(
		domain.WithTenantID(envA.Tenant.ID),
		domain.WithUserID(int64(envA.User.ID())),
		domain.WithTitle("Tenant A Artifacts"),
	)
	require.NoError(t, repo.CreateSession(envA.Ctx, sessionA))

	artifactA := domain.NewArtifact(
		domain.WithArtifactTenantID(envA.Tenant.ID),
		domain.WithArtifactSessionID(sessionA.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Tenant A Chart"),
	)
	require.NoError(t, repo.SaveArtifact(envA.Ctx, artifactA))

	// Different tenant context should not see the artifact.
	_, err := repo.GetArtifact(envB.Ctx, artifactA.ID())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrArtifactNotFound)
}

func TestPostgresChatRepository_DeleteSessionArtifacts(t *testing.T) {
	t.Parallel()
	envA := setupTest(t)
	envB := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	sessionA := domain.NewSession(
		domain.WithTenantID(envA.Tenant.ID),
		domain.WithUserID(int64(envA.User.ID())),
		domain.WithTitle("Tenant A Session"),
	)
	sessionB := domain.NewSession(
		domain.WithTenantID(envB.Tenant.ID),
		domain.WithUserID(int64(envB.User.ID())),
		domain.WithTitle("Tenant B Session"),
	)
	require.NoError(t, repo.CreateSession(envA.Ctx, sessionA))
	require.NoError(t, repo.CreateSession(envB.Ctx, sessionB))

	a1 := domain.NewArtifact(
		domain.WithArtifactTenantID(envA.Tenant.ID),
		domain.WithArtifactSessionID(sessionA.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("A1"),
	)
	a2 := domain.NewArtifact(
		domain.WithArtifactTenantID(envA.Tenant.ID),
		domain.WithArtifactSessionID(sessionA.ID()),
		domain.WithArtifactType(domain.ArtifactTypeExport),
		domain.WithArtifactName("A2"),
	)
	b1 := domain.NewArtifact(
		domain.WithArtifactTenantID(envB.Tenant.ID),
		domain.WithArtifactSessionID(sessionB.ID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("B1"),
	)
	require.NoError(t, repo.SaveArtifact(envA.Ctx, a1))
	require.NoError(t, repo.SaveArtifact(envA.Ctx, a2))
	require.NoError(t, repo.SaveArtifact(envB.Ctx, b1))

	deleted, err := repo.DeleteSessionArtifacts(envA.Ctx, sessionA.ID())
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	artifactsA, err := repo.GetSessionArtifacts(envA.Ctx, sessionA.ID(), domain.ListOptions{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, artifactsA)

	artifactsB, err := repo.GetSessionArtifacts(envB.Ctx, sessionB.ID(), domain.ListOptions{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, artifactsB, 1)
	assert.Equal(t, b1.ID(), artifactsB[0].ID())
}

func TestPostgresChatRepository_UpsertAndGetArtifactProviderFile(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo, ok := persistence.NewPostgresChatRepository().(*persistence.PostgresChatRepository)
	require.True(t, ok)

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Provider File Sync"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	artifact := domain.NewArtifact(
		domain.WithArtifactTenantID(env.Tenant.ID),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactType(domain.ArtifactTypeAttachment),
		domain.WithArtifactName("sales.csv"),
		domain.WithArtifactURL("https://example.com/uploads/sales.csv"),
		domain.WithArtifactSizeBytes(2048),
	)
	require.NoError(t, repo.SaveArtifact(env.Ctx, artifact))

	require.NoError(t, repo.UpsertArtifactProviderFile(
		env.Ctx,
		artifact.ID(),
		"openai",
		"file_abc123",
		artifact.URL(),
		artifact.SizeBytes(),
	))

	fileID, sourceURL, sourceSize, err := repo.GetArtifactProviderFile(env.Ctx, artifact.ID(), "openai")
	require.NoError(t, err)
	assert.Equal(t, "file_abc123", fileID)
	assert.Equal(t, artifact.URL(), sourceURL)
	assert.Equal(t, artifact.SizeBytes(), sourceSize)

	// Upsert should update existing mapping.
	require.NoError(t, repo.UpsertArtifactProviderFile(
		env.Ctx,
		artifact.ID(),
		"openai",
		"file_new999",
		artifact.URL(),
		artifact.SizeBytes(),
	))
	fileID, sourceURL, sourceSize, err = repo.GetArtifactProviderFile(env.Ctx, artifact.ID(), "openai")
	require.NoError(t, err)
	assert.Equal(t, "file_new999", fileID)
	assert.Equal(t, artifact.URL(), sourceURL)
	assert.Equal(t, artifact.SizeBytes(), sourceSize)
}

func TestPostgresChatRepository_GetArtifactProviderFile_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)
	repo, ok := persistence.NewPostgresChatRepository().(*persistence.PostgresChatRepository)
	require.True(t, ok)

	fileID, sourceURL, sourceSize, err := repo.GetArtifactProviderFile(env.Ctx, uuid.New(), "openai")
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrArtifactProviderFileNotFound)
	assert.Empty(t, fileID)
	assert.Empty(t, sourceURL)
	assert.Zero(t, sourceSize)
}
