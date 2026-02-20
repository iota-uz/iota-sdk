package llmproviders

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// OpenAIModel implements the agents.Model interface using OpenAI's Responses API.
// It provides both blocking and streaming modes with native tool support
// including web_search and code_interpreter (handled by the API).
type OpenAIModel struct {
	mu                           sync.RWMutex
	client                       *openai.Client
	modelName                    string
	logger                       logging.Logger
	codeInterpreterMemoryLimit   string
	codeInterpreterArtifactLimit int
	chatRepo                     domain.ChatRepository
	fileStorage                  storage.FileStorage
}

// OpenAIModelOption configures an OpenAIModel.
type OpenAIModelOption func(*OpenAIModel)

const (
	defaultCodeInterpreterMemoryLimit = "4g"
	defaultCodeInterpreterFileLimit   = 20
	codeInterpreterProviderOpenAI     = "openai"
	maxOpenAIFileUploadBytes          = 512 << 20 // 512MB
)

type artifactProviderFileSyncRepository interface {
	GetArtifactProviderFile(ctx context.Context, artifactID uuid.UUID, provider string) (providerFileID, sourceURL string, sourceSizeBytes int64, err error)
	UpsertArtifactProviderFile(ctx context.Context, artifactID uuid.UUID, provider, providerFileID, sourceURL string, sourceSizeBytes int64) error
}

// WithLogger sets the logger for the OpenAI model.
func WithLogger(logger logging.Logger) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.logger = logger
	}
}

// WithCodeInterpreterMemoryLimit sets the default memory limit for OpenAI code_interpreter containers.
// Allowed values: "1g", "4g", "16g", "64g".
func WithCodeInterpreterMemoryLimit(limit string) OpenAIModelOption {
	return func(m *OpenAIModel) {
		if normalized, ok := normalizeCodeInterpreterMemoryLimit(limit); ok {
			m.mu.Lock()
			m.codeInterpreterMemoryLimit = normalized
			m.mu.Unlock()
			return
		}
		m.logger.Warn(context.Background(), "invalid code_interpreter memory limit option ignored", map[string]any{
			"provided_limit": limit,
			"allowed_values": []string{"1g", "4g", "16g", "64g"},
		})
	}
}

// WithCodeInterpreterArtifactSource configures artifact lookup and file storage so
// uploaded session artifacts can be passed to code_interpreter as file_ids.
func WithCodeInterpreterArtifactSource(repo domain.ChatRepository, fileStorage storage.FileStorage) OpenAIModelOption {
	return func(m *OpenAIModel) {
		m.mu.Lock()
		m.chatRepo = repo
		m.fileStorage = fileStorage
		m.mu.Unlock()
	}
}

// WithCodeInterpreterArtifactLimit sets the max number of session artifacts uploaded
// to OpenAI and attached to code_interpreter per request.
func WithCodeInterpreterArtifactLimit(limit int) OpenAIModelOption {
	return func(m *OpenAIModel) {
		if limit > 0 {
			m.mu.Lock()
			m.codeInterpreterArtifactLimit = limit
			m.mu.Unlock()
		}
	}
}

// toolCallAccumEntry accumulates streaming function call data.
type toolCallAccumEntry struct {
	id     string // item ID (used as map key)
	callID string // function call ID (used in API)
	name   string
	args   string
}

