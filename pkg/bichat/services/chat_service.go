package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ChatService manages chat sessions and messages.
// This is the primary public API for chat functionality.
type ChatService interface {
	// Session management
	CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error)
	UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error)
	ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (ClearSessionHistoryResponse, error)
	CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (CompactSessionHistoryResponse, error)

	// Message management
	SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
	SendMessageStream(ctx context.Context, req SendMessageRequest, onChunk func(StreamChunk)) error
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error)

	// Resume after user answers questions (HITL)
	ResumeWithAnswer(ctx context.Context, req ResumeRequest) (*SendMessageResponse, error)

	// Cancel pending question - clears HITL state without resuming
	CancelPendingQuestion(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)

	// Generate session title from first message
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

// SendMessageRequest contains the input for sending a message
type SendMessageRequest struct {
	SessionID   uuid.UUID
	UserID      int64
	Content     string
	Attachments []domain.Attachment
	DebugMode   bool
}

// SendMessageResponse contains the result of sending a message
type SendMessageResponse struct {
	UserMessage      types.Message  // The user's message
	AssistantMessage types.Message  // The assistant's message (nil if interrupted)
	Session          domain.Session // Updated session
	Interrupt        *Interrupt     // Non-nil if agent has questions (HITL)
}

// Interrupt represents a pause in execution waiting for user input (HITL pattern)
type Interrupt struct {
	CheckpointID string
	Questions    []Question
}

// Question represents a question waiting for user input
type Question struct {
	ID      string
	Text    string
	Type    QuestionType
	Options []QuestionOption
}

// QuestionType represents the type of question
type QuestionType string

const (
	QuestionTypeSingleChoice   QuestionType = "single_choice"
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
)

// QuestionOption represents an option for a choice question
type QuestionOption struct {
	ID    string
	Label string
}

// ResumeRequest contains the input for resuming execution after user answers
type ResumeRequest struct {
	SessionID    uuid.UUID
	CheckpointID string
	Answers      map[string]string // Question ID -> Answer
}

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	Type         ChunkType
	Content      string
	Citation     *domain.Citation
	Usage        *TokenUsage
	Tool         *ToolEvent
	GenerationMs int64
	Error        error
	Timestamp    time.Time
}

// ChunkType represents the type of streaming chunk
type ChunkType string

const (
	ChunkTypeChunk     ChunkType = "chunk"
	ChunkTypeContent   ChunkType = "content"
	ChunkTypeCitation  ChunkType = "citation"
	ChunkTypeToolStart ChunkType = "tool_start"
	ChunkTypeToolEnd   ChunkType = "tool_end"
	ChunkTypeUsage     ChunkType = "usage"
	ChunkTypeDone      ChunkType = "done"
	ChunkTypeError     ChunkType = "error"
)

type ClearSessionHistoryResponse struct {
	Success          bool
	DeletedMessages  int64
	DeletedArtifacts int64
}

type CompactSessionHistoryResponse struct {
	Success          bool
	Summary          string
	DeletedMessages  int64
	DeletedArtifacts int64
}

// TokenUsage tracks token consumption and costs
type TokenUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CachedTokens     int     `json:"cachedTokens"`
	Cost             float64 `json:"cost"` // Estimated cost in USD
}
