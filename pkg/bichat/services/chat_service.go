package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ErrRunNotFoundOrFinished is returned by ResumeStream when the run is not active in this process.
var ErrRunNotFoundOrFinished = errors.New("generation run not found or already finished")

// SessionService manages session lifecycle, permissions, and metadata.
type SessionService interface {
	// Session management
	CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error)
	CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error)
	ListAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.SessionSummary, error)
	CountAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error)
	ListAllSessions(ctx context.Context, requestingUserID int64, opts domain.ListOptions, ownerUserID *int64) ([]domain.SessionSummary, error)
	CountAllSessions(ctx context.Context, opts domain.ListOptions, ownerUserID *int64) (int, error)
	ResolveSessionAccess(ctx context.Context, sessionID uuid.UUID, userID int64, allowReadAll bool) (domain.SessionAccess, error)
	ListSessionMembers(ctx context.Context, sessionID uuid.UUID) ([]domain.SessionMember, error)
	GetTenantUser(ctx context.Context, userID int64) (domain.SessionUser, error)
	UpsertSessionMember(ctx context.Context, sessionID uuid.UUID, userID int64, role domain.SessionMemberRole) error
	RemoveSessionMember(ctx context.Context, sessionID uuid.UUID, userID int64) error
	ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error)
	UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error)
	ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (ClearSessionHistoryResponse, error)
	CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (CompactSessionHistoryResponse, error)
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

// ConversationService handles non-streaming turn execution and message retrieval.
type ConversationService interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error)
}

// HITLService resumes or rejects pending user-interaction questions.
type HITLService interface {
	ResumeWithAnswer(ctx context.Context, req ResumeRequest) (*SendMessageResponse, error)

	// RejectPendingQuestion rejects a pending HITL question and resumes the agent
	// with "user rejected questions" feedback.
	RejectPendingQuestion(ctx context.Context, sessionID uuid.UUID) (*SendMessageResponse, error)
}

// StreamService manages streaming turn execution and active stream lifecycle.
type StreamService interface {
	SendMessageStream(ctx context.Context, req SendMessageRequest, onChunk func(StreamChunk)) error
	// StopGeneration cancels the active stream for the given session, if any.
	// After stop, no partial assistant message is persisted; the next send continues normally.
	StopGeneration(ctx context.Context, sessionID uuid.UUID) error

	// GetStreamStatus returns the active generation run for the session, if any.
	// Used for refresh-safe resume: client checks on load and resumes or shows passive "in progress".
	GetStreamStatus(ctx context.Context, sessionID uuid.UUID) (*StreamStatus, error)

	// ResumeStream attaches to an active run: delivers current snapshot then streams subsequent chunks.
	// Returns ErrRunNotFoundOrFinished if the run is not active in this process.
	ResumeStream(ctx context.Context, sessionID uuid.UUID, runID uuid.UUID, onChunk func(StreamChunk)) error
}

// StreamStatus describes an active streaming run for a session.
type StreamStatus struct {
	Active    bool
	RunID     uuid.UUID
	Snapshot  StreamSnapshot
	StartedAt time.Time
}

// StreamSnapshot is the partial state to send when resuming a stream.
type StreamSnapshot struct {
	PartialContent  string
	PartialMetadata map[string]any
}

// SendMessageRequest contains the input for sending a message
type SendMessageRequest struct {
	SessionID   uuid.UUID
	UserID      int64
	Content     string
	Attachments []domain.Attachment
	DebugMode   bool
	// ReplaceFromMessageID truncates session history from this user message onward
	// before sending the new content (used by edit/regenerate flows).
	ReplaceFromMessageID *uuid.UUID
	// ReasoningEffort overrides the default reasoning effort for this request.
	// Valid values: "low", "medium", "high", "xhigh". Nil means use model default.
	ReasoningEffort *string
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
	Citation     *types.Citation
	Usage        *types.DebugUsage
	Tool         *ToolEvent
	Interrupt    *InterruptEvent
	GenerationMs int64
	Error        error
	Timestamp    time.Time
	// Snapshot is set when Type is ChunkTypeSnapshot (resume flow: partial state so far)
	Snapshot *StreamSnapshot
	// RunID is set when Type is ChunkTypeStreamStarted so the client can store it for resume.
	RunID string
}

// ChunkType represents the type of streaming chunk
type ChunkType string

const (
	ChunkTypeChunk         ChunkType = "chunk"
	ChunkTypeContent       ChunkType = "content"
	ChunkTypeCitation      ChunkType = "citation"
	ChunkTypeToolStart     ChunkType = "tool_start"
	ChunkTypeToolEnd       ChunkType = "tool_end"
	ChunkTypeInterrupt     ChunkType = "interrupt"
	ChunkTypeUsage         ChunkType = "usage"
	ChunkTypeDone          ChunkType = "done"
	ChunkTypeError         ChunkType = "error"
	ChunkTypeThinking      ChunkType = "thinking"
	ChunkTypeSnapshot      ChunkType = "snapshot"
	ChunkTypeStreamStarted ChunkType = "stream_started"
)

// ToolEvent represents a tool execution event in a streaming chunk.
type ToolEvent struct {
	CallID     string
	Name       string
	AgentName  string
	Arguments  string
	Result     string
	Error      error
	DurationMs int64
	Artifacts  []types.ToolArtifact
}

// InterruptEvent represents a HITL interrupt in a streaming chunk.
type InterruptEvent struct {
	CheckpointID       string
	AgentName          string // Name of the agent that triggered this interrupt
	ProviderResponseID string
	Questions          []Question
}

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