// NewOpenAIModel creates a new OpenAI model from environment variables.
// It reads OPENAI_API_KEY (required) and OPENAI_MODEL (optional, defaults to gpt-5.2-2025-12-11).
func NewOpenAIModel(opts ...OpenAIModelOption) (agents.Model, error) {
	const op serrors.Op = "llmproviders.NewOpenAIModel"

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, serrors.E(op, "OPENAI_API_KEY environment variable is required")
	}

	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = "gpt-5.2-2025-12-11"
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	m := &OpenAIModel{
		client:                       &client,
		modelName:                    modelName,
		logger:                       logging.NewNoOpLogger(),
		codeInterpreterMemoryLimit:   defaultCodeInterpreterMemoryLimit,
		codeInterpreterArtifactLimit: defaultCodeInterpreterFileLimit,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Generate sends a blocking request to the OpenAI Responses API.
func (m *OpenAIModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	const op serrors.Op = "OpenAIModel.Generate"

	config := agents.ApplyGenerateOptions(opts...)
	params := m.buildResponseParams(ctx, req, config)

	resp, err := m.client.Responses.New(ctx, params)
	if err != nil {
		return nil, serrors.E(op, err, "OpenAI API request failed")
	}

	return m.mapResponse(resp)
}

// Stream sends a streaming request to the OpenAI Responses API.
func (m *OpenAIModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	const op serrors.Op = "OpenAIModel.Stream"

	config := agents.ApplyGenerateOptions(opts...)
	params := m.buildResponseParams(ctx, req, config)

	stream := m.client.Responses.NewStreaming(ctx, params)

	return types.NewGenerator(ctx, func(genCtx context.Context, yield func(agents.Chunk) bool) error {
		defer func() {
			if err := stream.Close(); err != nil {
				m.logger.Warn(genCtx, "stream close error", map[string]any{
					"error": err.Error(),
					"model": m.modelName,
				})
			}
		}()

		// Accumulate function call arguments per call ID
		toolCallAccum := make(map[string]*toolCallAccumEntry)
		var toolCallOrder []string

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "response.output_text.delta":
				if !yield(agents.Chunk{Delta: event.Delta}) {
					return nil
				}

			case "response.function_call_arguments.delta":
				callID := event.ItemID
				if _, ok := toolCallAccum[callID]; !ok {
					toolCallAccum[callID] = &toolCallAccumEntry{id: callID}
					toolCallOrder = append(toolCallOrder, callID)
				}
				toolCallAccum[callID].args += event.Delta

			case "response.function_call_arguments.done":
				callID := event.ItemID
				if a, ok := toolCallAccum[callID]; ok {
					a.name = event.Name
					a.args = event.Arguments
				}

			case "response.output_item.done":
				if event.Item.Type == "function_call" {
					itemID := functionCallItemKey(event.Item, event.ItemID)
					if itemID == "" {
						m.logger.Warn(genCtx, "skipping function_call output_item.done without item id", map[string]any{
							"call_id": event.Item.CallID,
							"name":    event.Item.Name,
						})
						continue
					}
					if a, ok := toolCallAccum[itemID]; ok {
						// Populate callID from the completed item
						a.callID = event.Item.CallID
						if a.name == "" {
							a.name = event.Item.Name
						}
						if a.args == "" {
							a.args = event.Item.Arguments
						}
					} else {
						// Entry not created by delta events; create from done event
						toolCallAccum[itemID] = &toolCallAccumEntry{
							id:     itemID,
							callID: event.Item.CallID,
							name:   event.Item.Name,
							args:   event.Item.Arguments,
						}
						toolCallOrder = append(toolCallOrder, itemID)
					}

					// Emit a non-final chunk with any ready tool calls so executors can start early.
					readyToolCalls := m.buildReadyToolCallsFromAccum(toolCallAccum, toolCallOrder)
					if len(readyToolCalls) > 0 {
						if !yield(agents.Chunk{ToolCalls: readyToolCalls}) {
							return nil
						}
					}
				}

			case "response.completed":
				resp := event.Response
				agentResp, err := m.mapResponse(&resp)
				if err != nil {
					return serrors.E(op, err, "failed to map completed response")
				}

				// Build final chunk with all accumulated tool calls
				toolCalls := m.buildToolCallsFromAccum(toolCallAccum, toolCallOrder)
				if len(agentResp.Message.ToolCalls()) > 0 && len(toolCalls) == 0 {
					toolCalls = agentResp.Message.ToolCalls()
				}

				chunk := agents.Chunk{
					Done:                   true,
					ToolCalls:              toolCalls,
					Usage:                  &agentResp.Usage,
					FinishReason:           agentResp.FinishReason,
					Citations:              agentResp.Message.Citations(),
					CodeInterpreterResults: agentResp.CodeInterpreterResults,
					FileAnnotations:        agentResp.FileAnnotations,
					ProviderResponseID:     agentResp.ProviderResponseID,
				}
				if !yield(chunk) {
					return nil
				}
				return nil
			}
		}

		if err := stream.Err(); err != nil {
			return serrors.E(op, err, "stream error")
		}

		return nil
	}, types.WithBufferSize(10)), nil
}

