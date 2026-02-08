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
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
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
				"description": "UUID, required for read when artifact_name is not provided",
			},
			"artifact_name": map[string]any{
				"type":        "string",
				"description": "Optional exact artifact name for read (supports renamed artifacts)",
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
	Action       string `json:"action"`
	ArtifactID   string `json:"artifact_id"`
	ArtifactName string `json:"artifact_name"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	Mode         string `json:"mode"`
}

// CallStructured executes the artifact reader operation and returns a structured result.
func (t *ArtifactReaderTool) CallStructured(ctx context.Context, input string) (*agents.ToolResult, error) {
	params, err := agents.ParseToolInput[artifactReaderInput](input)
	if err != nil {
		return &agents.ToolResult{ //nolint:nilerr // structured error payload for LLM
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "failed to parse input: " + err.Error(),
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}

	if t.repo == nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "Repository is not configured.",
				Hints:   []string{HintCheckConnection},
			},
		}, nil
	}

	sessionID, ok := agents.UseRuntimeSessionID(ctx)
	if !ok {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
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
		return t.readArtifactStructured(
			ctx,
			sessionID,
			strings.TrimSpace(params.ArtifactID),
			strings.TrimSpace(params.ArtifactName),
			page,
			pageSize,
			mode,
		)
	default:
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "Invalid action. Use action=\"list\" or action=\"read\".",
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}
}

// Call executes the artifact reader operation (delegates to CallStructured).
func (t *ArtifactReaderTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, bichatctx.DefaultFormatOptions())
				if fmtErr == nil {
					return formatted, err
				}
			}
		}
		return "", err
	}

	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, bichatctx.DefaultFormatOptions())
}

func (t *ArtifactReaderTool) listArtifactsStructured(ctx context.Context, sessionID uuid.UUID, page, pageSize int) (*agents.ToolResult, error) {
	artifacts, err := t.repo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  artifactReaderMaxArtifacts,
		Offset: 0,
	})
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
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
		return &agents.ToolResult{
			CodecID: formatters.CodecArtifactList,
			Payload: formatters.ArtifactListPayload{
				Page:       page,
				TotalPages: pages,
				Artifacts:  []formatters.ArtifactEntry{},
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

	entries := make([]formatters.ArtifactEntry, 0, end-start)
	for _, artifact := range artifacts[start:end] {
		entries = append(entries, formatters.ArtifactEntry{
			ID:        artifact.ID().String(),
			Type:      string(artifact.Type()),
			Name:      artifact.Name(),
			MimeType:  artifact.MimeType(),
			SizeBytes: artifact.SizeBytes(),
			CreatedAt: artifact.CreatedAt().UTC().Format(time.RFC3339),
		})
	}

	hasNext := page < pages

	return &agents.ToolResult{
		CodecID: formatters.CodecArtifactList,
		Payload: formatters.ArtifactListPayload{
			Page:       page,
			TotalPages: pages,
			Artifacts:  entries,
			HasNext:    hasNext,
			HitCap:     total >= artifactReaderMaxArtifacts,
		},
	}, nil
}

func (t *ArtifactReaderTool) readArtifactStructured(
	ctx context.Context,
	sessionID uuid.UUID,
	artifactID string,
	artifactName string,
	page,
	pageSize int,
	mode string,
) (*agents.ToolResult, error) {
	artifact, err := t.resolveArtifact(ctx, sessionID, artifactID, artifactName)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{HintCheckRequiredFields, "Use action=\"list\" to discover available artifact names and IDs"},
			},
		}, nil
	}

	if artifact.Type() == domain.ArtifactTypeChart {
		return t.readChartArtifactStructured(artifact, mode)
	}

	if t.fileStorage == nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "File storage is not configured.",
				Hints:   []string{HintCheckConnection},
			},
		}, nil
	}
	if strings.TrimSpace(artifact.URL()) == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeNoData),
				Message: "Artifact has no file URL to read.",
				Hints:   []string{"This artifact may not have associated file content"},
			},
		}, nil
	}

	rc, err := t.fileStorage.Get(ctx, artifact.URL())
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("Failed to open artifact content: %v", err),
				Hints:   []string{HintCheckConnection, HintRetryLater},
			},
		}, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("Failed to read artifact content: %v", err),
				Hints:   []string{HintRetryLater},
			},
		}, err
	}

	content, err := t.extractArtifactContent(ctx, artifact, data)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
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

	return &agents.ToolResult{
		CodecID: formatters.CodecArtifactContent,
		Payload: formatters.ArtifactContentPayload{
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

func (t *ArtifactReaderTool) resolveArtifact(
	ctx context.Context,
	sessionID uuid.UUID,
	artifactID string,
	artifactName string,
) (domain.Artifact, error) {
	if artifactID != "" {
		id, err := uuid.Parse(artifactID)
		if err != nil {
			return nil, fmt.Errorf("artifact_id must be a valid UUID")
		}

		artifact, err := t.repo.GetArtifact(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to read artifact: %v", err)
		}
		if artifact.SessionID() != sessionID {
			return nil, fmt.Errorf("access denied: artifact is not in the current session")
		}
		return artifact, nil
	}

	if artifactName == "" {
		return nil, fmt.Errorf("action=\"read\" requires artifact_id or artifact_name")
	}

	artifacts, err := t.repo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  artifactReaderMaxArtifacts,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %v", err)
	}

	var matches []domain.Artifact
	for _, artifact := range artifacts {
		if strings.EqualFold(strings.TrimSpace(artifact.Name()), artifactName) {
			matches = append(matches, artifact)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("artifact with name %q was not found in this session", artifactName)
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, artifact := range matches {
			ids = append(ids, artifact.ID().String())
		}
		return nil, fmt.Errorf(
			"multiple artifacts share name %q; use artifact_id instead (matches: %s)",
			artifactName,
			strings.Join(ids, ", "),
		)
	}

	return matches[0], nil
}

func (t *ArtifactReaderTool) readChartArtifactStructured(artifact domain.Artifact, mode string) (*agents.ToolResult, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "default"
	}

	if mode == "visual" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
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
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("Failed to render chart spec: %v", err),
				Hints:   []string{"Chart specification may be malformed"},
			},
		}, nil
	}

	content := "```json\n" + string(specJSON) + "\n```"

	return &agents.ToolResult{
		CodecID: formatters.CodecArtifactContent,
		Payload: formatters.ArtifactContentPayload{
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
