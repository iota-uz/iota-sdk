// Package spotlight defines shared contracts for Spotlight AI integrations.
package spotlight

import (
	"context"
	"reflect"
	"time"

	"github.com/google/uuid"
)

type AISearchActor struct {
	TenantID    uuid.UUID
	UserID      string
	Roles       []string
	Permissions []string
	Language    string
}

type AISearchCreateRequest struct {
	Query string
	Actor AISearchActor
}

type AISearchMessageRequest struct {
	SessionID string
	Message   string
	Actor     AISearchActor
}

type AISearchSessionAccess struct {
	TenantID uuid.UUID
	UserID   string
}

type AISearchMessageRole string

const (
	AISearchMessageRoleUser      AISearchMessageRole = "user"
	AISearchMessageRoleAssistant AISearchMessageRole = "assistant"
)

type AISearchMessage struct {
	ID        string              `json:"id"`
	Role      AISearchMessageRole `json:"role"`
	Content   string              `json:"content"`
	Pending   bool                `json:"pending,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
}

type AISearchLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type AISearchCandidate struct {
	ID             string         `json:"id"`
	EntityType     string         `json:"entity_type"`
	Title          string         `json:"title"`
	Subtitle       string         `json:"subtitle,omitempty"`
	Evidence       []string       `json:"evidence,omitempty"`
	URL            string         `json:"url,omitempty"`
	Source         string         `json:"source,omitempty"`
	RelatedLinks   []AISearchLink `json:"related_links,omitempty"`
	ConfidenceNote string         `json:"confidence_note,omitempty"`
}

type AISearchToolState struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Summary     string    `json:"summary,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type AISearchSnapshot struct {
	SessionID  string              `json:"session_id"`
	RunID      string              `json:"run_id"`
	Query      string              `json:"query"`
	Messages   []AISearchMessage   `json:"messages"`
	Candidates []AISearchCandidate `json:"candidates"`
	Tools      []AISearchToolState `json:"tools"`
	Loading    bool                `json:"loading"`
	Completed  bool                `json:"completed"`
	Error      string              `json:"error,omitempty"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

type AISearchService interface {
	CreateSession(ctx context.Context, req AISearchCreateRequest) (AISearchSnapshot, error)
	SendMessage(ctx context.Context, req AISearchMessageRequest) (AISearchSnapshot, error)
	Subscribe(ctx context.Context, sessionID, runID string, access AISearchSessionAccess) (<-chan AISearchSnapshot, error)
	Cancel(sessionID, runID string, access AISearchSessionAccess)
}

type AISearchServiceHolder struct {
	Service AISearchService
}

func ResolveAISearchService(services map[reflect.Type]interface{}) AISearchService {
	if len(services) == 0 {
		return nil
	}
	raw, ok := services[reflect.TypeOf(AISearchServiceHolder{})]
	if !ok {
		return nil
	}
	holder, ok := raw.(*AISearchServiceHolder)
	if !ok || holder == nil {
		return nil
	}
	return holder.Service
}