// Info returns model metadata including capabilities.
func (m *OpenAIModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:          m.modelName,
		Provider:      "openai",
		ContextWindow: contextWindowForModel(m.modelName),
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityJSONMode,
		},
	}
}

func contextWindowForModel(modelName string) int {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))

	if strings.HasPrefix(normalizedModelName, "gpt-5.2") {
		return 272000
	}

	modelContextWindows := map[string]int{
		"gpt-4o":      128000,
		"gpt-4o-mini": 128000,
		"gpt-4-turbo": 128000,
	}

	if contextWindow, ok := modelContextWindows[normalizedModelName]; ok {
		return contextWindow
	}

	return 0
}

// HasCapability checks if this model supports a specific capability.
func (m *OpenAIModel) HasCapability(capability agents.Capability) bool {
	return m.Info().HasCapability(capability)
}

// ModelParameters returns the model-level parameters sent to the API.
// Implements agents.ModelParameterReporter for observability.
func (m *OpenAIModel) ModelParameters() map[string]interface{} {
	return map[string]interface{}{
		"store": true,
	}
}

// Pricing returns pricing information for this OpenAI model.
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	pricing := map[string]agents.ModelPricing{
		"gpt-5.2-2025-12-11": {
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.18,
		},
		"gpt-4o": {
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 1.25,
			CacheReadPer1M:  0.625,
		},
		"gpt-4o-mini": {
			Currency:        "USD",
			InputPer1M:      0.150,
			OutputPer1M:     0.600,
			CacheWritePer1M: 0.075,
			CacheReadPer1M:  0.038,
		},
		"gpt-4-turbo": {
			Currency:        "USD",
			InputPer1M:      10.00,
			OutputPer1M:     30.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
		"gpt-4": {
			Currency:        "USD",
			InputPer1M:      30.00,
			OutputPer1M:     60.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
	}

	if p, exists := pricing[m.modelName]; exists {
		return p
	}

	return agents.ModelPricing{
		Currency:        "USD",
		InputPer1M:      1.75,
		OutputPer1M:     14.00,
		CacheWritePer1M: 0,
		CacheReadPer1M:  0.18,
	}
}

// buildResponseParams converts agents.Request to OpenAI Responses API parameters.
func (m *OpenAIModel) buildResponseParams(ctx context.Context, req agents.Request, config agents.GenerateConfig) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model: m.modelName,
		Store: openai.Bool(true),
	}

	// Build input items from messages
	inputItems := m.buildInputItems(req.Messages)
	params.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: inputItems,
	}
	if req.PreviousResponseID != nil {
		previousResponseID := strings.TrimSpace(*req.PreviousResponseID)
		if previousResponseID != "" {
			params.PreviousResponseID = openai.String(previousResponseID)
		}
	}

	// Apply max tokens
	if config.MaxTokens != nil {
		params.MaxOutputTokens = openai.Int(int64(*config.MaxTokens))
	}

	// Apply temperature
	if config.Temperature != nil {
		params.Temperature = openai.Float(*config.Temperature)
	}

	// Apply JSON mode
	if config.JSONMode && m.HasCapability(agents.CapabilityJSONMode) {
		params.Text = responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			},
		}
	}

	// Convert tools — detect native tools vs function tools
	var tools []responses.ToolUnionParam
	var includes []responses.ResponseIncludable
	hasWebSearch := false
	hasCodeInterpreter := false
	codeInterpreterFileIDs := make([]string, 0)

	for _, tool := range req.Tools {
		if tool.Name() == "code_interpreter" {
			hasCodeInterpreter = true
			break
		}
	}
	if hasCodeInterpreter {
		codeInterpreterFileIDs = m.resolveCodeInterpreterFileIDs(ctx)
	}

	for _, tool := range req.Tools {
		switch tool.Name() {
		case "web_search":
			hasWebSearch = true
			tools = append(tools, responses.ToolUnionParam{
				OfWebSearch: &responses.WebSearchToolParam{
					Type: responses.WebSearchToolTypeWebSearch,
				},
			})
		case "code_interpreter":
			tools = append(tools, responses.ToolParamOfCodeInterpreter(
				responses.ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam{
					MemoryLimit: m.getCodeInterpreterMemoryLimit(),
					FileIDs:     codeInterpreterFileIDs,
				},
			))
		default:
			tools = append(tools, responses.ToolUnionParam{
				OfFunction: &responses.FunctionToolParam{
					Name:        tool.Name(),
					Description: openai.String(tool.Description()),
					Parameters:  tool.Parameters(),
				},
			})
		}
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	// Request additional output data for native tools
	if hasWebSearch {
		includes = append(includes, responses.ResponseIncludableWebSearchCallActionSources)
	}
	if hasCodeInterpreter {
		includes = append(includes, responses.ResponseIncludableCodeInterpreterCallOutputs)
	}
	if len(includes) > 0 {
		params.Include = includes
	}

	return params
}

