package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactReaderRepoStub struct {
	artifactsByID map[uuid.UUID]domain.Artifact
	bySession     map[uuid.UUID][]domain.Artifact
}

func (s *artifactReaderRepoStub) CreateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (s *artifactReaderRepoStub) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return nil, errors.New("not found")
}

func (s *artifactReaderRepoStub) UpdateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (s *artifactReaderRepoStub) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return nil, nil
}

func (s *artifactReaderRepoStub) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *artifactReaderRepoStub) SaveMessage(ctx context.Context, msg types.Message) error {
	return nil
}

func (s *artifactReaderRepoStub) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	return nil, errors.New("not found")
}

func (s *artifactReaderRepoStub) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return nil, nil
}

func (s *artifactReaderRepoStub) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (s *artifactReaderRepoStub) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	return nil
}

func (s *artifactReaderRepoStub) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return nil, errors.New("not found")
}

func (s *artifactReaderRepoStub) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	return nil, nil
}

func (s *artifactReaderRepoStub) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *artifactReaderRepoStub) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	return nil
}

func (s *artifactReaderRepoStub) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	artifact, ok := s.artifactsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return artifact, nil
}

func (s *artifactReaderRepoStub) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	artifacts := s.bySession[sessionID]
	if opts.Offset >= len(artifacts) {
		return []domain.Artifact{}, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = len(artifacts)
	}
	end := opts.Offset + limit
	if end > len(artifacts) {
		end = len(artifacts)
	}
	return append([]domain.Artifact(nil), artifacts[opts.Offset:end]...), nil
}

func (s *artifactReaderRepoStub) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *artifactReaderRepoStub) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *artifactReaderRepoStub) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func (s *artifactReaderRepoStub) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	return nil
}

var errNoPendingQuestion = errors.New("no pending question")

func (s *artifactReaderRepoStub) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	return nil, errNoPendingQuestion
}

type artifactReaderStorageStub struct {
	contents map[string][]byte
}

func (s *artifactReaderStorageStub) Save(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error) {
	return "", nil
}

func (s *artifactReaderStorageStub) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	content, ok := s.contents[url]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *artifactReaderStorageStub) Delete(ctx context.Context, url string) error {
	return nil
}

func (s *artifactReaderStorageStub) Exists(ctx context.Context, url string) (bool, error) {
	_, ok := s.contents[url]
	return ok, nil
}

func TestArtifactReaderTool_ListAndPagination(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	now := time.Now().UTC()
	chartArtifact := domain.NewArtifact(
		domain.WithArtifactID(uuid.New()),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Revenue Chart"),
		domain.WithArtifactCreatedAt(now),
	)
	fileArtifact := domain.NewArtifact(
		domain.WithArtifactID(uuid.New()),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactType(domain.ArtifactTypeAttachment),
		domain.WithArtifactName("report.csv"),
		domain.WithArtifactCreatedAt(now.Add(-time.Minute)),
	)
	artifacts := []domain.Artifact{chartArtifact}
	for i := 0; i < 9; i++ {
		artifacts = append(artifacts, domain.NewArtifact(
			domain.WithArtifactID(uuid.New()),
			domain.WithArtifactSessionID(sessionID),
			domain.WithArtifactType(domain.ArtifactTypeAttachment),
			domain.WithArtifactName(fmt.Sprintf("filler_%d.txt", i)),
			domain.WithArtifactCreatedAt(now.Add(-time.Duration(i+2)*time.Minute)),
		))
	}
	artifacts = append(artifacts, fileArtifact)

	repo := &artifactReaderRepoStub{
		artifactsByID: map[uuid.UUID]domain.Artifact{
			chartArtifact.ID(): chartArtifact,
			fileArtifact.ID():  fileArtifact,
		},
		bySession: map[uuid.UUID][]domain.Artifact{
			sessionID: artifacts,
		},
	}

	tool := NewArtifactReaderTool(repo, &artifactReaderStorageStub{contents: map[string][]byte{}})
	ctx := agents.WithRuntimeSessionID(context.Background(), sessionID)

	output, err := tool.Call(ctx, `{"action":"list","page":1,"page_size":10}`)
	require.NoError(t, err)
	assert.Contains(t, output, "## Artifacts (page 1/2)")
	assert.Contains(t, output, chartArtifact.ID().String())
	assert.Contains(t, output, "has_next_page: true")

	outputPageTwo, err := tool.Call(ctx, `{"action":"list","page":2,"page_size":10}`)
	require.NoError(t, err)
	assert.Contains(t, outputPageTwo, "## Artifacts (page 2/2)")
	assert.Contains(t, outputPageTwo, fileArtifact.ID().String())
	assert.Contains(t, outputPageTwo, "has_next_page: false")
}

