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

	updated := session.UpdatePendingQuestionAgent(nil).UpdateUpdatedAt(time.Now())
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

	userMsg := types.UserMessage("/compact", types.WithSessionID(sessionID))
	if err := s.chatRepo.SaveMessage(ctx, userMsg); err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	assistantMsg := types.AssistantMessage(summary, types.WithSessionID(sessionID))
	if err := s.chatRepo.SaveMessage(ctx, assistantMsg); err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	updated := session.UpdatePendingQuestionAgent(nil).UpdateUpdatedAt(time.Now())
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

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
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

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(ctx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	var assistantContent strings.Builder
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)
	var interrupt *bichatservices.Interrupt

	for {
		event, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			return nil, serrors.E(op, err)
		}

		switch event.Type {
		case bichatservices.EventTypeContent:
			assistantContent.WriteString(event.Content)
		case bichatservices.EventTypeToolStart, bichatservices.EventTypeToolEnd:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)

		case bichatservices.EventTypeInterrupt:
			// Handle HITL interrupt
			if event.Interrupt != nil {
				interrupt = &bichatservices.Interrupt{
					CheckpointID: event.Interrupt.CheckpointID,
					Questions:    event.Interrupt.Questions,
				}
				// Update session with pending question agent
				agentName := event.Interrupt.AgentName
				if agentName == "" {
					agentName = "default-agent"
				}
				session = session.UpdatePendingQuestionAgent(&agentName)
				if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
					return nil, serrors.E(op, err)
				}
			}
		case bichatservices.EventTypeCitation, bichatservices.EventTypeUsage,
			bichatservices.EventTypeDone, bichatservices.EventTypeError:
			// no-op in this handler
		}
	}

	// If interrupted, return without saving assistant message
	if interrupt != nil {
		return &bichatservices.SendMessageResponse{
			UserMessage:      userMsg,
			AssistantMessage: nil,
			Session:          session,
			Interrupt:        interrupt,
		}, nil
	}

	// Create and save assistant message
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	if savedToolCalls := orderedToolCalls(toolCalls, toolOrder); len(savedToolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(savedToolCalls...))
	}
	assistantMsg := types.AssistantMessage(assistantContent.String(), assistantMsgOpts...)

	err = s.chatRepo.SaveMessage(ctx, assistantMsg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	session = session.UpdateUpdatedAt(time.Now())
	if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
		return nil, serrors.E(op, err)
	}

	s.maybeGenerateTitleAsync(ctx, req.SessionID)

	return &bichatservices.SendMessageResponse{
		UserMessage:      userMsg,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        nil,
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

	// Create user message
	userMsg := types.UserMessage(req.Content, types.WithSessionID(req.SessionID))

	// Save user message
	err = s.chatRepo.SaveMessage(ctx, userMsg)
	if err != nil {
		return serrors.E(op, err)
	}

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

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(ctx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return serrors.E(op, err)
	}
	defer gen.Close()

	// Stream agent response
	var assistantContent strings.Builder
	var interrupted bool
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)

	for {
		event, err := gen.Next(ctx)
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
			interrupted = true
			agentName := event.Interrupt.AgentName
			if agentName == "" {
				agentName = "default-agent"
			}
			session = session.UpdatePendingQuestionAgent(&agentName)
			if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
				return serrors.E(op, err)
			}

		case bichatservices.EventTypeUsage:
			if event.Usage != nil {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}

		case bichatservices.EventTypeDone:
			if event.Usage != nil {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}
			// Send done chunk
			onChunk(bichatservices.StreamChunk{
				Type:         bichatservices.ChunkTypeDone,
				GenerationMs: time.Since(startedAt).Milliseconds(),
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

	// Save assistant message if not interrupted
	if !interrupted {
		assistantMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
		if savedToolCalls := orderedToolCalls(toolCalls, toolOrder); len(savedToolCalls) > 0 {
			assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(savedToolCalls...))
		}
		assistantMsg := types.AssistantMessage(assistantContent.String(), assistantMsgOpts...)

		err = s.chatRepo.SaveMessage(ctx, assistantMsg)
		if err != nil {
			return serrors.E(op, err)
		}

		session = session.UpdateUpdatedAt(time.Now())
		if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
			return serrors.E(op, err)
		}
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

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
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
	var assistantContent strings.Builder
	toolCalls := make(map[string]types.ToolCall)
	toolOrder := make([]string, 0)

	for {
		event, err := gen.Next(ctx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			return nil, serrors.E(op, err)
		}

		switch event.Type {
		case bichatservices.EventTypeContent:
			assistantContent.WriteString(event.Content)
		case bichatservices.EventTypeToolStart, bichatservices.EventTypeToolEnd:
			recordToolEvent(toolCalls, &toolOrder, event.Tool)
		case bichatservices.EventTypeCitation, bichatservices.EventTypeUsage,
			bichatservices.EventTypeInterrupt, bichatservices.EventTypeDone,
			bichatservices.EventTypeError:
			// no-op for resume
		}
	}

	// Create and save assistant message
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	if savedToolCalls := orderedToolCalls(toolCalls, toolOrder); len(savedToolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(savedToolCalls...))
	}
	assistantMsg := types.AssistantMessage(assistantContent.String(), assistantMsgOpts...)

	err = s.chatRepo.SaveMessage(ctx, assistantMsg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Clear pending question agent
	session = session.UpdatePendingQuestionAgent(nil)
	session = session.UpdateUpdatedAt(time.Now())
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      nil, // No new user message for resume
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        nil,
	}, nil
}

// CancelPendingQuestion cancels a pending HITL question without resuming execution.
// This clears the pending question state from the session.
func (s *chatServiceImpl) CancelPendingQuestion(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.CancelPendingQuestion"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if !session.HasPendingQuestion() {
		return nil, serrors.E(op, serrors.KindValidation, "no pending question to cancel")
	}
	updated := session.UpdatePendingQuestionAgent(nil)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
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
		return "History compaction complete. Summary generation failed, original history was compacted.", nil
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
