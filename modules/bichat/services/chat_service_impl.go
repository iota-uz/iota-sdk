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
	return s.chatRepo.GetSession(ctx, sessionID)
}

// ListUserSessions lists all sessions for a user.
func (s *chatServiceImpl) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ListUserSessions"
	return s.chatRepo.ListUserSessions(ctx, userID, opts)
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

// SendMessage sends a message to a session and processes it with the agent.
func (s *chatServiceImpl) SendMessage(ctx context.Context, req bichatservices.SendMessageRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.SendMessage"

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Create user message
	userMsg := &types.Message{
		ID:        uuid.New(),
		SessionID: req.SessionID,
		Role:      types.RoleUser,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	// Save user message
	err = s.chatRepo.SaveMessage(ctx, userMsg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert attachments to domain attachments
	domainAttachments := make([]domain.Attachment, len(req.Attachments))
	for i, att := range req.Attachments {
		domainAttachments[i] = domain.Attachment{
			ID:        att.ID,
			MessageID: userMsg.ID,
			FileName:  att.FileName,
			MimeType:  att.MimeType,
			SizeBytes: att.SizeBytes,
			FilePath:  att.FilePath,
			CreatedAt: att.CreatedAt,
		}
	}

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(ctx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	var assistantContent strings.Builder
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
	assistantMsg := &types.Message{
		ID:        uuid.New(),
		SessionID: req.SessionID,
		Role:      types.RoleAssistant,
		Content:   assistantContent.String(),
		CreatedAt: time.Now(),
	}

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

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Create user message
	userMsg := &types.Message{
		ID:        uuid.New(),
		SessionID: req.SessionID,
		Role:      types.RoleUser,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	// Save user message
	err = s.chatRepo.SaveMessage(ctx, userMsg)
	if err != nil {
		return serrors.E(op, err)
	}

	// Convert attachments to domain attachments
	domainAttachments := make([]domain.Attachment, len(req.Attachments))
	for i, att := range req.Attachments {
		domainAttachments[i] = domain.Attachment{
			ID:        att.ID,
			MessageID: userMsg.ID,
			FileName:  att.FileName,
			MimeType:  att.MimeType,
			SizeBytes: att.SizeBytes,
			FilePath:  att.FilePath,
			CreatedAt: att.CreatedAt,
		}
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
			// Stream content chunk
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeContent,
				Content:   event.Content,
				Timestamp: time.Now(),
			})

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

		case bichatservices.EventTypeDone:
			// Send usage chunk
			if event.Usage != nil {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}
			// Send done chunk
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeDone,
				Timestamp: time.Now(),
			})
		}
	}

	// Save assistant message if not interrupted
	if !interrupted {
		assistantMsg := &types.Message{
			ID:        uuid.New(),
			SessionID: req.SessionID,
			Role:      types.RoleAssistant,
			Content:   assistantContent.String(),
			CreatedAt: time.Now(),
		}

		err = s.chatRepo.SaveMessage(ctx, assistantMsg)
		if err != nil {
			return serrors.E(op, err)
		}

		session = session.UpdateUpdatedAt(time.Now())
		if err := s.chatRepo.UpdateSession(ctx, session); err != nil {
			return nil, serrors.E(op, err)
		}
		s.maybeGenerateTitleAsync(ctx, req.SessionID)
	}

	return nil
}

// GetSessionMessages retrieves all messages for a session.
func (s *chatServiceImpl) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*types.Message, error) {
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
		}
	}

	// Create and save assistant message
	assistantMsg := &types.Message{
		ID:        uuid.New(),
		SessionID: req.SessionID,
		Role:      types.RoleAssistant,
		Content:   assistantContent.String(),
		CreatedAt: time.Now(),
	}

	err = s.chatRepo.SaveMessage(ctx, assistantMsg)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Clear pending question agent
	session.PendingQuestionAgent = nil
	session.UpdatedAt = time.Now()
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
	if firstMessage.Role != types.RoleUser {
		return serrors.E(op, serrors.KindValidation, "first message is not a user message")
	}

	// Generate title using LLM
	prompt := fmt.Sprintf("Generate a short, concise title (max 5 words) for a chat conversation that starts with this user message: \"%s\"\n\nProvide only the title, no quotes or extra text.", firstMessage.Content)

	titleMsg := types.SystemMessage(prompt)
	resp, err := s.model.Generate(ctx, agents.Request{
		Messages: []types.Message{*titleMsg},
	}, agents.WithMaxTokens(20))
	if err != nil {
		return serrors.E(op, err)
	}

	title := strings.TrimSpace(resp.Message.Content)
	title = strings.Trim(title, "\"'")
	updated := session.UpdateTitle(title)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// stringPtr returns a pointer to a string.
func stringPtr(s string) *string {
	return &s
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
		// Create new context for background operation (detached from request context)
		titleCtx := context.Background()

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
