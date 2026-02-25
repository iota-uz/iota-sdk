package services

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	hitlsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/hitl"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func cloneAttachmentsForMessage(messageID uuid.UUID, attachments []domain.Attachment) []domain.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	out := make([]domain.Attachment, len(attachments))
	for i, att := range attachments {
		opts := []domain.AttachmentOption{
			domain.WithAttachmentMessageID(messageID),
			domain.WithFileName(att.FileName()),
			domain.WithMimeType(att.MimeType()),
			domain.WithSizeBytes(att.SizeBytes()),
			domain.WithFilePath(att.FilePath()),
		}
		if att.UploadID() != nil {
			opts = append(opts, domain.WithUploadID(*att.UploadID()))
		}
		out[i] = domain.NewAttachment(opts...)
	}

	return out
}

func recordToolEvent(toolCalls map[string]types.ToolCall, toolOrder *[]string, tool *agents.ToolEvent) {
	if tool == nil {
		return
	}

	key := tool.CallID
	if key == "" {
		key = fmt.Sprintf("__unnamed_tool_%d", len(*toolOrder))
	}

	call, exists := toolCalls[key]
	if !exists {
		call = types.ToolCall{
			ID:        key,
			Name:      tool.Name,
			Arguments: tool.Arguments,
		}
		*toolOrder = append(*toolOrder, key)
	}

	if call.ID == "" {
		call.ID = key
	}
	if tool.Name != "" {
		call.Name = tool.Name
	}
	if tool.Arguments != "" {
		call.Arguments = tool.Arguments
	}
	if tool.Result != "" {
		call.Result = tool.Result
	}
	if tool.Error != nil {
		call.Error = tool.Error.Error()
	}
	if tool.DurationMs > 0 {
		call.DurationMs = tool.DurationMs
	}

	toolCalls[key] = call
}

func recordToolArtifacts(artifactMap map[string]types.ToolArtifact, artifacts []types.ToolArtifact) {
	for _, artifact := range artifacts {
		key := toolArtifactDedupeKey(artifact)
		if key == "" {
			continue
		}
		artifactMap[key] = artifact
	}
}

func collectCodeInterpreterArtifacts(
	executions []types.CodeInterpreterResult,
	annotations []types.FileAnnotation,
) []types.ToolArtifact {
	artifacts := make([]types.ToolArtifact, 0)
	for _, execution := range executions {
		for idx, output := range execution.Outputs {
			if output.Type != "image" || strings.TrimSpace(output.URL) == "" {
				continue
			}
			name := inferNameFromURL(output.URL)
			if name == "" {
				name = fmt.Sprintf("code-output-%d.png", idx+1)
			}
			artifacts = append(artifacts, types.ToolArtifact{
				Type:     string(domain.ArtifactTypeCodeOutput),
				Name:     name,
				MimeType: inferMimeTypeFromName(name),
				URL:      output.URL,
				Metadata: map[string]any{
					"container_id": execution.ContainerID,
					"execution_id": execution.ID,
				},
			})
		}
	}
	for _, annotation := range annotations {
		name := strings.TrimSpace(annotation.Filename)
		if name == "" {
			name = "code-output"
		}
		artifacts = append(artifacts, types.ToolArtifact{
			Type:     string(domain.ArtifactTypeCodeOutput),
			Name:     name,
			MimeType: inferMimeTypeFromName(name),
			Metadata: map[string]any{
				"annotation_type": annotation.Type,
				"container_id":    annotation.ContainerID,
				"file_id":         annotation.FileID,
			},
		})
	}
	return artifacts
}

func toolArtifactDedupeKey(artifact types.ToolArtifact) string {
	parts := []string{
		strings.TrimSpace(artifact.Type),
		strings.TrimSpace(artifact.Name),
		strings.TrimSpace(artifact.URL),
	}
	if len(parts[0]) == 0 && len(parts[1]) == 0 && len(parts[2]) == 0 {
		return ""
	}
	return strings.Join(parts, "|")
}

func inferNameFromURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	base := path.Base(strings.TrimSpace(parsed.Path))
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func inferMimeTypeFromName(name string) string {
	ext := strings.ToLower(path.Ext(strings.TrimSpace(name)))
	if ext == "" {
		return ""
	}
	return mime.TypeByExtension(ext)
}

func mapsValues(in map[string]types.ToolArtifact) []types.ToolArtifact {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.ToolArtifact, 0, len(in))
	for _, value := range in {
		out = append(out, value)
	}
	return out
}

func (s *chatServiceImpl) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if _, err := composables.UsePool(ctx); errors.Is(err, composables.ErrNoPool) {
		return fn(ctx)
	}
	return composables.InTx(ctx, fn)
}

func (s *chatServiceImpl) maybeReplaceHistoryFromMessage(
	ctx context.Context,
	session domain.Session,
	replaceFromMessageID *uuid.UUID,
) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.maybeReplaceHistoryFromMessage"

	if replaceFromMessageID == nil {
		return session, nil
	}

	msg, err := s.chatRepo.GetMessage(ctx, *replaceFromMessageID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if msg.SessionID() != session.ID() {
		return nil, serrors.E(op, serrors.KindValidation, "replaceFromMessageId does not belong to session")
	}
	if msg.Role() != types.RoleUser {
		return nil, serrors.E(op, serrors.KindValidation, "replaceFromMessageId must point to a user message")
	}

	if _, err := s.chatRepo.TruncateMessagesFrom(ctx, session.ID(), msg.CreatedAt()); err != nil {
		return nil, serrors.E(op, err)
	}

	updated := session.
		UpdateLLMPreviousResponseID(nil).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}

	return updated, nil
}

func orderedToolCalls(toolCalls map[string]types.ToolCall, toolOrder []string) []types.ToolCall {
	if len(toolOrder) == 0 {
		return nil
	}

	result := make([]types.ToolCall, 0, len(toolOrder))
	for _, key := range toolOrder {
		call, ok := toolCalls[key]
		if !ok {
			continue
		}
		result = append(result, call)
	}

	return result
}

