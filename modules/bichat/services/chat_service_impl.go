package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// chatServiceImpl is the production implementation of ChatService.
// It orchestrates chat sessions, messages, and agent execution.
type chatServiceImpl struct {
	chatRepo     domain.ChatRepository
	agentService bichatservices.AgentService
	model        agents.Model
	titleService TitleGenerationService
}

// NewChatService creates a production implementation of ChatService.
//
// Example:
//
//	service := NewChatService(chatRepo, agentService, model, titleService)
func NewChatService(
	chatRepo domain.ChatRepository,
	agentService bichatservices.AgentService,
	model agents.Model,
	titleService TitleGenerationService,
) bichatservices.ChatService {
	return &chatServiceImpl{
		chatRepo:     chatRepo,
		agentService: agentService,
		model:        model,
		titleService: titleService,
	}
}

// CreateSession creates a new chat session.
func (s *chatServiceImpl) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.CreateSession"

	session := domain.NewSession(
		domain.WithTenantID(tenantID),
		domain.WithUserID(userID),
		domain.WithTitle(title),
	)
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, serrors.E(op, err)
	}
	return session, nil
}

// GetSession retrieves a session by ID.
func (s *chatServiceImpl) GetSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.GetSession"
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return session, nil
}

// ListUserSessions lists all sessions for a user.
func (s *chatServiceImpl) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ListUserSessions"
	sessions, err := s.chatRepo.ListUserSessions(ctx, userID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return sessions, nil
}

// CountUserSessions returns the total number of sessions for a user matching the same filter as ListUserSessions.
func (s *chatServiceImpl) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountUserSessions"
	count, err := s.chatRepo.CountUserSessions(ctx, userID, opts)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// ArchiveSession archives a session.
func (s *chatServiceImpl) ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ArchiveSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateStatus(domain.SessionStatusArchived)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UnarchiveSession unarchives a session.
func (s *chatServiceImpl) UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UnarchiveSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateStatus(domain.SessionStatusActive)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// PinSession pins a session.
func (s *chatServiceImpl) PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.PinSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdatePinned(true)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UnpinSession unpins a session.
func (s *chatServiceImpl) UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UnpinSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdatePinned(false)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UpdateSessionTitle updates the title of a session.
func (s *chatServiceImpl) UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UpdateSessionTitle"

	if title == "" {
		return nil, serrors.E(op, serrors.KindValidation, "title cannot be empty")
	}

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateTitle(title)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// DeleteSession deletes a session and all its messages.
func (s *chatServiceImpl) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.DeleteSession"

	// Repository handles cascade deletion of messages and attachments
	err := s.chatRepo.DeleteSession(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// ClearSessionHistory removes all messages/artifacts while preserving session metadata.
func (s *chatServiceImpl) ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (bichatservices.ClearSessionHistoryResponse, error) {
	const op serrors.Op = "chatServiceImpl.ClearSessionHistory"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return bichatservices.ClearSessionHistoryResponse{}, serrors.E(op, err)
	}

	deletedMessages, err := s.chatRepo.TruncateMessagesFrom(ctx, sessionID, time.Unix(0, 0))
	if err != nil {
		return bichatservices.ClearSessionHistoryResponse{}, serrors.E(op, err)
	}

	deletedArtifacts, err := s.chatRepo.DeleteSessionArtifacts(ctx, sessionID)
	if err != nil {
		return bichatservices.ClearSessionHistoryResponse{}, serrors.E(op, err)
	}

	updated := session.
		UpdateLLMPreviousResponseID(nil).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return bichatservices.ClearSessionHistoryResponse{}, serrors.E(op, err)
	}

	return bichatservices.ClearSessionHistoryResponse{
		Success:          true,
		DeletedMessages:  deletedMessages,
		DeletedArtifacts: deletedArtifacts,
	}, nil
}