// SetCodeInterpreterMemoryLimit updates the default memory limit for OpenAI code_interpreter containers.
func (m *OpenAIModel) SetCodeInterpreterMemoryLimit(limit string) error {
	const op serrors.Op = "OpenAIModel.SetCodeInterpreterMemoryLimit"
	normalized, ok := normalizeCodeInterpreterMemoryLimit(limit)
	if !ok {
		return serrors.E(op, serrors.KindValidation, "invalid code interpreter memory limit: must be one of 1g, 4g, 16g, 64g")
	}
	m.mu.Lock()
	m.codeInterpreterMemoryLimit = normalized
	m.mu.Unlock()
	return nil
}

// SetCodeInterpreterArtifactSource configures the session artifact source and storage
// used to upload attachments to OpenAI Files for code_interpreter.
func (m *OpenAIModel) SetCodeInterpreterArtifactSource(repo domain.ChatRepository, fileStorage storage.FileStorage) {
	m.mu.Lock()
	m.chatRepo = repo
	m.fileStorage = fileStorage
	m.mu.Unlock()
}

func normalizeCodeInterpreterMemoryLimit(limit string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(limit))
	switch normalized {
	case "1g", "4g", "16g", "64g":
		return normalized, true
	default:
		return "", false
	}
}

func (m *OpenAIModel) getCodeInterpreterMemoryLimit() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.codeInterpreterMemoryLimit
}