func TestArtifactReaderTool_ReadChartModes(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	chartArtifact := domain.NewArtifact(
		domain.WithArtifactID(uuid.New()),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName("Revenue Chart"),
		domain.WithArtifactMetadata(map[string]any{
			"spec": map[string]any{
				"chartType": "bar",
				"title":     "Revenue",
			},
		}),
	)

	repo := &artifactReaderRepoStub{
		artifactsByID: map[uuid.UUID]domain.Artifact{
			chartArtifact.ID(): chartArtifact,
		},
		bySession: map[uuid.UUID][]domain.Artifact{
			sessionID: {chartArtifact},
		},
	}

	tool := NewArtifactReaderTool(repo, &artifactReaderStorageStub{contents: map[string][]byte{}})
	ctx := agents.WithRuntimeSessionID(context.Background(), sessionID)

	specOutput, err := tool.Call(ctx, fmt.Sprintf(`{"action":"read","artifact_id":"%s","mode":"spec"}`, chartArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, specOutput, "## Chart Spec")
	assert.Contains(t, specOutput, `"chartType": "bar"`)

	visualOutput, err := tool.Call(ctx, fmt.Sprintf(`{"action":"read","artifact_id":"%s","mode":"visual"}`, chartArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, visualOutput, `Chart visual mode is not implemented yet. Use mode="spec".`)
}

func TestArtifactReaderTool_ReadTextWithPaginationAndSessionIsolation(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	otherSessionID := uuid.New()
	artifactURL := "https://files.local/report.txt"

	lines := make([]string, 0, 12)
	for i := 1; i <= 12; i++ {
		lines = append(lines, fmt.Sprintf("line_%02d", i))
	}

	textArtifact := domain.NewArtifact(
		domain.WithArtifactID(uuid.New()),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactType(domain.ArtifactTypeAttachment),
		domain.WithArtifactName("report.txt"),
		domain.WithArtifactMimeType("text/plain"),
		domain.WithArtifactURL(artifactURL),
		domain.WithArtifactSizeBytes(int64(len(strings.Join(lines, "\n")))),
	)

	repo := &artifactReaderRepoStub{
		artifactsByID: map[uuid.UUID]domain.Artifact{
			textArtifact.ID(): textArtifact,
		},
		bySession: map[uuid.UUID][]domain.Artifact{
			sessionID: {textArtifact},
		},
	}
	storageStub := &artifactReaderStorageStub{
		contents: map[string][]byte{
			artifactURL: []byte(strings.Join(lines, "\n")),
		},
	}

	tool := NewArtifactReaderTool(repo, storageStub)
	ctx := agents.WithRuntimeSessionID(context.Background(), sessionID)

	pageOne, err := tool.Call(ctx, fmt.Sprintf(`{"action":"read","artifact_id":"%s","page":1,"page_size":10}`, textArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, pageOne, "- page: 1/2")
	assert.Contains(t, pageOne, "line_01")
	assert.Contains(t, pageOne, "line_10")
	assert.Contains(t, pageOne, "has_next_page: true")

	pageTwo, err := tool.Call(ctx, fmt.Sprintf(`{"action":"read","artifact_id":"%s","page":2,"page_size":10}`, textArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, pageTwo, "- page: 2/2")
	assert.Contains(t, pageTwo, "line_11")
	assert.Contains(t, pageTwo, "line_12")
	assert.Contains(t, pageTwo, "has_next_page: false")

	outOfRange, err := tool.Call(ctx, fmt.Sprintf(`{"action":"read","artifact_id":"%s","page":3,"page_size":10}`, textArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, outOfRange, "Requested page is out of range")

	otherCtx := agents.WithRuntimeSessionID(context.Background(), otherSessionID)
	denied, err := tool.Call(otherCtx, fmt.Sprintf(`{"action":"read","artifact_id":"%s"}`, textArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, denied, "Access denied")

	missingSessionCtxOutput, err := tool.Call(context.Background(), fmt.Sprintf(`{"action":"read","artifact_id":"%s"}`, textArtifact.ID()))
	require.NoError(t, err)
	assert.Contains(t, missingSessionCtxOutput, "Session context is unavailable")
}
