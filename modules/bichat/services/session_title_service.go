package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	maxTitleLength            = 60
	minTitleLength            = 3
	titleGenerationMaxRetries = 3
	titleGenDefaultTokens     = 20
	titleGenRetryTokens       = 40
	titleGenFinalRetryTokens  = 80
	titleGenTimeout           = 10 * time.Second
	untitledChatTitle         = "Untitled Chat"
)

// SessionTitleMode controls generation behavior.
type SessionTitleMode string

const (
	SessionTitleModeAuto       SessionTitleMode = "auto"
	SessionTitleModeRegenerate SessionTitleMode = "regenerate"
)

type sessionTitleService struct {
	model    agents.Model
	chatRepo domain.ChatRepository
	eventBus hooks.EventBus
}

// NewSessionTitleService creates a session title service.
func NewSessionTitleService(model agents.Model, chatRepo domain.ChatRepository, eventBus hooks.EventBus) (*sessionTitleService, error) {
	const op serrors.Op = "NewSessionTitleService"

	if model == nil {
		return nil, serrors.E(op, "model is required")
	}
	if chatRepo == nil {
		return nil, serrors.E(op, "chat repository is required")
	}

	return &sessionTitleService{
		model:    model,
		chatRepo: chatRepo,
		eventBus: eventBus,
	}, nil
}

// GenerateSessionTitle ensures a title exists without overwriting existing non-empty titles.
func (s *sessionTitleService) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	return s.generate(ctx, sessionID, SessionTitleModeAuto)
}

// RegenerateSessionTitle always replaces the session title.
func (s *sessionTitleService) RegenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	return s.generate(ctx, sessionID, SessionTitleModeRegenerate)
}

func (s *sessionTitleService) generate(ctx context.Context, sessionID uuid.UUID, mode SessionTitleMode) error {
	const op serrors.Op = "SessionTitleService.generate"

	ctx, cancel := context.WithTimeout(ctx, titleGenTimeout)
	defer cancel()

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err, "failed to get session")
	}
	if mode == SessionTitleModeAuto && strings.TrimSpace(session.Title()) != "" {
		return nil
	}

	userMsg, assistantMsg, err := s.firstExchange(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err, "failed to load first exchange")
	}

	title := untitledChatTitle
	if strings.TrimSpace(userMsg) != "" {
		candidate, candidateErr := s.generateTitleWithRetry(ctx, userMsg, assistantMsg)
		if candidateErr != nil {
			candidate = extractFallbackSessionTitle(userMsg)
		}
		title = s.finalizeTitle(candidate, userMsg)
	}

	updated, err := s.persistTitle(ctx, session.ID(), session.TenantID(), title, mode)
	if err != nil {
		return serrors.E(op, err, "failed to persist generated title")
	}
	if !updated {
		return nil
	}

	if s.eventBus != nil {
		evt := events.NewSessionTitleUpdatedEvent(session.ID(), session.TenantID(), title)
		_ = s.eventBus.Publish(ctx, evt)
	}

	return nil
}

func (s *sessionTitleService) firstExchange(ctx context.Context, sessionID uuid.UUID) (string, string, error) {
	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, domain.ListOptions{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		return "", "", err
	}

	var userMsg string
	var assistantMsg string
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		if msg.Role() == types.RoleUser && userMsg == "" {
			userMsg = msg.Content()
		}
		if msg.Role() == types.RoleAssistant && assistantMsg == "" {
			assistantMsg = msg.Content()
		}
		if userMsg != "" && assistantMsg != "" {
			break
		}
	}

	return userMsg, assistantMsg, nil
}

func (s *sessionTitleService) persistTitle(ctx context.Context, sessionID uuid.UUID, tenantID uuid.UUID, title string, mode SessionTitleMode) (bool, error) {
	cleaned := s.finalizeTitle(title, "")
	if mode == SessionTitleModeRegenerate {
		if err := s.chatRepo.UpdateSessionTitle(ctx, sessionID, cleaned); err != nil {
			return false, err
		}
		return true, nil
	}

	updated, err := s.chatRepo.UpdateSessionTitleIfEmpty(ctx, sessionID, cleaned)
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (s *sessionTitleService) generateTitleWithRetry(ctx context.Context, userMsg, assistantMsg string) (string, error) {
	const op serrors.Op = "SessionTitleService.generateTitleWithRetry"

	tokenLimits := []int{titleGenDefaultTokens, titleGenRetryTokens, titleGenFinalRetryTokens}
	for _, maxTokens := range tokenLimits {
		title, err := s.generateTitleWithLLM(ctx, userMsg, assistantMsg, maxTokens)
		if err == nil && isValidSessionTitle(cleanSessionTitle(title)) {
			return title, nil
		}
	}

	return "", serrors.E(op, "all retry attempts failed")
}

func (s *sessionTitleService) generateTitleWithLLM(ctx context.Context, userMsg, assistantMsg string, maxTokens int) (string, error) {
	const op serrors.Op = "SessionTitleService.generateTitleWithLLM"

	prompt, err := renderSessionTitlePrompt(userMsg, assistantMsg)
	if err != nil {
		return "", serrors.E(op, err)
	}

	resp, err := s.model.Generate(ctx, agents.Request{
		Messages: []types.Message{types.UserMessage(prompt)},
	}, agents.WithMaxTokens(maxTokens))
	if err != nil {
		return "", serrors.E(op, err, "LLM generation failed")
	}

	return strings.TrimSpace(resp.Message.Content()), nil
}

func (s *sessionTitleService) finalizeTitle(generated, userMessage string) string {
	title := cleanSessionTitle(generated)
	if title == "" && strings.TrimSpace(userMessage) != "" {
		title = cleanSessionTitle(extractFallbackSessionTitle(userMessage))
	}
	if title == "" {
		title = untitledChatTitle
	}
	return title
}

var _ TitleGenerationService = (*sessionTitleService)(nil)
var _ SessionTitleRegenerationService = (*sessionTitleService)(nil)