func (m *OpenAIModel) resolveCodeInterpreterFileIDs(ctx context.Context) []string {
	m.mu.RLock()
	chatRepo := m.chatRepo
	fileStorage := m.fileStorage
	client := m.client
	limit := m.codeInterpreterArtifactLimit
	m.mu.RUnlock()

	if chatRepo == nil || fileStorage == nil || client == nil {
		return nil
	}

	sessionID, ok := agents.UseRuntimeSessionID(ctx)
	if !ok {
		return nil
	}

	if limit <= 0 {
		limit = defaultCodeInterpreterFileLimit
	}

	artifacts, err := chatRepo.GetSessionArtifacts(ctx, sessionID, domain.ListOptions{
		Limit:  limit,
		Offset: 0,
		Types:  []domain.ArtifactType{domain.ArtifactTypeAttachment},
	})
	if err != nil {
		m.logger.Warn(ctx, "failed to list session artifacts for code_interpreter files", map[string]any{
			"session_id": sessionID.String(),
			"error":      err.Error(),
		})
		return nil
	}

	fileIDs := make([]string, 0, len(artifacts))
	seenURLs := make(map[string]struct{}, len(artifacts))
	var syncRepo artifactProviderFileSyncRepository
	if repo, ok := chatRepo.(artifactProviderFileSyncRepository); ok {
		syncRepo = repo
	}
	for _, artifact := range artifacts {
		fileURL := strings.TrimSpace(artifact.URL())
		if fileURL == "" {
			continue
		}
		if _, exists := seenURLs[fileURL]; exists {
			continue
		}
		seenURLs[fileURL] = struct{}{}

		if syncRepo != nil {
			providerFileID, sourceURL, sourceSizeBytes, mapErr := syncRepo.GetArtifactProviderFile(ctx, artifact.ID(), codeInterpreterProviderOpenAI)
			if mapErr == nil {
				if strings.TrimSpace(providerFileID) != "" &&
					sourceURL == fileURL &&
					sourceSizeBytes == artifact.SizeBytes() {
					fileIDs = append(fileIDs, providerFileID)
					if len(fileIDs) >= limit {
						break
					}
					continue
				}
			}
		}

		rc, err := fileStorage.Get(ctx, fileURL)
		if err != nil {
			m.logger.Warn(ctx, "failed to open artifact content for code_interpreter upload", map[string]any{
				"session_id":  sessionID.String(),
				"artifact_id": artifact.ID().String(),
				"url":         fileURL,
				"error":       err.Error(),
			})
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(rc, maxOpenAIFileUploadBytes+1))
		_ = rc.Close()
		if readErr != nil {
			m.logger.Warn(ctx, "failed to read artifact content for code_interpreter upload", map[string]any{
				"session_id":  sessionID.String(),
				"artifact_id": artifact.ID().String(),
				"url":         fileURL,
				"error":       readErr.Error(),
			})
			continue
		}
		if int64(len(data)) > maxOpenAIFileUploadBytes {
			m.logger.Warn(ctx, "artifact exceeds OpenAI file upload size limit", map[string]any{
				"session_id":        sessionID.String(),
				"artifact_id":       artifact.ID().String(),
				"url":               fileURL,
				"size_bytes":        artifact.SizeBytes(),
				"max_allowed_bytes": maxOpenAIFileUploadBytes,
			})
			continue
		}
		if len(data) == 0 {
			continue
		}

		filename := strings.TrimSpace(artifact.Name())
		if filename == "" {
			filename = "artifact.bin"
		}
		contentType := strings.TrimSpace(artifact.MimeType())

		uploaded, uploadErr := client.Files.New(ctx, openai.FileNewParams{
			File:    openai.File(bytes.NewReader(data), filename, contentType),
			Purpose: openai.FilePurposeAssistants,
		})
		if uploadErr != nil {
			m.logger.Warn(ctx, "failed to upload artifact to OpenAI for code_interpreter", map[string]any{
				"session_id":  sessionID.String(),
				"artifact_id": artifact.ID().String(),
				"filename":    filename,
				"mime_type":   contentType,
				"error":       uploadErr.Error(),
			})
			continue
		}
		if strings.TrimSpace(uploaded.ID) == "" {
			continue
		}

		if syncRepo != nil {
			if err := syncRepo.UpsertArtifactProviderFile(
				ctx,
				artifact.ID(),
				codeInterpreterProviderOpenAI,
				uploaded.ID,
				fileURL,
				artifact.SizeBytes(),
			); err != nil {
				m.logger.Warn(ctx, "failed to persist artifact/provider file mapping", map[string]any{
					"session_id":  sessionID.String(),
					"artifact_id": artifact.ID().String(),
					"provider":    codeInterpreterProviderOpenAI,
					"file_id":     uploaded.ID,
					"error":       err.Error(),
				})
			}
		}

		fileIDs = append(fileIDs, uploaded.ID)
		if len(fileIDs) >= limit {
			break
		}
	}

	return fileIDs
}

