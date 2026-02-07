package tools

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
)

const artifactReaderMaxArtifacts = 5000

// PDFFallbackReader is an optional OCR/LLM fallback for scanned PDFs.
type PDFFallbackReader interface {
	ExtractPDFText(ctx context.Context, filename string, content []byte) (string, error)
}

type ArtifactReaderTool struct {
	repo           domain.ChatRepository
	fileStorage    storage.FileStorage
	pdfFallback    PDFFallbackReader
	maxOutputChars int
}

type ArtifactReaderOption func(*ArtifactReaderTool)

func WithArtifactReaderMaxOutputChars(maxChars int) ArtifactReaderOption {
	return func(t *ArtifactReaderTool) {
		if maxChars > 0 {
			t.maxOutputChars = maxChars
		}
	}
}

func WithArtifactReaderPDFFallback(fallback PDFFallbackReader) ArtifactReaderOption {
	return func(t *ArtifactReaderTool) {
		t.pdfFallback = fallback
	}
}

func NewArtifactReaderTool(repo domain.ChatRepository, fileStorage storage.FileStorage, opts ...ArtifactReaderOption) agents.Tool {
	tool := &ArtifactReaderTool{
		repo:           repo,
		fileStorage:    fileStorage,
		maxOutputChars: artifactReaderMaxOutputSize,
	}
	for _, opt := range opts {
		opt(tool)
	}
	return tool
}

func (t *ArtifactReaderTool) Name() string {
	return "artifact_reader"
}

func (t *ArtifactReaderTool) Description() string {
	return "List and read artifacts attached to the current chat session, including charts and document attachments."
}

func (t *ArtifactReaderTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"list", "read"},
			},
			"artifact_id": map[string]any{
				"type":        "string",
				"description": "UUID, required for read",
			},
			"page": map[string]any{
				"type":    "integer",
				"minimum": 1,
				"default": artifactReaderDefaultPage,
			},
			"page_size": map[string]any{
				"type":    "integer",
				"minimum": artifactReaderMinPageSize,
				"maximum": artifactReaderMaxPageSize,
				"default": artifactReaderDefaultSize,
			},
			"mode": map[string]any{
				"type":    "string",
				"enum":    []string{"default", "spec", "visual"},
				"default": "default",
			},
		},
		"required": []string{"action"},
	}
}

