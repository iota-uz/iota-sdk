// Package spotlight defines shared contracts for Spotlight AI integrations.
package spotlight

import (
	"context"
	"reflect"
	"strings"
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
	Kind     string `json:"kind,omitempty"`
	Label    string `json:"label,omitempty"`
	LabelKey string `json:"label_key,omitempty"`
	URL      string `json:"url"`
}

type AISearchEvidenceItem struct {
	Label    string `json:"label,omitempty"`
	LabelKey string `json:"label_key,omitempty"`
	Value    string `json:"value"`
}

type AISearchCandidate struct {
	ID                 string                 `json:"id"`
	EntityType         string                 `json:"entity_type"`
	EntityLabelKey     string                 `json:"entity_label_key,omitempty"`
	Title              string                 `json:"title,omitempty"`
	TitleLabelKey      string                 `json:"title_label_key,omitempty"`
	TitleValue         string                 `json:"title_value,omitempty"`
	Subtitle           string                 `json:"subtitle,omitempty"`
	EvidenceItems      []AISearchEvidenceItem `json:"evidence_items,omitempty"`
	URL                string                 `json:"url,omitempty"`
	Source             string                 `json:"source,omitempty"`
	RelatedLinks       []AISearchLink         `json:"related_links,omitempty"`
	ConfidenceLabelKey string                 `json:"confidence_label_key,omitempty"`
	ConfidenceText     string                 `json:"confidence_text,omitempty"`
}

func (c AISearchCandidate) HasDisplayTitle() bool {
	return strings.TrimSpace(c.Title) != "" || strings.TrimSpace(c.TitleValue) != ""
}

func (c AISearchCandidate) DisplayTitle(localize func(string) string) string {
	if title := strings.TrimSpace(c.Title); title != "" {
		return title
	}

	value := strings.TrimSpace(c.TitleValue)
	if value == "" {
		return ""
	}
	if key := strings.TrimSpace(c.TitleLabelKey); key != "" {
		label := strings.TrimSpace(localize(key))
		if label != "" {
			return strings.TrimSpace(label + " " + value)
		}
	}
	return value
}

func (c AISearchCandidate) DisplayConfidence(localize func(string) string) string {
	if key := strings.TrimSpace(c.ConfidenceLabelKey); key != "" {
		if label := strings.TrimSpace(localize(key)); label != "" {
			return label
		}
	}
	return strings.TrimSpace(c.ConfidenceText)
}

func (i AISearchEvidenceItem) DisplayLabel(localize func(string) string) string {
	if key := strings.TrimSpace(i.LabelKey); key != "" {
		if label := strings.TrimSpace(localize(key)); label != "" {
			return label
		}
	}
	return strings.TrimSpace(i.Label)
}

func (l AISearchLink) DisplayLabel(localize func(string) string) string {
	if key := strings.TrimSpace(l.LabelKey); key != "" {
		if label := strings.TrimSpace(localize(key)); label != "" {
			return label
		}
	}
	return strings.TrimSpace(l.Label)
}

type AISearchToolState struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	LabelKey    string    `json:"label_key,omitempty"`
	Status      string    `json:"status"`
	StatusKey   string    `json:"status_key,omitempty"`
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