// buildInputItems converts types.Message slice to Responses API input items.
func (m *OpenAIModel) buildInputItems(messages []types.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(messages))
	skippedToolCallIDs := make(map[string]struct{})

	for _, msg := range messages {
		switch msg.Role() {
		case types.RoleSystem:
			items = append(items, responses.ResponseInputItemParamOfMessage(
				msg.Content(),
				responses.EasyInputMessageRoleDeveloper,
			))

		case types.RoleUser:
			if len(msg.Attachments()) > 0 {
				// Build multipart content with text + image inputs.
				// Non-image attachments are represented as text hints so the model uses artifact_reader.
				parts := make(responses.ResponseInputMessageContentListParam, 0, 1+len(msg.Attachments()))
				if msg.Content() != "" {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(msg.Content()))
				}
				nonImageNotes := make([]string, 0, len(msg.Attachments()))
				for _, attachment := range msg.Attachments() {
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(attachment.MimeType)), "image/") && strings.TrimSpace(attachment.FilePath) != "" {
						parts = append(parts, responses.ResponseInputContentUnionParam{
							OfInputImage: &responses.ResponseInputImageParam{
								ImageURL: openai.String(attachment.FilePath),
								Detail:   responses.ResponseInputImageDetailLow,
							},
						})
						continue
					}
					nonImageNotes = append(nonImageNotes, fmt.Sprintf("- %s (%s, %d bytes)", attachment.FileName, attachment.MimeType, attachment.SizeBytes))
				}
				if len(nonImageNotes) > 0 {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(
						"Attached files are available in this session. Use artifact_reader to inspect them:\n"+strings.Join(nonImageNotes, "\n"),
					))
				}
				if len(parts) == 0 {
					parts = append(parts, responses.ResponseInputContentParamOfInputText(msg.Content()))
				}
				items = append(items, responses.ResponseInputItemParamOfMessage(
					parts,
					responses.EasyInputMessageRoleUser,
				))
			} else {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleUser,
				))
			}

		case types.RoleAssistant:
			// Emit text content as an assistant message (if present)
			if msg.Content() != "" {
				items = append(items, responses.ResponseInputItemParamOfMessage(
					msg.Content(),
					responses.EasyInputMessageRoleAssistant,
				))
			}
			// Emit each tool call as a separate function_call input item
			for _, tc := range msg.ToolCalls() {
				callID := strings.TrimSpace(tc.ID)
				callName := strings.TrimSpace(tc.Name)
				if callID == "" || callName == "" {
					m.logger.Warn(context.Background(), "skipping tool call with empty name or ID in buildInputItems", map[string]any{
						"call_id": tc.ID,
						"name":    tc.Name,
					})
					if callID != "" {
						skippedToolCallIDs[callID] = struct{}{}
					}
					continue
				}

				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					tc.Arguments,
					callID,
					callName,
				))
			}

		case types.RoleTool:
			if msg.ToolCallID() != nil {
				callID := strings.TrimSpace(*msg.ToolCallID())
				if callID == "" {
					continue
				}
				if _, skipped := skippedToolCallIDs[callID]; skipped {
					continue
				}
				items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
					callID,
					msg.Content(),
				))
			}
		}
	}

	return items
}