// CompactSessionHistory replaces full session history with a compacted summary turn.
func (s *chatServiceImpl) CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (bichatservices.CompactSessionHistoryResponse, error) {
	const op serrors.Op = "chatServiceImpl.CompactSessionHistory"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, domain.ListOptions{
		Limit:  5000,
		Offset: 0,
	})
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	summary, err := s.generateCompactionSummary(ctx, messages)
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	deletedMessages, err := s.chatRepo.TruncateMessagesFrom(ctx, sessionID, time.Unix(0, 0))
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	deletedArtifacts, err := s.chatRepo.DeleteSessionArtifacts(ctx, sessionID)
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	systemMsg := types.SystemMessage(summary, types.WithSessionID(sessionID))
	if err := s.chatRepo.SaveMessage(ctx, systemMsg); err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	updated := session.
		UpdateLLMPreviousResponseID(nil).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	return bichatservices.CompactSessionHistoryResponse{
		Success:          true,
		Summary:          summary,
		DeletedMessages:  deletedMessages,
		DeletedArtifacts: deletedArtifacts,
	}, nil
}

// SendMessage sends a message to a session and processes it with the agent.
func (s *chatServiceImpl) SendMessage(ctx context.Context, req bichatservices.SendMessageRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.SendMessage"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	session, err = s.maybeReplaceHistoryFromMessage(ctx, session, req.ReplaceFromMessageID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Create user message
	userMsg := types.UserMessage(req.Content, types.WithSessionID(req.SessionID))

	// Save user message
	err = s.chatRepo.SaveMessage(ctx, userMsg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	processCtx := bichatservices.WithArtifactMessageID(ctx, userMsg.ID())

	// Convert attachments to domain attachments
	domainAttachments := make([]domain.Attachment, len(req.Attachments))
	for i, att := range req.Attachments {
		domainAttachments[i] = domain.NewAttachment(
			domain.WithAttachmentMessageID(userMsg.ID()),
			domain.WithFileName(att.FileName()),
			domain.WithMimeType(att.MimeType()),
			domain.WithSizeBytes(att.SizeBytes()),
			domain.WithFilePath(att.FilePath()),
		)
	}

	for _, att := range domainAttachments {
		if err := s.chatRepo.SaveAttachment(ctx, att); err != nil {
			return nil, serrors.E(op, err)
		}
		msgID := userMsg.ID()
		artifact := domain.NewArtifact(
			domain.WithArtifactTenantID(session.TenantID()),
			domain.WithArtifactSessionID(session.ID()),
			domain.WithArtifactMessageID(&msgID),
			domain.WithArtifactType(domain.ArtifactTypeAttachment),
			domain.WithArtifactName(att.FileName()),
			domain.WithArtifactMimeType(att.MimeType()),
			domain.WithArtifactURL(att.FilePath()),
			domain.WithArtifactSizeBytes(att.SizeBytes()),
		)
		if err := s.chatRepo.SaveArtifact(ctx, artifact); err != nil {
			return nil, serrors.E(op, err)
		}
	}

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(processCtx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(processCtx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	assistantMsg, session, err := s.saveAgentResult(ctx, op, session, req.SessionID, result, startedAt)
	if err != nil {
		return nil, err
	}

	if result.interrupt == nil {
		s.maybeGenerateTitleAsync(ctx, req.SessionID)
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      userMsg,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// SendMessageStream sends a message and streams the response via callback.
func (s *chatServiceImpl) SendMessageStream(ctx context.Context, req bichatservices.SendMessageRequest, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.SendMessageStream"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return serrors.E(op, err)
	}
	session, err = s.maybeReplaceHistoryFromMessage(ctx, session, req.ReplaceFromMessageID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Create user message
	userMsg := types.UserMessage(req.Content, types.WithSessionID(req.SessionID))

	// Save user message
	err = s.chatRepo.SaveMessage(ctx, userMsg)
	if err != nil {
		return serrors.E(op, err)
	}

	processCtx := bichatservices.WithArtifactMessageID(ctx, userMsg.ID())

	// Convert attachments to domain attachments
	domainAttachments := make([]domain.Attachment, len(req.Attachments))
	for i, att := range req.Attachments {
		domainAttachments[i] = domain.NewAttachment(
			domain.WithAttachmentMessageID(userMsg.ID()),
			domain.WithFileName(att.FileName()),
			domain.WithMimeType(att.MimeType()),
			domain.WithSizeBytes(att.SizeBytes()),
			domain.WithFilePath(att.FilePath()),
		)
	}

	for _, att := range domainAttachments {
		if err := s.chatRepo.SaveAttachment(ctx, att); err != nil {
			return serrors.E(op, err)
		}
		msgID := userMsg.ID()
		artifact := domain.NewArtifact(
			domain.WithArtifactTenantID(session.TenantID()),
			domain.WithArtifactSessionID(session.ID()),
			domain.WithArtifactMessageID(&msgID),
			domain.WithArtifactType(domain.ArtifactTypeAttachment),
			domain.WithArtifactName(att.FileName()),
			domain.WithArtifactMimeType(att.MimeType()),
			domain.WithArtifactURL(att.FilePath()),
			domain.WithArtifactSizeBytes(att.SizeBytes()),
		)
		if err := s.chatRepo.SaveArtifact(ctx, artifact); err != nil {
			return serrors.E(op, err)
		}
	}

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(processCtx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return serrors.E(op, err)
	}
	defer gen.Close()

	// Stream agent response
	var assistantContent strings.Builder
	var interrupt *bichatservices.Interrupt
	var interruptAgentName string
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)
	var providerResponseID *string
	var finalUsage *types.DebugUsage
	var generationMs int64

	for {
		event, err := gen.Next(processCtx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			// Send error chunk
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeError,
				Error:     err,
				Timestamp: time.Now(),
			})
			return serrors.E(op, err)
		}

		switch event.Type {
		case bichatservices.EventTypeContent:
			assistantContent.WriteString(event.Content)
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeContent,
				Content:   event.Content,
				Timestamp: time.Now(),
			})

		case bichatservices.EventTypeToolStart:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)
			if event.Tool != nil {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeToolStart,
					Tool:      event.Tool,
					Timestamp: time.Now(),
				})
			}

		case bichatservices.EventTypeToolEnd:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)
			if event.Tool != nil {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeToolEnd,
					Tool:      event.Tool,
					Timestamp: time.Now(),
				})
			}

		case bichatservices.EventTypeInterrupt:
			if event.Interrupt == nil {
				continue
			}
			interrupt = &bichatservices.Interrupt{
				CheckpointID: event.Interrupt.CheckpointID,
				Questions:    event.Interrupt.Questions,
			}
			interruptAgentName = event.Interrupt.AgentName
			if interruptAgentName == "" {
				interruptAgentName = "default-agent"
			}
			providerResponseID = optionalStringPtr(event.Interrupt.ProviderResponseID)
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeInterrupt,
				Interrupt: event.Interrupt,
				Timestamp: time.Now(),
			})

		case bichatservices.EventTypeUsage:
			if event.Usage != nil {
				finalUsage = event.Usage
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}

		case bichatservices.EventTypeDone:
			providerResponseID = optionalStringPtr(event.ProviderResponseID)
			if event.Usage != nil {
				finalUsage = event.Usage
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}
			generationMs = time.Since(startedAt).Milliseconds()
			// Send done chunk
			onChunk(bichatservices.StreamChunk{
				Type:         bichatservices.ChunkTypeDone,
				GenerationMs: generationMs,
				Timestamp:    time.Now(),
			})
		case bichatservices.EventTypeError:
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeError,
				Error:     event.Error,
				Timestamp: time.Now(),
			})
		case bichatservices.EventTypeCitation:
			// no-op in stream handler
		}
	}

	// Build assistant message options
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	savedToolCalls := orderedToolCalls(toolCalls, toolOrder)
	if len(savedToolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(savedToolCalls...))
	}
	if debugTrace := buildDebugTrace(savedToolCalls, finalUsage, generationMs); debugTrace != nil {
		assistantMsgOpts = append(assistantMsgOpts, types.WithDebugTrace(debugTrace))
	}

	// Attach QuestionData if interrupted
	if interrupt != nil {
		qd, err := buildQuestionData(interrupt.CheckpointID, interruptAgentName, interrupt.Questions)
		if err != nil {
			return serrors.E(op, err)
		}
		if qd != nil {
			assistantMsgOpts = append(assistantMsgOpts, types.WithQuestionData(qd))
		}
	}

	// Always save assistant message
	assistantMsg := types.AssistantMessage(assistantContent.String(), assistantMsgOpts...)
	err = s.chatRepo.SaveMessage(ctx, assistantMsg)
	if err != nil {
		return serrors.E(op, err)
	}

	// Update session
	session = session.
		UpdateLLMPreviousResponseID(providerResponseID).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
		return serrors.E(op, err)
	}

	// Generate title if not interrupted
	if interrupt == nil {
		s.maybeGenerateTitleAsync(ctx, req.SessionID)
	}

	return nil
}