func buildDebugTrace(
	sessionID uuid.UUID,
	traceID string,
	toolCalls []types.ToolCall,
	usage *types.DebugUsage,
	generationMs int64,
	thinking string,
	observationReason string,
	model string,
	provider string,
	requestID string,
	finishReason string,
	input string,
	output string,
	startedAt time.Time,
) *types.DebugTrace {
	debugTools := make([]types.DebugToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		debugTools = append(debugTools, types.DebugToolCall{
			CallID:     toolCall.ID,
			Name:       toolCall.Name,
			Arguments:  toolCall.Arguments,
			Result:     toolCall.Result,
			Error:      toolCall.Error,
			DurationMs: toolCall.DurationMs,
		})
	}

	trimmedTraceID := strings.TrimSpace(traceID)
	if trimmedTraceID == "" {
		trimmedTraceID = sessionID.String()
	}
	traceURL := buildLangfuseTraceURL(trimmedTraceID)
	obsReason := strings.TrimSpace(observationReason)

	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	completedAt := startedAt
	if generationMs > 0 {
		completedAt = startedAt.Add(time.Duration(generationMs) * time.Millisecond)
	}

	attemptID := strings.TrimSpace(requestID)
	if attemptID == "" {
		attemptID = uuid.NewString()
	}

	attempt := types.DebugGeneration{
		ID:                attemptID,
		RequestID:         strings.TrimSpace(requestID),
		Model:             strings.TrimSpace(model),
		Provider:          strings.TrimSpace(provider),
		FinishReason:      strings.TrimSpace(finishReason),
		LatencyMs:         generationMs,
		Input:             strings.TrimSpace(input),
		Output:            strings.TrimSpace(output),
		Thinking:          thinking,
		ObservationReason: obsReason,
		StartedAt:         startedAt.UTC().Format(time.RFC3339),
		CompletedAt:       completedAt.UTC().Format(time.RFC3339),
		ToolCalls:         debugTools,
	}
	if usage != nil {
		attempt.PromptTokens = usage.PromptTokens
		attempt.CompletionTokens = usage.CompletionTokens
		attempt.TotalTokens = usage.TotalTokens
		attempt.CachedTokens = usage.CachedTokens
		attempt.Cost = usage.Cost
	}

	spans := make([]types.DebugSpan, 0, len(debugTools))
	for _, tool := range debugTools {
		status := "success"
		outputValue := tool.Result
		errorValue := ""
		if strings.TrimSpace(tool.Error) != "" {
			status = "error"
			outputValue = ""
			errorValue = tool.Error
		}
		spanID := strings.TrimSpace(tool.CallID)
		if spanID == "" {
			spanID = uuid.NewString()
		}
		spans = append(spans, types.DebugSpan{
			ID:           spanID,
			GenerationID: attemptID,
			Name:         "tool.execute",
			Type:         "tool",
			Status:       status,
			CallID:       tool.CallID,
			ToolName:     tool.Name,
			Input:        tool.Arguments,
			Output:       outputValue,
			Error:        errorValue,
			DurationMs:   tool.DurationMs,
			StartedAt:    completedAt.UTC().Format(time.RFC3339),
			CompletedAt:  completedAt.UTC().Format(time.RFC3339),
			Attributes: map[string]interface{}{
				"tool_name": tool.Name,
				"call_id":   tool.CallID,
			},
		})
	}

	events := make([]types.DebugEvent, 0, 1)
	if obsReason != "" {
		events = append(events, types.DebugEvent{
			ID:        uuid.NewString(),
			Name:      "observation",
			Type:      "diagnostic",
			Level:     "warning",
			Reason:    obsReason,
			Message:   obsReason,
			Timestamp: completedAt.UTC().Format(time.RFC3339),
		})
	}

	return &types.DebugTrace{
		SchemaVersion:     "v2",
		StartedAt:         startedAt.UTC().Format(time.RFC3339),
		CompletedAt:       completedAt.UTC().Format(time.RFC3339),
		Usage:             usage,
		GenerationMs:      generationMs,
		Tools:             debugTools,
		Attempts:          []types.DebugGeneration{attempt},
		Spans:             spans,
		Events:            events,
		TraceID:           trimmedTraceID,
		TraceURL:          traceURL,
		SessionID:         sessionID.String(),
		Thinking:          thinking,
		ObservationReason: obsReason,
	}
}