// mapResponse converts a Responses API Response to agents.Response.
func (m *OpenAIModel) mapResponse(resp *responses.Response) (*agents.Response, error) {
	var content string
	var toolCalls []types.ToolCall
	var citations []types.Citation
	var codeResults []types.CodeInterpreterResult
	var fileAnnotations []types.FileAnnotation

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					content += part.Text

					// Extract citations from annotations
					for _, ann := range part.Annotations {
						if ann.Type == "url_citation" {
							citations = append(citations, types.Citation{
								Type:       "web",
								Title:      ann.Title,
								URL:        ann.URL,
								StartIndex: int(ann.StartIndex),
								EndIndex:   int(ann.EndIndex),
							})
						}
						if ann.Type == "container_file_citation" {
							fileAnnotations = append(fileAnnotations, types.FileAnnotation{
								Type:        ann.Type,
								ContainerID: ann.ContainerID,
								FileID:      ann.FileID,
								Filename:    ann.Filename,
								StartIndex:  int(ann.StartIndex),
								EndIndex:    int(ann.EndIndex),
							})
						}
					}
				}
			}

		case "function_call":
			toolCalls = append(toolCalls, types.ToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})

		case "code_interpreter_call":
			result := types.CodeInterpreterResult{
				ID:          item.ID,
				Code:        item.Code,
				ContainerID: item.ContainerID,
				Status:      item.Status,
			}
			for _, out := range item.Outputs {
				if out.Type == "logs" {
					result.Logs = out.Logs
				}
			}
			codeResults = append(codeResults, result)
		}
	}

	// Determine finish reason
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	if resp.Status == "incomplete" {
		finishReason = "length"
	}

	// Build message
	msgOpts := []types.MessageOption{}
	if len(toolCalls) > 0 {
		msgOpts = append(msgOpts, types.WithToolCalls(toolCalls...))
	}
	if len(citations) > 0 {
		msgOpts = append(msgOpts, types.WithCitations(citations...))
	}
	msg := types.AssistantMessage(content, msgOpts...)

	// Build usage
	usage := types.TokenUsage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		TotalTokens:      int(resp.Usage.TotalTokens),
	}
	if resp.Usage.InputTokensDetails.CachedTokens > 0 {
		usage.CacheReadTokens = int(resp.Usage.InputTokensDetails.CachedTokens)
	}

	return &agents.Response{
		Message:                msg,
		Usage:                  usage,
		FinishReason:           finishReason,
		CodeInterpreterResults: codeResults,
		FileAnnotations:        fileAnnotations,
		ProviderResponseID:     resp.ID,
	}, nil
}

// buildToolCallsFromAccum converts accumulated tool call data to types.ToolCall slice.
func (m *OpenAIModel) buildToolCallsFromAccum(accum map[string]*toolCallAccumEntry, order []string) []types.ToolCall {
	if len(accum) == 0 {
		return nil
	}
	merged := make(map[string]types.ToolCall, len(accum))
	callOrder := make([]string, 0, len(accum))
	for _, key := range order {
		if a, ok := accum[key]; ok {
			// Prefer callID (from output_item.done) over itemID
			id := strings.TrimSpace(a.callID)
			if id == "" {
				id = strings.TrimSpace(a.id)
			}
			name := strings.TrimSpace(a.name)
			if id == "" || name == "" {
				continue
			}

			if _, exists := merged[id]; !exists {
				callOrder = append(callOrder, id)
			}

			merged[id] = types.ToolCall{
				ID:        id,
				Name:      name,
				Arguments: a.args,
			}
		}
	}

	calls := make([]types.ToolCall, 0, len(callOrder))
	for _, callID := range callOrder {
		calls = append(calls, merged[callID])
	}

	return calls
}

// buildReadyToolCallsFromAccum returns tool calls that are ready to execute during streaming.
// A tool call is considered ready once we have a stable CallID and Name.
func (m *OpenAIModel) buildReadyToolCallsFromAccum(accum map[string]*toolCallAccumEntry, order []string) []types.ToolCall {
	if len(accum) == 0 {
		return nil
	}
	calls := make([]types.ToolCall, 0, len(accum))
	seen := make(map[string]struct{}, len(accum))
	for _, key := range order {
		a, ok := accum[key]
		if !ok {
			continue
		}
		callID := strings.TrimSpace(a.callID)
		name := strings.TrimSpace(a.name)
		if callID == "" || name == "" {
			continue
		}
		if _, exists := seen[callID]; exists {
			continue
		}
		seen[callID] = struct{}{}
		calls = append(calls, types.ToolCall{
			ID:        callID,
			Name:      name,
			Arguments: a.args,
		})
	}
	return calls
}

func functionCallItemKey(item responses.ResponseOutputItemUnion, fallback string) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return id
	}
	if id := strings.TrimSpace(fallback); id != "" {
		return id
	}
	if callID := strings.TrimSpace(item.CallID); callID != "" {
		return callID
	}
	return ""
}