func recordToolEvent(toolCalls map[string]types.ToolCall, toolOrder *[]string, tool *bichatservices.ToolEvent) {
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

func buildDebugTrace(toolCalls []types.ToolCall, usage *types.DebugUsage, generationMs int64) *types.DebugTrace {
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

	if usage == nil && generationMs <= 0 && len(debugTools) == 0 {
		return nil
	}

	return &types.DebugTrace{
		Usage:        usage,
		GenerationMs: generationMs,
		Tools:        debugTools,
	}
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
	interrupt          *bichatservices.Interrupt
	interruptAgentName string
	providerResponseID *string
	usage              *types.DebugUsage
	lastError          error
}

// consumeAgentEvents drains the generator and collects the result.
// This is used by non-streaming callers (SendMessage, ResumeWithAnswer, RejectPendingQuestion).
func consumeAgentEvents(ctx context.Context, gen types.Generator[bichatservices.Event]) (*agentResult, error) {
	var content strings.Builder
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)
	var interrupt *bichatservices.Interrupt
	var interruptAgentName string
	var providerResponseID *string
	var finalUsage *types.DebugUsage
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
		case bichatservices.EventTypeContent:
			content.WriteString(event.Content)
		case bichatservices.EventTypeToolStart, bichatservices.EventTypeToolEnd:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)
		case bichatservices.EventTypeInterrupt:
			if event.Interrupt != nil {
				interrupt = &bichatservices.Interrupt{
					CheckpointID: event.Interrupt.CheckpointID,
					Questions:    event.Interrupt.Questions,
				}
				interruptAgentName = event.Interrupt.AgentName
				if interruptAgentName == "" {
					interruptAgentName = "default-agent"
				}
				providerResponseID = optionalStringPtr(event.Interrupt.ProviderResponseID)
			}
		case bichatservices.EventTypeDone:
			providerResponseID = optionalStringPtr(event.ProviderResponseID)
			if event.Usage != nil {
				finalUsage = event.Usage
			}
		case bichatservices.EventTypeUsage:
			if event.Usage != nil {
				finalUsage = event.Usage
			}
		case bichatservices.EventTypeCitation:
			// no-op
		case bichatservices.EventTypeError:
			if event.Error != nil {
				lastError = event.Error
			} else if event.Content != "" {
				lastError = fmt.Errorf("%s", event.Content)
			}
		}
	}

	result := &agentResult{
		content:            content.String(),
		toolCalls:          orderedToolCalls(toolCalls, toolOrder),
		interrupt:          interrupt,
		interruptAgentName: interruptAgentName,
		providerResponseID: providerResponseID,
		usage:              finalUsage,
		lastError:          lastError,
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
) (types.Message, domain.Session, error) {
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(sessionID)}
	if len(result.toolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(result.toolCalls...))
	}
	if debugTrace := buildDebugTrace(result.toolCalls, result.usage, time.Since(startedAt).Milliseconds()); debugTrace != nil {
		assistantMsgOpts = append(assistantMsgOpts, types.WithDebugTrace(debugTrace))
	}

	if result.interrupt != nil {
		qd, err := buildQuestionData(result.interrupt.CheckpointID, result.interruptAgentName, result.interrupt.Questions)
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

	session = session.
		UpdateLLMPreviousResponseID(result.providerResponseID).
		UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
		return nil, nil, serrors.E(op, err)
	}

	return assistantMsg, session, nil
}