func buildLangfuseTraceURL(traceID string) string {
	trimmedTraceID := strings.TrimSpace(traceID)
	if trimmedTraceID == "" {
		return ""
	}

	baseURL := strings.TrimSpace(os.Getenv("LANGFUSE_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("LANGFUSE_HOST"))
	}
	if baseURL == "" {
		return ""
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return ""
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return ""
	}

	parsedBaseURL.RawQuery = ""
	parsedBaseURL.Fragment = ""
	basePath := strings.TrimRight(parsedBaseURL.Path, "/")
	rawPath := basePath + "/trace/" + url.PathEscape(trimmedTraceID)
	parsedBaseURL.Path = basePath + "/trace/" + trimmedTraceID
	parsedBaseURL.RawPath = rawPath
	return parsedBaseURL.String()
}

func optionalStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// agentResult holds the collected output from processing an agent event generator.
type agentResult struct {
	content            string
	toolCalls          []types.ToolCall
	artifacts          []types.ToolArtifact
	interrupt          *bichatservices.Interrupt
	interruptAgentName string
	providerResponseID *string
	usage              *types.DebugUsage
	traceID            string
	requestID          string
	model              string
	provider           string
	finishReason       string
	thinking           string
	observationReason  string
	lastError          error
}

// consumeAgentEvents drains the generator and collects the result.
// This is used by non-streaming callers (SendMessage, ResumeWithAnswer, RejectPendingQuestion).
func consumeAgentEvents(ctx context.Context, gen types.Generator[agents.ExecutorEvent]) (*agentResult, error) {
	var content strings.Builder
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)
	artifactMap := make(map[string]types.ToolArtifact)
	var interrupt *bichatservices.Interrupt
	var interruptAgentName string
	var providerResponseID *string
	var finalUsage *types.DebugUsage
	var traceID string
	var requestID string
	var model string
	var provider string
	var finishReason string
	var thinking strings.Builder
	var observationReason string
	var lastError error

	for {
		event, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			return nil, err
		}

		switch event.Type {
		case agents.EventTypeContent:
			content.WriteString(event.Content)
		case agents.EventTypeThinking:
			if event.Content != "" {
				thinking.WriteString(event.Content)
			}
			// Reasoning/thinking content from LLM; not appended to user-visible content
		case agents.EventTypeToolStart, agents.EventTypeToolEnd:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)
			if event.Tool != nil && len(event.Tool.Artifacts) > 0 {
				recordToolArtifacts(artifactMap, event.Tool.Artifacts)
			}
		case agents.EventTypeInterrupt:
			if event.ParsedInterrupt != nil {
				pi := event.ParsedInterrupt
				interrupt = &bichatservices.Interrupt{
					CheckpointID: pi.CheckpointID,
					Questions:    hitlsvc.AgentQuestionsToServiceQuestions(pi.Questions),
				}
				interruptAgentName = pi.AgentName
				if interruptAgentName == "" {
					interruptAgentName = "default-agent"
				}
				providerResponseID = optionalStringPtr(pi.ProviderResponseID)
			}
		case agents.EventTypeDone:
			providerResponseID = optionalStringPtr(event.ProviderResponseID)
			if event.Result != nil {
				if event.Result.TraceID != "" {
					traceID = event.Result.TraceID
				}
				requestID = event.Result.RequestID
				model = event.Result.Model
				provider = event.Result.Provider
				finishReason = event.Result.FinishReason
				if event.Result.Thinking != "" {
					thinking.Reset()
					thinking.WriteString(event.Result.Thinking)
				}
			}
			if event.Usage != nil {
				finalUsage = event.Usage
			}
			recordToolArtifacts(artifactMap, collectCodeInterpreterArtifacts(event.CodeInterpreter, event.FileAnnotations))

		case agents.EventTypeError:
			var errDetail error
			if event.Error != nil {
				errDetail = event.Error
			} else if event.Content != "" {
				errDetail = fmt.Errorf("%s", event.Content)
			} else {
				errDetail = fmt.Errorf("agent error")
			}
			if event.ProviderResponseID != "" {
				providerResponseID = optionalStringPtr(event.ProviderResponseID)
				lastError = fmt.Errorf("providerResponseID=%s: %w", event.ProviderResponseID, errDetail)
			} else {
				lastError = errDetail
			}
		}
	}

	result := &agentResult{
		content:            content.String(),
		toolCalls:          orderedToolCalls(toolCalls, toolOrder),
		artifacts:          mapsValues(artifactMap),
		interrupt:          interrupt,
		interruptAgentName: interruptAgentName,
		providerResponseID: providerResponseID,
		usage:              finalUsage,
		traceID:            traceID,
		requestID:          requestID,
		model:              model,
		provider:           provider,
		finishReason:       finishReason,
		thinking:           thinking.String(),
		observationReason:  observationReason,
		lastError:          lastError,
	}
	if result.observationReason == "" && strings.TrimSpace(result.content) == "" && len(result.toolCalls) == 0 {
		result.observationReason = "empty_assistant_output"
	}
	if lastError != nil {
		return result, lastError
	}
	return result, nil
}

