package services

import (
	"context"
	"fmt"
	"regexp"
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
	maxTitleLength    = 60
	minTitleLength    = 3
	maxRetries        = 3
	defaultTokenLimit = 20
	retryTokenLimit   = 40
	finalRetryTokens  = 80
	titleGenTimeout   = 10 * time.Second
)

// TitleGenerationService generates session titles from conversation
type TitleGenerationService interface {
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

type titleGenerationService struct {
	model    agents.Model
	chatRepo domain.ChatRepository
	eventBus hooks.EventBus
}

// NewTitleGenerationService creates a new title generation service.
// The eventBus parameter is optional (can be nil) — when provided,
// a SessionTitleUpdatedEvent is published after successful title generation.
func NewTitleGenerationService(model agents.Model, chatRepo domain.ChatRepository, eventBus hooks.EventBus) (TitleGenerationService, error) {
	const op serrors.Op = "NewTitleGenerationService"

	if model == nil {
		return nil, serrors.E(op, "model is required")
	}
	if chatRepo == nil {
		return nil, serrors.E(op, "chat repository is required")
	}

	return &titleGenerationService{
		model:    model,
		chatRepo: chatRepo,
		eventBus: eventBus,
	}, nil
}

// GenerateSessionTitle generates a title for the session from the first user/assistant exchange.
// It uses a multi-layer approach:
// 1. Attempt LLM generation with retry logic
// 2. Fall back to smart extraction from user message
// 3. Keep "New Chat" as final fallback
//
// Skips generation if:
// - Session already has a non-empty title
// - Session has fewer than 2 messages (no conversation yet)
func (s *titleGenerationService) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "TitleGenerationService.GenerateSessionTitle"

	// Set timeout for title generation
	ctx, cancel := context.WithTimeout(ctx, titleGenTimeout)
	defer cancel()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err, "failed to get session")
	}

	if session.Title() != "" {
		return nil
	}

	// Get messages (first 2 are enough)
	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, domain.ListOptions{
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		return serrors.E(op, err, "failed to get messages")
	}

	// Need at least 1 message (user message) to generate title
	if len(messages) == 0 {
		return nil // No messages yet, skip
	}

	// Extract first user message
	var userMsg, assistantMsg string
	for _, msg := range messages {
		if msg.Role() == types.RoleUser && userMsg == "" {
			userMsg = msg.Content()
		}
		if msg.Role() == types.RoleAssistant && assistantMsg == "" {
			assistantMsg = msg.Content()
		}
	}

	if userMsg == "" {
		return nil // No user message, skip
	}

	// Try LLM generation with retry
	title, err := s.generateTitleWithRetry(ctx, userMsg, assistantMsg)
	if err != nil {
		// Fall back to extraction from user message
		title = s.extractTitleFromMessage(userMsg)
	}

	// Validate and clean title
	title = s.cleanTitle(title)
	if title == "" {
		return nil // Give up, keep empty title
	}

	updated := session.UpdateTitle(title)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return serrors.E(op, err, "failed to update session title")
	}

	// Publish title event so observability providers can update trace names.
	if s.eventBus != nil {
		evt := events.NewSessionTitleUpdatedEvent(sessionID, session.TenantID(), title)
		_ = s.eventBus.Publish(ctx, evt)
	}

	return nil
}

// generateTitleWithRetry attempts to generate title with exponential retry logic
func (s *titleGenerationService) generateTitleWithRetry(ctx context.Context, userMsg, assistantMsg string) (string, error) {
	const op serrors.Op = "TitleGenerationService.generateTitleWithRetry"

	tokenLimits := []int{defaultTokenLimit, retryTokenLimit, finalRetryTokens}

	for attempt := 0; attempt < maxRetries; attempt++ {
		maxTokens := tokenLimits[attempt]

		title, err := s.generateTitleWithLLM(ctx, userMsg, assistantMsg, maxTokens)
		if err == nil && s.isValidTitle(title) {
			return title, nil
		}

		// Log retry (if logger available)
		// Continue to next attempt
	}

	return "", serrors.E(op, "all retry attempts failed")
}

// generateTitleWithLLM calls the LLM to generate a title
func (s *titleGenerationService) generateTitleWithLLM(ctx context.Context, userMsg, assistantMsg string, maxTokens int) (string, error) {
	const op serrors.Op = "TitleGenerationService.generateTitleWithLLM"

	// Build prompt
	prompt := s.buildPrompt(userMsg, assistantMsg)

	// Create request
	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage(prompt),
		},
	}

	// Generate with LLM
	resp, err := s.model.Generate(ctx, req, agents.WithMaxTokens(maxTokens))
	if err != nil {
		return "", serrors.E(op, err, "LLM generation failed")
	}

	title := strings.TrimSpace(resp.Message.Content())
	return title, nil
}

// buildPrompt creates the prompt for title generation
func (s *titleGenerationService) buildPrompt(userMsg, assistantMsg string) string {
	// Simple template substitution
	template := `Generate a concise, descriptive 5-7 word title for this conversation based on the user's first message and the assistant's response. The title should capture the main topic or intent clearly and professionally.

Return ONLY the title, no quotes, no additional text, and no extra formatting.

User's first message:
%s

Assistant's response:
%s

Title:`

	return fmt.Sprintf(template, userMsg, assistantMsg)
}

// isValidTitle checks if the generated title is valid
func (s *titleGenerationService) isValidTitle(title string) bool {
	if title == "" {
		return false
	}

	titleLen := len(title)
	if titleLen < minTitleLength || titleLen > maxTitleLength {
		return false
	}

	// Check for common invalid patterns
	lower := strings.ToLower(title)
	invalidPrefixes := []string{"here is", "here's", "the title", "title:", "as an ai"}
	for _, prefix := range invalidPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}

	return true
}

// extractTitleFromMessage creates a title by extracting keywords from the user message
func (s *titleGenerationService) extractTitleFromMessage(msg string) string {
	// Clean and truncate
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	// Remove common prefixes
	lower := strings.ToLower(msg)
	prefixes := []string{
		"can you ", "could you ", "please ", "i need ", "i want ",
		"show me ", "tell me ", "give me ", "help me ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			msg = msg[len(prefix):]
			break
		}
	}

	// Capitalize first letter
	if len(msg) > 0 {
		msg = strings.ToUpper(string(msg[0])) + msg[1:]
	}

	// Truncate to reasonable length
	if len(msg) > maxTitleLength {
		msg = msg[:maxTitleLength-3] + "..."
	}

	return msg
}

// cleanTitle cleans and validates the final title
func (s *titleGenerationService) cleanTitle(title string) string {
	// Trim whitespace and quotes
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`")

	// Remove markdown formatting
	title = regexp.MustCompile(`[*_~\[\]()]`).ReplaceAllString(title, "")

	// Collapse multiple spaces
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	// Truncate if too long
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength-3] + "..."
	}

	return title
}
