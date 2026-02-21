package services

import (
	"context"

	"github.com/google/uuid"
)

// TitleService generates and manages session titles.
type TitleService interface {
	// GenerateSessionTitle ensures a title exists without overwriting existing non-empty titles.
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error

	// RegenerateSessionTitle always replaces the existing session title.
	RegenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}
