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
func (s *chatServiceImpl) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.CreateSession"

	// Create session entity
	session := &domain.Session{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Title:    title,
		Status:   domain.SessionStatusActive,
		Pinned:   false,
	}

	// Save to database
	err := s.chatRepo.CreateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (s *chatServiceImpl) GetSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.GetSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
}

// ListUserSessions lists all sessions for a user.
func (s *chatServiceImpl) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ListUserSessions"

	sessions, err := s.chatRepo.ListUserSessions(ctx, userID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return sessions, nil
}

// ArchiveSession archives a session.
func (s *chatServiceImpl) ArchiveSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ArchiveSession"

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Update status
	session.Status = domain.SessionStatusArchived

	// Save changes
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
}

// PinSession pins a session.
func (s *chatServiceImpl) PinSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.PinSession"

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Update pinned status
	session.Pinned = true

	// Save changes
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
}

// UnpinSession unpins a session.
func (s *chatServiceImpl) UnpinSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UnpinSession"

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Update pinned status
	session.Pinned = false

	// Save changes
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
}

// UpdateSessionTitle updates the title of a session.
func (s *chatServiceImpl) UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (*domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UpdateSessionTitle"

	// Validate title
	if title == "" {
		return nil, serrors.E(op, serrors.KindValidation, "title cannot be empty")
	}

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Update title
	session.Title = title
	session.UpdatedAt = time.Now()

	// Save changes
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return session, nil
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
				// Update session with pending question agent name
				if event.Interrupt.AgentName != "" {
					session.PendingQuestionAgent = &event.Interrupt.AgentName
				}
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

	// Update session
	session.UpdatedAt = time.Now()
	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Trigger async title generation for first message
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
			// Update session with pending question agent name
			if event.Interrupt != nil && event.Interrupt.AgentName != "" {
				session.PendingQuestionAgent = &event.Interrupt.AgentName
			}
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

		// Update session
		session.UpdatedAt = time.Now()
		err = s.chatRepo.UpdateSession(ctx, session)
		if err != nil {
			return serrors.E(op, err)
		}

		// Trigger async title generation for first message
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

	// Resume agent execution with answers (already in canonical format)
	gen, err := s.agentService.ResumeWithAnswer(ctx, req.SessionID, req.CheckpointID, req.Answers)
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

	// Update session title
	title := strings.TrimSpace(resp.Message.Content)
	title = strings.Trim(title, "\"'") // Remove quotes if present
	session.Title = title

	err = s.chatRepo.UpdateSession(ctx, session)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
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
