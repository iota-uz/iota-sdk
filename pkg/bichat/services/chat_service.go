// Package services provides this package.
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

// ErrRunEventLogUnavailable is returned by TailRunEvents when the durable
// event log is not configured (e.g. REDIS_URL unset in dev). Controllers
// map this to 501 Not Implemented so clients know to fall back to the
// legacy in-memory ResumeStream endpoint.
var ErrRunEventLogUnavailable = errors.New("run event log unavailable")

// SessionCommands manages mutating session actions.
type SessionCommands interface {
	CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error)
	UpsertSessionMember(ctx context.Context, command domain.SessionMemberUpsert) error
	RemoveSessionMember(ctx context.Context, command domain.SessionMemberRemoval) error
	UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error)
	ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (ClearSessionHistoryResponse, error)
	CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (CompactSessionHistoryResponse, error)
	CompactSessionHistoryAsync(ctx context.Context, sessionID uuid.UUID) (AsyncRunAccepted, error)
	GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error
}

// SessionQueries reads session and access projections.
type SessionQueries interface {
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
	ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error)
}

// TurnCommands handles non-streaming turn execution.
type TurnCommands interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
}

// TurnQueries reads conversation messages for a session.
type TurnQueries interface {
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error)
}

// HITLCommands resumes or rejects pending user-interaction questions.
type HITLCommands interface {
	ResumeWithAnswer(ctx context.Context, req ResumeRequest) (*SendMessageResponse, error)
	ResumeWithAnswerAsync(ctx context.Context, req ResumeRequest) (AsyncRunAccepted, error)

	// RejectPendingQuestion rejects a pending HITL question and resumes the agent
	// with "user rejected questions" feedback.
	RejectPendingQuestion(ctx context.Context, sessionID uuid.UUID) (*SendMessageResponse, error)
	RejectPendingQuestionAsync(ctx context.Context, sessionID uuid.UUID) (AsyncRunAccepted, error)
}

// StreamCommands manages streaming turn execution and active stream lifecycle.
type StreamCommands interface {
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

	// TailRunEvents reads the durable per-run event log. Events with
	// stream id > `from` are delivered in order; an empty `from` streams
	// from the beginning. onEvent is invoked once per event and must be
	// non-blocking; callers typically forward it as an SSE line. The
	// call returns when a terminal event is observed, ctx is cancelled,
	// or the event log TTL has expired. Returns ErrRunEventLogUnavailable
	// when the backing Redis stream is not configured.
	TailRunEvents(ctx context.Context, sessionID, runID uuid.UUID, from string, onEvent func(RunEventDelivery)) error
}

// RunEventDelivery is a single durable event delivered through TailRunEvents.
// StreamID is the Redis stream id used as the SSE `id:` field for
// Last-Event-ID reconnect. Payload is the JSON-encoded
// httpdto.StreamChunkPayload ready to be written verbatim to the SSE body.
type RunEventDelivery struct {
	StreamID string
	Type     string
	Payload  []byte
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

type AsyncRunOperation string

const (
	AsyncRunOperationQuestionSubmit AsyncRunOperation = "question_submit"
	AsyncRunOperationQuestionReject AsyncRunOperation = "question_reject"
	AsyncRunOperationSessionCompact AsyncRunOperation = "session_compact"
)

type AsyncRunAccepted struct {
	Accepted  bool
	Operation AsyncRunOperation
	SessionID uuid.UUID
	RunID     uuid.UUID
	StartedAt time.Time
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
	// Model overrides the default model for this request. Must match a registered model name.
	Model *string
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
	// TextBlockSeq identifies the assistant text segment ordinal that just ended
	// (when Type is ChunkTypeTextBlockEnd). Zero-based within a single run.
	TextBlockSeq int
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
	ChunkTypePing          ChunkType = "ping"
	// ChunkTypeTextBlockEnd marks the boundary of an assistant text segment
	// inside a turn that contains tool calls. The frontend uses it to render
	// each text-then-tool sequence as a distinct block instead of merging
	// every text delta into one paragraph.
	ChunkTypeTextBlockEnd ChunkType = "text_block_end"
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