type artifactReaderInput struct {
	Action     string `json:"action"`
	ArtifactID string `json:"artifact_id"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Mode       string `json:"mode"`
}

func (t *ArtifactReaderTool) Call(ctx context.Context, input string) (string, error) {
	params, err := agents.ParseToolInput[artifactReaderInput](input)
	if err != nil {
		return "## Artifact Reader\n\nInvalid input: unable to parse tool arguments.", nil
	}

	if t.repo == nil {
		return "## Artifact Reader\n\nRepository is not configured.", nil
	}

	sessionID, ok := agents.UseRuntimeSessionID(ctx)
	if !ok {
		return "## Artifact Reader\n\nSession context is unavailable for artifact lookup.", nil
	}

	action := strings.ToLower(strings.TrimSpace(params.Action))
	page := clampPage(params.Page)
	pageSize := clampPageSize(params.PageSize)
	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		mode = "default"
	}

	switch action {
	case "list":
		return t.listArtifacts(ctx, sessionID, page, pageSize), nil
	case "read":
		return t.readArtifact(ctx, sessionID, strings.TrimSpace(params.ArtifactID), page, pageSize, mode), nil
	default:
		return "## Artifact Reader\n\nInvalid action. Use action=\"list\" or action=\"read\".", nil
	}
}

func (t *ArtifactReaderTool) listArtifacts(ctx context.Context, sessionID uuid.UUID, page, pageSize int) string {
	artifacts, err := t.repo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  artifactReaderMaxArtifacts,
		Offset: 0,
	})
	if err != nil {
		return fmt.Sprintf("## Artifact Reader\n\nFailed to list artifacts: %v", err)
	}

	total := len(artifacts)
	pages := 1
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}

	if page > pages {
		return fmt.Sprintf("## Artifacts (page %d/%d)\n\nNo artifacts on this page.\n\nhas_next_page: false", page, pages)
	}

	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Artifacts (page %d/%d)\n", page, pages)
	b.WriteString("| id | type | name | mime | size_bytes | created_at |\n")
	b.WriteString("| --- | --- | --- | --- | ---: | --- |\n")

	for _, artifact := range artifacts[start:end] {
		fmt.Fprintf(&b,
			"| %s | %s | %s | %s | %d | %s |\n",
			artifact.ID().String(),
			string(artifact.Type()),
			escapeTableCell(artifact.Name()),
			escapeTableCell(artifact.MimeType()),
			artifact.SizeBytes(),
			artifact.CreatedAt().UTC().Format(time.RFC3339),
		)
	}

	hasNext := page < pages
	fmt.Fprintf(&b, "\nhas_next_page: %t", hasNext)
	if hasNext {
		fmt.Fprintf(&b, " (use page=%d)", page+1)
	}
	if total >= artifactReaderMaxArtifacts {
		b.WriteString("\n\nNote: artifact listing reached tool cap and may be truncated.")
	}

	return clampOutput(b.String(), t.maxOutputChars)
}

func (t *ArtifactReaderTool) readArtifact(ctx context.Context, sessionID uuid.UUID, artifactID string, page, pageSize int, mode string) string {
	if artifactID == "" {
		return "## Artifact Reader\n\naction=\"read\" requires artifact_id."
	}

	id, err := uuid.Parse(artifactID)
	if err != nil {
		return "## Artifact Reader\n\nartifact_id must be a valid UUID."
	}

	artifact, err := t.repo.GetArtifact(ctx, id)
	if err != nil {
		return fmt.Sprintf("## Artifact Reader\n\nFailed to read artifact: %v", err)
	}
	if artifact.SessionID() != sessionID {
		return "## Artifact Reader\n\nAccess denied: artifact is not in the current session."
	}

	if artifact.Type() == domain.ArtifactTypeChart {
		return clampOutput(renderChartArtifact(artifact, mode), t.maxOutputChars)
	}

	if t.fileStorage == nil {
		return "## Artifact Read\n\nFile storage is not configured."
	}
	if strings.TrimSpace(artifact.URL()) == "" {
		return "## Artifact Read\n\nArtifact has no file URL to read."
	}

	rc, err := t.fileStorage.Get(ctx, artifact.URL())
	if err != nil {
		return fmt.Sprintf("## Artifact Read\n\nFailed to open artifact content: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Sprintf("## Artifact Read\n\nFailed to read artifact content: %v", err)
	}

	content, err := t.extractArtifactContent(ctx, artifact, data)
	if err != nil {
		return fmt.Sprintf("## Artifact Read\n\nCould not extract content: %v", err)
	}

	lines := normalizeToLines(content)
	window := paginateLines(lines, page, pageSize)

	var b strings.Builder
	b.WriteString("## Artifact Read\n")
	fmt.Fprintf(&b, "- id: %s\n", artifact.ID())
	fmt.Fprintf(&b, "- type: %s\n", artifact.Type())
	fmt.Fprintf(&b, "- name: %s\n", artifact.Name())
	fmt.Fprintf(&b, "- mime: %s\n", artifact.MimeType())
	fmt.Fprintf(&b, "- page: %d/%d\n", window.Page, window.TotalPages)
	fmt.Fprintf(&b, "- page_size: %d\n\n", window.PageSize)

	if window.OutOfRange {
		b.WriteString("Requested page is out of range for this artifact content.\n")
	} else if len(window.Lines) == 0 {
		b.WriteString("(no content on this page)\n")
	} else {
		b.WriteString(strings.Join(window.Lines, "\n"))
		b.WriteString("\n")
	}

	if window.HasNext {
		fmt.Fprintf(&b, "\nhas_next_page: true (use page=%d)", window.Page+1)
	} else {
		b.WriteString("\nhas_next_page: false")
	}

	return clampOutput(b.String(), t.maxOutputChars)
}

func escapeTableCell(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ")
	return replacer.Replace(value)
}
