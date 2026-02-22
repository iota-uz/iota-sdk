package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ArtifactHandler creates artifacts from ToolCompleteEvent and persists them.
//
// CRITICAL: This handler MUST be subscribed to the EventBus for artifacts to be created.
// The handler should be wired during application initialization:
//
//	handler := handlers.NewArtifactHandler(chatRepo)
//	eventBus.Subscribe(handler, hooks.EventToolComplete)
//
// Without this subscription, tool outputs will NOT be persisted as artifacts.
type ArtifactHandler struct {
	repo domain.ChatRepository
}

// NewArtifactHandler creates a new ArtifactHandler.
// Remember to subscribe this handler to the EventBus for it to receive events.
func NewArtifactHandler(repo domain.ChatRepository) *ArtifactHandler {
	return &ArtifactHandler{repo: repo}
}

// Handle implements hooks.EventHandler.
func (h *ArtifactHandler) Handle(ctx context.Context, event hooks.Event) error {
	toolEvent, ok := event.(*events.ToolCompleteEvent)
	if !ok {
		return nil
	}

	ctx = composables.WithTenantID(ctx, toolEvent.TenantID())

	switch toolEvent.ToolName {
	case "code_interpreter":
		return h.handleCodeInterpreter(ctx, toolEvent)
	case "draw_chart":
		return h.handleChart(ctx, toolEvent)
	case "export_query_to_excel", "export_data_to_excel", "export_to_pdf":
		return h.handleExport(ctx, toolEvent)
	}
	return nil
}

type codeInterpreterResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Outputs []struct {
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		URL      string `json:"url"`
		Size     int64  `json:"size"`
	} `json:"outputs"`
}

func (h *ArtifactHandler) handleCodeInterpreter(ctx context.Context, e *events.ToolCompleteEvent) error {
	const op serrors.Op = "ArtifactHandler.handleCodeInterpreter"

	var result codeInterpreterResult
	if err := json.Unmarshal([]byte(e.Result), &result); err != nil {
		return serrors.E(op, err, "failed to parse code_interpreter result")
	}
	if len(result.Outputs) == 0 {
		return nil
	}

	messageID, hasMessageID := bichatservices.UseArtifactMessageID(ctx)

	for _, out := range result.Outputs {
		opts := []domain.ArtifactOption{
			domain.WithArtifactTenantID(e.TenantID()),
			domain.WithArtifactSessionID(e.SessionID()),
			domain.WithArtifactType(domain.ArtifactTypeCodeOutput),
			domain.WithArtifactName(out.Name),
			domain.WithArtifactMimeType(out.MimeType),
			domain.WithArtifactURL(out.URL),
			domain.WithArtifactSizeBytes(out.Size),
		}
		if hasMessageID {
			opts = append(opts, domain.WithArtifactMessageID(messageID))
		}

		a := domain.NewArtifact(opts...)
		if err := h.repo.SaveArtifact(ctx, a); err != nil {
			return serrors.E(op, err, "failed to save code_output artifact")
		}
	}
	return nil
}

func (h *ArtifactHandler) handleChart(ctx context.Context, e *events.ToolCompleteEvent) error {
	const op serrors.Op = "ArtifactHandler.handleChart"

	spec, metadata, artifactName, err := parseChartArtifactPayload(e)
	if err != nil {
		return serrors.E(op, err, "failed to parse draw_chart result")
	}
	title := resolveChartTitle(spec, artifactName)

	opts := []domain.ArtifactOption{
		domain.WithArtifactTenantID(e.TenantID()),
		domain.WithArtifactSessionID(e.SessionID()),
		domain.WithArtifactType(domain.ArtifactTypeChart),
		domain.WithArtifactName(title),
		domain.WithArtifactMetadata(metadata),
	}

	if messageID, ok := bichatservices.UseArtifactMessageID(ctx); ok {
		opts = append(opts, domain.WithArtifactMessageID(messageID))
	}

	a := domain.NewArtifact(opts...)
	if err := h.repo.SaveArtifact(ctx, a); err != nil {
		return serrors.E(op, err, "failed to save chart artifact")
	}
	return nil
}

func parseChartArtifactPayload(e *events.ToolCompleteEvent) (map[string]any, map[string]any, string, error) {
	metadata := map[string]any{}
	artifactName := ""
	var spec map[string]any

	for _, artifact := range e.Artifacts {
		if artifact.Type != "chart" {
			continue
		}
		artifactName = strings.TrimSpace(artifact.Name)
		for k, v := range artifact.Metadata {
			metadata[k] = v
		}
		if rawSpec, ok := artifact.Metadata["spec"]; ok {
			if parsedSpec, ok := rawSpec.(map[string]any); ok {
				spec = parsedSpec
			}
		}
		break
	}

	if spec == nil {
		if err := json.Unmarshal([]byte(e.Result), &spec); err != nil {
			return nil, nil, "", err
		}
	}

	if _, ok := metadata["spec"]; !ok {
		metadata["spec"] = spec
	}

	return spec, metadata, artifactName, nil
}

func resolveChartTitle(spec map[string]any, preferred string) string {
	if preferred != "" {
		return preferred
	}

	title, _ := spec["title"].(string)
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}

	if titleObj, ok := spec["title"].(map[string]any); ok {
		if text, ok := titleObj["text"].(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return "Chart"
}

type exportResult struct {
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	RowCount    int    `json:"row_count"`
	Description string `json:"description,omitempty"`
	Size        int64  `json:"size,omitempty"`
	FileSizeKB  int64  `json:"file_size_kb,omitempty"`
}

func (h *ArtifactHandler) handleExport(ctx context.Context, e *events.ToolCompleteEvent) error {
	const op serrors.Op = "ArtifactHandler.handleExport"

	var result exportResult
	if err := json.Unmarshal([]byte(e.Result), &result); err != nil {
		return serrors.E(op, err, "failed to parse export result")
	}

	name := result.Filename
	if name == "" {
		name = "export.xlsx"
	}

	metadata := map[string]any{}
	if result.RowCount > 0 {
		metadata["row_count"] = result.RowCount
	}
	if result.Description != "" {
		metadata["description"] = result.Description
	}

	sizeBytes := result.Size
	if sizeBytes == 0 && result.FileSizeKB > 0 {
		sizeBytes = result.FileSizeKB * 1024
	}

	opts := []domain.ArtifactOption{
		domain.WithArtifactTenantID(e.TenantID()),
		domain.WithArtifactSessionID(e.SessionID()),
		domain.WithArtifactType(domain.ArtifactTypeExport),
		domain.WithArtifactName(name),
		domain.WithArtifactDescription(result.Description),
		domain.WithArtifactURL(result.URL),
	}
	if len(metadata) > 0 {
		opts = append(opts, domain.WithArtifactMetadata(metadata))
	}
	if sizeBytes > 0 {
		opts = append(opts, domain.WithArtifactSizeBytes(sizeBytes))
	}

	lowerName := strings.ToLower(name)
	if strings.HasSuffix(lowerName, ".pdf") {
		opts = append(opts, domain.WithArtifactMimeType("application/pdf"))
	} else if strings.HasSuffix(lowerName, ".xlsx") || strings.HasSuffix(lowerName, ".xls") {
		opts = append(opts, domain.WithArtifactMimeType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
	}
	if messageID, ok := bichatservices.UseArtifactMessageID(ctx); ok {
		opts = append(opts, domain.WithArtifactMessageID(messageID))
	}

	a := domain.NewArtifact(opts...)
	if err := h.repo.SaveArtifact(ctx, a); err != nil {
		return serrors.E(op, err, "failed to save export artifact")
	}
	return nil
}
