package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

func NewArtifactReaderTool(repo domain.ChatRepository, fileStorage storage.FileStorage, opts ...ArtifactReaderOption) *ArtifactReaderTool {
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

// CallStructured executes the artifact reader operation and returns a structured result.
func (t *ArtifactReaderTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[artifactReaderInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "failed to parse input: " + err.Error(),
				Hints:   []string{HintCheckRequiredFields},
			},
		}, agents.ErrStructuredToolOutput
	}

	if t.repo == nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "Repository is not configured.",
				Hints:   []string{HintCheckConnection},
			},
		}, nil
	}

	sessionID, ok := agents.UseRuntimeSessionID(ctx)
	if !ok {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "Session context is unavailable for artifact lookup.",
				Hints:   []string{"Ensure the agent is running in a session context"},
			},
		}, nil
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
		return t.listArtifactsStructured(ctx, sessionID, page, pageSize)
	case "read":
		return t.readArtifactStructured(ctx, sessionID, strings.TrimSpace(params.ArtifactID), page, pageSize, mode)
	default:
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "Invalid action. Use action=\"list\" or action=\"read\".",
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}
}

// Call executes the artifact reader operation (delegates to CallStructured).
func (t *ArtifactReaderTool) Call(ctx context.Context, input string) (string, error) {
	return FormatStructuredResult(t.CallStructured(ctx, input))
}

func (t *ArtifactReaderTool) listArtifactsStructured(ctx context.Context, sessionID uuid.UUID, page, pageSize int) (*types.ToolResult, error) {
	artifacts, err := t.repo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  artifactReaderMaxArtifacts,
		Offset: 0,
	})
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeQueryError),
				Message: fmt.Sprintf("Failed to list artifacts: %v", err),
				Hints:   []string{HintRetryLater},
			},
		}, err
	}

	total := len(artifacts)
	pages := 1
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}

	if page > pages && pages > 0 {
		return &types.ToolResult{
			CodecID: types.CodecArtifactList,
			Payload: types.ArtifactListPayload{
				Page:       page,
				TotalPages: pages,
				Artifacts:  []types.ArtifactEntry{},
				HasNext:    false,
				HitCap:     total >= artifactReaderMaxArtifacts,
			},
		}, nil
	}

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	entries := make([]types.ArtifactEntry, 0, end-start)
	for _, artifact := range artifacts[start:end] {
		entries = append(entries, types.ArtifactEntry{
			ID:        artifact.ID().String(),
			Type:      string(artifact.Type()),
			Name:      artifact.Name(),
			MimeType:  artifact.MimeType(),
			SizeBytes: artifact.SizeBytes(),
			CreatedAt: artifact.CreatedAt().UTC().Format(time.RFC3339),
		})
	}

	hasNext := page < pages

	return &types.ToolResult{
		CodecID: types.CodecArtifactList,
		Payload: types.ArtifactListPayload{
			Page:       page,
			TotalPages: pages,
			Artifacts:  entries,
			HasNext:    hasNext,
			HitCap:     total >= artifactReaderMaxArtifacts,
		},
	}, nil
}

func (t *ArtifactReaderTool) readArtifactStructured(ctx context.Context, sessionID uuid.UUID, artifactID string, page, pageSize int, mode string) (*types.ToolResult, error) {
	if artifactID == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "action=\"read\" requires artifact_id.",
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}

	id, err := uuid.Parse(artifactID)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "artifact_id must be a valid UUID.",
				Hints:   []string{HintCheckFieldFormat},
			},
		}, nil
	}

	artifact, err := t.repo.GetArtifact(ctx, id)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeQueryError),
				Message: fmt.Sprintf("Failed to read artifact: %v", err),
				Hints:   []string{HintRetryLater},
			},
		}, err
	}
	if artifact.SessionID() != sessionID {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodePermissionDenied),
				Message: "Access denied: artifact is not in the current session.",
				Hints:   []string{"Verify the artifact_id belongs to the current session"},
			},
		}, nil
	}

	if artifact.Type() == domain.ArtifactTypeChart {
		return t.readChartArtifactStructured(artifact, mode)
	}

	if t.fileStorage == nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "File storage is not configured.",
				Hints:   []string{HintCheckConnection},
			},
		}, nil
	}
	if strings.TrimSpace(artifact.URL()) == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeNoData),
				Message: "Artifact has no file URL to read.",
				Hints:   []string{"This artifact may not have associated file content"},
			},
		}, nil
	}

	rc, err := t.fileStorage.Get(ctx, artifact.URL())
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("Failed to open artifact content: %v", err),
				Hints:   []string{HintCheckConnection, HintRetryLater},
			},
		}, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("Failed to read artifact content: %v", err),
				Hints:   []string{HintRetryLater},
			},
		}, err
	}

	content, err := t.extractArtifactContent(ctx, artifact, data)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("Could not extract content: %v", err),
				Hints:   []string{"File format may not be supported or file may be corrupted"},
			},
		}, nil
	}

	lines := normalizeToLines(content)
	window := paginateLines(lines, page, pageSize)

	pageContent := ""
	if !window.OutOfRange && len(window.Lines) > 0 {
		pageContent = strings.Join(window.Lines, "\n")
	}

	return &types.ToolResult{
		CodecID: types.CodecArtifactContent,
		Payload: types.ArtifactContentPayload{
			ID:         artifact.ID().String(),
			Type:       string(artifact.Type()),
			Name:       artifact.Name(),
			MimeType:   artifact.MimeType(),
			Page:       window.Page,
			TotalPages: window.TotalPages,
			PageSize:   window.PageSize,
			Content:    pageContent,
			HasNext:    window.HasNext,
			OutOfRange: window.OutOfRange,
		},
	}, nil
}

func (t *ArtifactReaderTool) readChartArtifactStructured(artifact domain.Artifact, mode string) (*types.ToolResult, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "default"
	}

	if mode == "visual" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "Chart visual mode is not implemented yet. Use mode=\"spec\".",
				Hints:   []string{"Use mode=\"spec\" or mode=\"default\" to view chart specification"},
			},
		}, nil
	}

	spec, ok := artifact.Metadata()["spec"]
	if !ok {
		spec = map[string]any{}
	}

	specJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("Failed to render chart spec: %v", err),
				Hints:   []string{"Chart specification may be malformed"},
			},
		}, nil
	}

	content := "```json\n" + string(specJSON) + "\n```"

	return &types.ToolResult{
		CodecID: types.CodecArtifactContent,
		Payload: types.ArtifactContentPayload{
			ID:         artifact.ID().String(),
			Type:       string(artifact.Type()),
			Name:       artifact.Name(),
			MimeType:   artifact.MimeType(),
			Page:       1,
			TotalPages: 1,
			PageSize:   0,
			Content:    content,
			HasNext:    false,
			OutOfRange: false,
		},
	}, nil
}