// GetSessionMessages retrieves all messages for a session.
func (s *chatServiceImpl) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	const op serrors.Op = "chatServiceImpl.GetSessionMessages"

	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return messages, nil
}

// ResumeWithAnswer resumes execution after user answers questions (HITL).
func (s *chatServiceImpl) ResumeWithAnswer(ctx context.Context, req bichatservices.ResumeRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.ResumeWithAnswer"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Get pending question message
	pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoPendingQuestion) {
			return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
		}
		return nil, serrors.E(op, err)
	}
	if pendingMsg == nil {
		return nil, serrors.E(op, serrors.NotFound, "no pending question for session")
	}

	// Validate question data before resuming (defer mutation until resume succeeds)
	qd := pendingMsg.QuestionData()
	if qd == nil {
		return nil, serrors.E(op, serrors.KindValidation, "pending message has no question data")
	}

	answeredQD, err := qd.Answer(req.Answers)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert map[string]string to map[string]types.Answer
	answersMap := make(map[string]types.Answer, len(req.Answers))
	for questionID, answerValue := range req.Answers {
		answersMap[questionID] = types.NewAnswer(answerValue)
	}

	// Resume agent execution with answers
	gen, err := s.agentService.ResumeWithAnswer(ctx, req.SessionID, req.CheckpointID, answersMap)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(ctx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Mark question as answered only after resume succeeds — prevents irreversible
	// state drift if the provider returns a transient error, timeout, or bad checkpoint.
	if err := s.chatRepo.UpdateMessageQuestionData(ctx, pendingMsg.ID(), answeredQD); err != nil {
		return nil, serrors.E(op, err)
	}

	assistantMsg, session, err := s.saveAgentResult(ctx, op, session, req.SessionID, result, startedAt)
	if err != nil {
		return nil, err
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      nil,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// RejectPendingQuestion rejects a pending HITL question and resumes execution.
// This marks the question data as rejected and tells the agent the user dismissed it.
func (s *chatServiceImpl) RejectPendingQuestion(ctx context.Context, sessionID uuid.UUID) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.RejectPendingQuestion"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Get pending question message
	pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoPendingQuestion) {
			return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
		}
		return nil, serrors.E(op, err)
	}
	if pendingMsg == nil {
		return nil, serrors.E(op, serrors.NotFound, "no pending question for session")
	}

	// Validate question data before resuming (defer mutation until resume succeeds)
	qd := pendingMsg.QuestionData()
	if qd == nil {
		return nil, serrors.E(op, serrors.KindValidation, "pending message has no question data")
	}

	rejectedQD, err := qd.Reject()
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Resume agent with rejection signal
	rejectionAnswers := map[string]types.Answer{
		"__rejected__": types.NewAnswer("User dismissed the questions"),
	}

	gen, err := s.agentService.ResumeWithAnswer(ctx, sessionID, qd.CheckpointID, rejectionAnswers)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(ctx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Mark question as rejected only after resume succeeds — prevents irreversible
	// state drift if the provider returns a transient error, timeout, or bad checkpoint.
	if err := s.chatRepo.UpdateMessageQuestionData(ctx, pendingMsg.ID(), rejectedQD); err != nil {
		return nil, serrors.E(op, err)
	}

	assistantMsg, session, err := s.saveAgentResult(ctx, op, session, sessionID, result, startedAt)
	if err != nil {
		return nil, err
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      nil,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// GenerateSessionTitle generates a title for a session based on first message.
func (s *chatServiceImpl) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.GenerateSessionTitle"

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Get first user message
	opts := domain.ListOptions{Limit: 1, Offset: 0}
	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, opts)
	if err != nil {
		return serrors.E(op, err)
	}

	if len(messages) == 0 {
		return serrors.E(op, serrors.KindValidation, "no messages found for session")
	}

	firstMessage := messages[0]
	if firstMessage.Role() != types.RoleUser {
		return serrors.E(op, serrors.KindValidation, "first message is not a user message")
	}

	// Generate title using LLM
	prompt := fmt.Sprintf("Generate a short, concise title (max 5 words) for a chat conversation that starts with this user message: \"%s\"\n\nProvide only the title, no quotes or extra text.", firstMessage.Content())

	titleMsg := types.SystemMessage(prompt)
	resp, err := s.model.Generate(ctx, agents.Request{
		Messages: []types.Message{titleMsg},
	}, agents.WithMaxTokens(20))
	if err != nil {
		return serrors.E(op, err)
	}

	title := strings.TrimSpace(resp.Message.Content())
	title = strings.Trim(title, "\"'")
	updated := session.UpdateTitle(title)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) generateCompactionSummary(ctx context.Context, messages []types.Message) (string, error) {
	if len(messages) == 0 {
		return "History compaction complete. No previous messages were available to summarize.", nil
	}

	var transcript strings.Builder
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		switch msg.Role() {
		case types.RoleUser:
			transcript.WriteString("User: ")
		case types.RoleAssistant:
			transcript.WriteString("Assistant: ")
		case types.RoleSystem, types.RoleTool:
			continue
		default:
			continue
		}
		transcript.WriteString(strings.TrimSpace(msg.Content()))
		transcript.WriteString("\n")
	}

	if transcript.Len() == 0 {
		return "History compaction complete. No user/assistant turns were available to summarize.", nil
	}

	prompt := fmt.Sprintf(
		`Summarize this chat history into a compact state snapshot.
Return markdown with these sections:
1. Conversation Summary
2. Key Facts and Decisions
3. Open Questions or Follow-ups
Keep it concise, factual, and preserve important numeric values.

CHAT TRANSCRIPT:
%s`,
		transcript.String(),
	)

	if s.model == nil {
		return "History compaction complete. A concise summary could not be generated because no model is configured.", nil
	}

	resp, err := s.model.Generate(ctx, agents.Request{
		Messages: []types.Message{types.SystemMessage(prompt)},
	}, agents.WithMaxTokens(700))
	if err != nil {
		return "History compaction complete. Summary generation failed, original history was compacted.", err
	}

	summary := strings.TrimSpace(resp.Message.Content())
	if summary == "" {
		return "History compaction complete. The model returned an empty summary.", nil
	}

	return summary, nil
}

