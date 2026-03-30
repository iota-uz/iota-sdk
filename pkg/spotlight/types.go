// Package spotlight defines the document, request, response, and session types
// shared across Spotlight indexing and search flows.
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

type ResultDomain string

const (
	ResultDomainLookup    ResultDomain = "lookup"
	ResultDomainNavigate  ResultDomain = "navigate"
	ResultDomainKnowledge ResultDomain = "knowledge"
	ResultDomainAction    ResultDomain = "action"
	ResultDomainOther     ResultDomain = "other"
)

type QueryMode string

const (
	QueryModeExplore  QueryMode = "explore"
	QueryModeLookup   QueryMode = "lookup"
	QueryModeNavigate QueryMode = "navigate"
	QueryModeHelp     QueryMode = "help"
)

type SearchStage string

const (
	SearchStageFast    SearchStage = "fast"
	SearchStageIndexed SearchStage = "indexed"
	SearchStageExpand  SearchStage = "expand"
)

type SearchStageStatus string

const (
	SearchStageStatusPending   SearchStageStatus = "pending"
	SearchStageStatusRunning   SearchStageStatus = "running"
	SearchStageStatusCompleted SearchStageStatus = "completed"
	SearchStageStatusFailed    SearchStageStatus = "failed"
	SearchStageStatusSkipped   SearchStageStatus = "skipped"
)

type Visibility string

const (
	VisibilityPublic     Visibility = "public"
	VisibilityOwner      Visibility = "owner"
	VisibilityRestricted Visibility = "restricted"
)

type AccessPolicy struct {
	Visibility         Visibility `json:"visibility"`
	OwnerID            string     `json:"owner_id"`
	AllowedUsers       []string   `json:"allowed_users"`
	AllowedRoles       []string   `json:"allowed_roles"`
	AllowedPermissions []string   `json:"allowed_permissions"`
}

type SearchDocument struct {
	ID          string            `json:"id"`
	TenantID    uuid.UUID         `json:"tenant_id"`
	Provider    string            `json:"provider"`
	EntityType  string            `json:"entity_type"`
	Domain      ResultDomain      `json:"domain"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Body        string            `json:"body"`
	SearchText  string            `json:"search_text"`
	ExactTerms  []string          `json:"exact_terms"`
	URL         string            `json:"url"`
	Language    string            `json:"language"`
	Metadata    map[string]string `json:"metadata"`
	Access      AccessPolicy      `json:"access_policy"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Embedding   []float32         `json:"embedding,omitempty"`
}

type DocumentEvent struct {
	Type       DocumentEventType `json:"type"`
	Document   *SearchDocument   `json:"document,omitempty"`
	DocumentID string            `json:"document_id,omitempty"`
	OccurredAt time.Time         `json:"occurred_at,omitempty"`
}

type SearchRequest struct {
	Query            string
	TenantID         uuid.UUID
	UserID           string
	Roles            []string
	Permissions      []string
	TopK             int
	Intent           SearchIntent
	Language         string
	Filters          map[string]string
	QueryEmbedding   []float32
	Mode             QueryMode
	ExactTerms       []string
	PreferredDomains []ResultDomain
}

type SearchSessionAccess struct {
	TenantID uuid.UUID
	UserID   string
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
	LabelTrKey        string // if set, label is localized at render time
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
	Groups    []SearchGroup
	Agent     *AgentAnswer
}

type SearchStageState struct {
	Stage            SearchStage       `json:"stage"`
	Status           SearchStageStatus `json:"status"`
	TotalSources     int               `json:"total_sources"`
	CompletedSources int               `json:"completed_sources"`
	PendingSources   int               `json:"pending_sources"`
	ResultCount      int               `json:"result_count"`
	Error            string            `json:"error,omitempty"`
}

type SearchSessionSnapshot struct {
	ID        string             `json:"id"`
	Query     string             `json:"query"`
	Response  SearchResponse     `json:"response"`
	Stages    []SearchStageState `json:"stages"`
	Loading   bool               `json:"loading"`
	Completed bool               `json:"completed"`
	Version   int64              `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type SearchGroup struct {
	Domain ResultDomain
	Title  string
	Hits   []SearchHit
}

type ViewResponse struct {
	Groups []ViewGroup
	Agent  *ViewAgent
}

type ViewGroup struct {
	Key      string
	Title    string
	TitleKey string
	Hits     []SearchHit
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

func (s SearchSessionSnapshot) PendingCount() int {
	count := 0
	for _, stage := range s.Stages {
		count += stage.PendingSources
	}
	return count
}
