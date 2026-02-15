package spotlight

import (
	"time"

	"github.com/google/uuid"
)

type DocumentEventType string

const (
	DocumentEventCreate DocumentEventType = "create"
	DocumentEventUpdate DocumentEventType = "update"
	DocumentEventDelete DocumentEventType = "delete"
)

type SearchIntent string

const (
	SearchIntentNavigate SearchIntent = "navigate"
	SearchIntentEntity   SearchIntent = "entity"
	SearchIntentHelp     SearchIntent = "help"
	SearchIntentMixed    SearchIntent = "mixed"
)

type ActionType string

const (
	ActionTypeNavigate   ActionType = "navigate"
	ActionTypeOpenReport ActionType = "open_report"
	ActionTypeShowSteps  ActionType = "show_steps"
)

type Visibility string

const (
	VisibilityPublic     Visibility = "public"
	VisibilityOwner      Visibility = "owner"
	VisibilityRestricted Visibility = "restricted"
)

type AccessPolicy struct {
	Visibility         Visibility
	OwnerID            string
	AllowedUsers       []string
	AllowedRoles       []string
	AllowedPermissions []string
}

type SearchDocument struct {
	ID         string
	TenantID   uuid.UUID
	Provider   string
	EntityType string
	Title      string
	Body       string
	URL        string
	Language   string
	Metadata   map[string]string
	Access     AccessPolicy
	UpdatedAt  time.Time
	Embedding  []float32
}

type DocumentEvent struct {
	Type       DocumentEventType
	Document   *SearchDocument
	DocumentID string
	OccurredAt time.Time
}

type SearchRequest struct {
	Query          string
	TenantID       uuid.UUID
	UserID         string
	Roles          []string
	Permissions    []string
	TopK           int
	Intent         SearchIntent
	Language       string
	Filters        map[string]string
	QueryEmbedding []float32
}

type SearchHit struct {
	Document     SearchDocument
	LexicalScore float64
	VectorScore  float64
	FinalScore   float64
	WhyMatched   string
}

type AgentAction struct {
	Type              ActionType
	Label             string
	TargetURL         string
	NeedsConfirmation bool
}

type AgentAnswer struct {
	Summary   string
	Citations []SearchDocument
	Actions   []AgentAction
}

type SearchResponse struct {
	Navigate  []SearchHit
	Data      []SearchHit
	Knowledge []SearchHit
	Other     []SearchHit
	Agent     *AgentAnswer
}

type ViewResponse struct {
	Groups []ViewGroup
	Agent  *ViewAgent
}

type ViewGroup struct {
	Key   string
	Title string
	Hits  []SearchHit
}

type ViewAgent struct {
	Summary string
	Actions []AgentAction
}

func (r SearchRequest) normalizedTopK() int {
	if r.TopK <= 0 {
		return 20
	}
	if r.TopK > 100 {
		return 100
	}
	return r.TopK
}