// maybeGenerateTitleAsync triggers async title generation if this is the first message in the session.
// Runs in background goroutine with timeout to avoid blocking the response.
func (s *chatServiceImpl) maybeGenerateTitleAsync(ctx context.Context, sessionID uuid.UUID) {
	// Skip if no title service configured
	if s.titleService == nil {
		return
	}

	// Launch async title generation (don't block response)
	go func() {
		// Create detached context and copy required request-scoped values.
		// Background title generation needs tenant/pool context, but must not reuse request cancellation.
		titleCtx := buildTitleGenerationContext(ctx)

		// Set timeout (15s allows for 3 retries)
		titleCtx, cancel := context.WithTimeout(titleCtx, 15*time.Second)
		defer cancel()

		// GenerateSessionTitle has built-in logic to skip if title already exists
		if err := s.titleService.GenerateSessionTitle(titleCtx, sessionID); err != nil {
			// Just log - don't fail the request
			// If logger is available in context, use it
			// For now, silently ignore errors
			_ = err
		}
	}()
}

func buildTitleGenerationContext(ctx context.Context) context.Context {
	titleCtx := context.Background()

	if tenantID, err := composables.UseTenantID(ctx); err == nil {
		titleCtx = composables.WithTenantID(titleCtx, tenantID)
	}
	if pool, err := composables.UsePool(ctx); err == nil {
		titleCtx = composables.WithPool(titleCtx, pool)
	}
	if user, err := composables.UseUser(ctx); err == nil {
		titleCtx = composables.WithUser(titleCtx, user)
	}

	return titleCtx
}

// buildQuestionData converts service-level interrupt questions to QuestionData
func buildQuestionData(checkpointID, agentName string, questions []bichatservices.Question) (*types.QuestionData, error) {
	items := make([]types.QuestionDataItem, len(questions))
	for i, q := range questions {
		opts := make([]types.QuestionDataOption, len(q.Options))
		for j, o := range q.Options {
			opts[j] = types.QuestionDataOption{ID: o.ID, Label: o.Label}
		}
		qType := "single_choice"
		if q.Type == bichatservices.QuestionTypeMultipleChoice {
			qType = "multiple_choice"
		}
		items[i] = types.QuestionDataItem{
			ID:      q.ID,
			Text:    q.Text,
			Type:    qType,
			Options: opts,
		}
	}
	qd, err := types.NewQuestionData(checkpointID, agentName, items)
	if err != nil {
		return nil, err
	}
	return qd, nil
}
