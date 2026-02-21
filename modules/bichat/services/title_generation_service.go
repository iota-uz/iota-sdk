package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
)

// TitleGenerationService ensures a session has a generated title.
// This method is idempotent and will not overwrite non-empty titles.
type TitleGenerationService interface {
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

// SessionTitleRegenerationService regenerates an existing title explicitly.
type SessionTitleRegenerationService interface {
	RegenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

// NewTitleGenerationService creates a title generation service.
func NewTitleGenerationService(model agents.Model, chatRepo domain.ChatRepository, eventBus hooks.EventBus) (TitleGenerationService, error) {
	return NewSessionTitleService(model, chatRepo, eventBus)
}