// saveAgentResult builds and persists the assistant message and updates the session.
func (s *chatServiceImpl) saveAgentResult(
	ctx context.Context,
	op serrors.Op,
	session domain.Session,
	sessionID uuid.UUID,
	result *agentResult,
	startedAt time.Time,
	userInput string,
) (types.Message, domain.Session, error) {
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(sessionID)}
	if len(result.toolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(result.toolCalls...))
	}
	if debugTrace := buildDebugTrace(
		sessionID,
		result.traceID,
		result.toolCalls,
		result.usage,
		time.Since(startedAt).Milliseconds(),
		result.thinking,
		result.observationReason,
		result.model,
		result.provider,
		result.requestID,
		result.finishReason,
		userInput,
		result.content,
		startedAt,
	); debugTrace != nil {
		assistantMsgOpts = append(assistantMsgOpts, types.WithDebugTrace(debugTrace))
	}

	if result.interrupt != nil {
		qd, err := hitlsvc.BuildQuestionData(result.interrupt.CheckpointID, result.interruptAgentName, result.interrupt.Questions)
		if err != nil {
			return nil, nil, serrors.E(op, err)
		}
		if qd != nil {
			assistantMsgOpts = append(assistantMsgOpts, types.WithQuestionData(qd))
		}
	}

	assistantMsg := types.AssistantMessage(result.content, assistantMsgOpts...)
	if err := s.chatRepo.SaveMessage(ctx, assistantMsg); err != nil {
		return nil, nil, serrors.E(op, err)
	}
	if err := s.persistGeneratedArtifacts(ctx, session, assistantMsg.ID(), result.artifacts); err != nil {
		return nil, nil, serrors.E(op, err)
	}

	session = session.
		UpdateLLMPreviousResponseID(result.providerResponseID).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
		return nil, nil, serrors.E(op, err)
	}

	return assistantMsg, session, nil
}

func (s *chatServiceImpl) persistGeneratedArtifacts(
	ctx context.Context,
	session domain.Session,
	messageID uuid.UUID,
	artifacts []types.ToolArtifact,
) error {
	const op serrors.Op = "chatServiceImpl.persistGeneratedArtifacts"
	if len(artifacts) == 0 {
		return nil
	}

	for idx, artifact := range artifacts {
		artifactType := strings.TrimSpace(artifact.Type)
		if artifactType == "" {
			artifactType = string(domain.ArtifactTypeCodeOutput)
		}
		name := strings.TrimSpace(artifact.Name)
		if name == "" {
			name = fmt.Sprintf("artifact-%d", idx+1)
		}
		mimeType := strings.TrimSpace(artifact.MimeType)
		if mimeType == "" {
			mimeType = inferMimeTypeFromName(name)
		}
		url := strings.TrimSpace(artifact.URL)

		msgID := messageID
		opts := []domain.ArtifactOption{
			domain.WithArtifactTenantID(session.TenantID()),
			domain.WithArtifactSessionID(session.ID()),
			domain.WithArtifactMessageID(&msgID),
			domain.WithArtifactType(domain.ArtifactType(artifactType)),
			domain.WithArtifactName(name),
			domain.WithArtifactDescription(strings.TrimSpace(artifact.Description)),
			domain.WithArtifactMimeType(mimeType),
			domain.WithArtifactURL(url),
			domain.WithArtifactSizeBytes(artifact.SizeBytes),
			domain.WithArtifactStatus(domain.ArtifactStatusAvailable),
			domain.WithArtifactIdempotencyKey(fmt.Sprintf("assistant:%s:%s:%d", messageID.String(), artifactType, idx)),
		}
		if len(artifact.Metadata) > 0 {
			opts = append(opts, domain.WithArtifactMetadata(artifact.Metadata))
		}
		if err := s.chatRepo.SaveArtifact(ctx, domain.NewArtifact(opts...)); err != nil {
			return serrors.E(op, err)
		}
	}

	return nil
}

// GetSessionMessages retrieves all messages for a session.
