package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type PingParams struct{}

type PingResult struct {
	Ok       bool   `json:"ok"`
	TenantID string `json:"tenantId"`
}

type Session struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type SessionListParams struct {
	Limit           int  `json:"limit"`
	Offset          int  `json:"offset"`
	IncludeArchived bool `json:"includeArchived"`
}

// SessionListResult is the response for bichat.session.list.
type SessionListResult struct {
	Sessions []Session `json:"sessions"`
	// Total is the full count of sessions matching the filter (includeArchived, etc.), not the page size.
	Total   int  `json:"total,omitempty"`
	HasMore bool `json:"hasMore"`
}

type SessionCreateParams struct {
	Title string `json:"title"`
}

type SessionCreateResult struct {
	Session Session `json:"session"`
}

type SessionGetParams struct {
	ID string `json:"id"`
}

type Attachment struct {
	ID        string `json:"id"`
	UploadID  *int64 `json:"uploadId,omitempty"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
	URL       string `json:"url,omitempty"`
}

type Citation struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	StartIndex int    `json:"startIndex"`
	EndIndex   int    `json:"endIndex"`
	Excerpt    string `json:"excerpt,omitempty"`
	Source     string `json:"source,omitempty"`
}

type CodeOutput struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

type UserTurn struct {
	ID          string       `json:"id"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments"`
	CreatedAt   string       `json:"createdAt"`
}

type AssistantTurn struct {
	ID          string       `json:"id"`
	Role        string       `json:"role,omitempty"`
	Content     string       `json:"content"`
	Explanation string       `json:"explanation,omitempty"`
	Citations   []Citation   `json:"citations"`
	ToolCalls   []ToolCall   `json:"toolCalls,omitempty"`
	Debug       *DebugTrace  `json:"debug,omitempty"`
	Artifacts   []any        `json:"artifacts"` // kept for UI compatibility; populated via separate RPC call
	CodeOutputs []CodeOutput `json:"codeOutputs"`
	CreatedAt   string       `json:"createdAt"`
}

type ToolCall struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Arguments  string `json:"arguments"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type DebugUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CachedTokens     int     `json:"cachedTokens"`
	Cost             float64 `json:"cost"`
}

type DebugToolCall struct {
	CallID     string `json:"callId,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type DebugGeneration struct {
	ID                string          `json:"id,omitempty"`
	RequestID         string          `json:"requestId,omitempty"`
	Model             string          `json:"model,omitempty"`
	Provider          string          `json:"provider,omitempty"`
	FinishReason      string          `json:"finishReason,omitempty"`
	PromptTokens      int             `json:"promptTokens,omitempty"`
	CompletionTokens  int             `json:"completionTokens,omitempty"`
	TotalTokens       int             `json:"totalTokens,omitempty"`
	CachedTokens      int             `json:"cachedTokens,omitempty"`
	Cost              float64         `json:"cost,omitempty"`
	LatencyMs         int64           `json:"latencyMs,omitempty"`
	Input             string          `json:"input,omitempty"`
	Output            string          `json:"output,omitempty"`
	Thinking          string          `json:"thinking,omitempty"`
	ObservationReason string          `json:"observationReason,omitempty"`
	StartedAt         string          `json:"startedAt,omitempty"`
	CompletedAt       string          `json:"completedAt,omitempty"`
	ToolCalls         []DebugToolCall `json:"toolCalls,omitempty"`
}

type DebugSpan struct {
	ID           string                 `json:"id,omitempty"`
	ParentID     string                 `json:"parentId,omitempty"`
	GenerationID string                 `json:"generationId,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Level        string                 `json:"level,omitempty"`
	CallID       string                 `json:"callId,omitempty"`
	ToolName     string                 `json:"toolName,omitempty"`
	Input        string                 `json:"input,omitempty"`
	Output       string                 `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	DurationMs   int64                  `json:"durationMs,omitempty"`
	StartedAt    string                 `json:"startedAt,omitempty"`
	CompletedAt  string                 `json:"completedAt,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

type DebugEvent struct {
	ID           string                 `json:"id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Level        string                 `json:"level,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	SpanID       string                 `json:"spanId,omitempty"`
	GenerationID string                 `json:"generationId,omitempty"`
	Timestamp    string                 `json:"timestamp,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

type DebugTrace struct {
	SchemaVersion     string            `json:"schemaVersion,omitempty"`
	StartedAt         string            `json:"startedAt,omitempty"`
	CompletedAt       string            `json:"completedAt,omitempty"`
	Usage             *DebugUsage       `json:"usage,omitempty"`
	GenerationMs      int64             `json:"generationMs,omitempty"`
	Tools             []DebugToolCall   `json:"tools,omitempty"`
	Attempts          []DebugGeneration `json:"attempts,omitempty"`
	Spans             []DebugSpan       `json:"spans,omitempty"`
	Events            []DebugEvent      `json:"events,omitempty"`
	TraceID           string            `json:"traceId,omitempty"`
	TraceURL          string            `json:"traceUrl,omitempty"`
	SessionID         string            `json:"sessionId,omitempty"`
	Thinking          string            `json:"thinking,omitempty"`
	ObservationReason string            `json:"observationReason,omitempty"`
}

type ConversationTurn struct {
	ID            string         `json:"id"`
	SessionID     string         `json:"sessionId"`
	UserTurn      UserTurn       `json:"userTurn"`
	AssistantTurn *AssistantTurn `json:"assistantTurn,omitempty"`
	CreatedAt     string         `json:"createdAt"`
}

type PendingQuestionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type PendingQuestionItem struct {
	ID      string                  `json:"id"`
	Text    string                  `json:"text"`
	Type    string                  `json:"type"`
	Options []PendingQuestionOption `json:"options"`
}

type PendingQuestion struct {
	CheckpointID string                `json:"checkpointId"`
	AgentName    string                `json:"agentName,omitempty"`
	TurnID       string                `json:"turnId"`
	Questions    []PendingQuestionItem `json:"questions"`
}

type SessionGetResult struct {
	Session         Session            `json:"session"`
	Turns           []ConversationTurn `json:"turns"`
	PendingQuestion *PendingQuestion   `json:"pendingQuestion,omitempty"`
}

// ArtifactIDParams is used by bichat.artifact.delete (p.ID is artifact ID).
type ArtifactIDParams struct {
	ID string `json:"id"`
}

type SessionIDParams struct {
	ID string `json:"id"`
}

type SessionClearResult struct {
	Success          bool  `json:"success"`
	DeletedMessages  int64 `json:"deletedMessages"`
	DeletedArtifacts int64 `json:"deletedArtifacts"`
}

type SessionCompactResult struct {
	Success          bool   `json:"success"`
	Summary          string `json:"summary"`
	DeletedMessages  int64  `json:"deletedMessages"`
	DeletedArtifacts int64  `json:"deletedArtifacts"`
}

type SessionUpdateTitleParams struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type OkResult struct {
	Ok bool `json:"ok"`
}

type Artifact struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"sessionId"`
	MessageID   string         `json:"messageId,omitempty"`
	UploadID    *int64         `json:"uploadId,omitempty"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	MimeType    string         `json:"mimeType,omitempty"`
	URL         string         `json:"url,omitempty"`
	SizeBytes   int64          `json:"sizeBytes"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"createdAt"`
}

type SessionArtifactsParams struct {
	SessionID string `json:"sessionId"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type SessionArtifactsResult struct {
	Artifacts  []Artifact `json:"artifacts"`
	HasMore    bool       `json:"hasMore"`
	NextOffset int        `json:"nextOffset"`
}

type SessionUploadArtifactsParams struct {
	SessionID   string       `json:"sessionId"`
	Attachments []Attachment `json:"attachments"`
}

type SessionUploadArtifactsResult struct {
	Artifacts []Artifact `json:"artifacts"`
}

type ArtifactUpdateParams struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ArtifactResult struct {
	Artifact Artifact `json:"artifact"`
}

type QuestionSubmitParams struct {
	SessionID    string            `json:"sessionId"`
	CheckpointID string            `json:"checkpointId"`
	Answers      map[string]string `json:"answers"`
}

type QuestionCancelParams struct {
	SessionID string `json:"sessionId"`
}

func toSessionDTO(s domain.Session) Session {
	if s == nil {
		return Session{
			ID:        "",
			Title:     "",
			Status:    "active",
			Pinned:    false,
			CreatedAt: "",
			UpdatedAt: "",
		}
	}
	status := "active"
	if string(s.Status()) != "" {
		switch string(s.Status()) {
		case "ARCHIVED", "archived":
			status = "archived"
		default:
			status = "active"
		}
	}
	var createdAt, updatedAt time.Time
	createdAt = s.CreatedAt()
	updatedAt = s.UpdatedAt()
	return Session{
		ID:        s.ID().String(),
		Title:     s.Title(),
		Status:    status,
		Pinned:    s.Pinned(),
		CreatedAt: createdAt.Format(time.RFC3339),
		UpdatedAt: updatedAt.Format(time.RFC3339),
	}
}

func buildTurns(msgs []types.Message) []ConversationTurn {
	turns := make([]ConversationTurn, 0)
	var current *ConversationTurn

	for _, m := range msgs {
		if m == nil {
			continue
		}

		switch m.Role() {
		case types.RoleUser:
			t := ConversationTurn{
				ID:        m.ID().String(),
				SessionID: m.SessionID().String(),
				UserTurn: UserTurn{
					ID:          m.ID().String(),
					Content:     m.Content(),
					Attachments: mapAttachments(m.Attachments()),
					CreatedAt:   m.CreatedAt().Format(time.RFC3339),
				},
				CreatedAt: m.CreatedAt().Format(time.RFC3339),
			}
			turns = append(turns, t)
			current = &turns[len(turns)-1]
		case types.RoleSystem:
			t := ConversationTurn{
				ID:        m.ID().String(),
				SessionID: m.SessionID().String(),
				UserTurn: UserTurn{
					ID:          m.ID().String(),
					Content:     "",
					Attachments: []Attachment{},
					CreatedAt:   m.CreatedAt().Format(time.RFC3339),
				},
				AssistantTurn: &AssistantTurn{
					ID:          m.ID().String(),
					Role:        string(types.RoleSystem),
					Content:     m.Content(),
					Citations:   mapCitations(m.Citations()),
					ToolCalls:   mapToolCalls(m.ToolCalls()),
					Debug:       mapDebugTrace(m.DebugTrace()),
					Artifacts:   []any{},
					CodeOutputs: mapCodeOutputs(m.CodeOutputs()),
					CreatedAt:   m.CreatedAt().Format(time.RFC3339),
				},
				CreatedAt: m.CreatedAt().Format(time.RFC3339),
			}
			turns = append(turns, t)
		case types.RoleTool:
			continue
		case types.RoleAssistant:
			if current == nil {
				continue
			}
			if current.AssistantTurn != nil {
				// Concatenate consecutive assistant messages (e.g., original + continuation after resume)
				if current.AssistantTurn.Content != "" && m.Content() != "" {
					current.AssistantTurn.Content += "\n\n"
				}
				current.AssistantTurn.Content += m.Content()
				// Merge tool calls
				newToolCalls := mapToolCalls(m.ToolCalls())
				if len(newToolCalls) > 0 {
					current.AssistantTurn.ToolCalls = append(current.AssistantTurn.ToolCalls, newToolCalls...)
				}
				// Merge code outputs
				newCodeOutputs := mapCodeOutputs(m.CodeOutputs())
				if len(newCodeOutputs) > 0 {
					current.AssistantTurn.CodeOutputs = append(current.AssistantTurn.CodeOutputs, newCodeOutputs...)
				}
				// Use latest debug trace
				if dt := mapDebugTrace(m.DebugTrace()); dt != nil {
					current.AssistantTurn.Debug = dt
				}
				continue
			}
			current.AssistantTurn = &AssistantTurn{
				ID:          m.ID().String(),
				Role:        string(types.RoleAssistant),
				Content:     m.Content(),
				Citations:   mapCitations(m.Citations()),
				ToolCalls:   mapToolCalls(m.ToolCalls()),
				Debug:       mapDebugTrace(m.DebugTrace()),
				Artifacts:   []any{},
				CodeOutputs: mapCodeOutputs(m.CodeOutputs()),
				CreatedAt:   m.CreatedAt().Format(time.RFC3339),
			}
		default:
			continue
		}
	}

	return turns
}

func mapAttachments(in []types.Attachment) []Attachment {
	out := make([]Attachment, 0, len(in))
	for _, a := range in {
		dto := Attachment{
			ID:        a.ID.String(),
			UploadID:  a.UploadID,
			Filename:  a.FileName,
			MimeType:  a.MimeType,
			SizeBytes: a.SizeBytes,
		}
		if a.FilePath != "" {
			dto.URL = a.FilePath
		}
		out = append(out, dto)
	}
	return out
}

func mapCitations(in []types.Citation) []Citation {
	out := make([]Citation, 0, len(in))
	for i, c := range in {
		id := fmt.Sprintf("%d", i)
		out = append(out, Citation{
			ID:         id,
			Type:       c.Type,
			Title:      c.Title,
			URL:        c.URL,
			StartIndex: c.StartIndex,
			EndIndex:   c.EndIndex,
			Excerpt:    c.Excerpt,
		})
	}
	return out
}

func mapToolCalls(in []types.ToolCall) []ToolCall {
	if len(in) == 0 {
		return nil
	}

	out := make([]ToolCall, 0, len(in))
	for _, tc := range in {
		out = append(out, ToolCall{
			ID:         tc.ID,
			Name:       tc.Name,
			Arguments:  tc.Arguments,
			Result:     tc.Result,
			Error:      tc.Error,
			DurationMs: tc.DurationMs,
		})
	}

	return out
}

func mapDebugTrace(trace *types.DebugTrace) *DebugTrace {
	if trace == nil {
		return nil
	}

	var usage *DebugUsage
	if trace.Usage != nil {
		usage = &DebugUsage{
			PromptTokens:     trace.Usage.PromptTokens,
			CompletionTokens: trace.Usage.CompletionTokens,
			TotalTokens:      trace.Usage.TotalTokens,
			CachedTokens:     trace.Usage.CachedTokens,
			Cost:             trace.Usage.Cost,
		}
	}

	tools := make([]DebugToolCall, 0, len(trace.Tools))
	for _, tool := range trace.Tools {
		tools = append(tools, DebugToolCall{
			CallID:     tool.CallID,
			Name:       tool.Name,
			Arguments:  tool.Arguments,
			Result:     tool.Result,
			Error:      tool.Error,
			DurationMs: tool.DurationMs,
		})
	}

	attempts := make([]DebugGeneration, 0, len(trace.Attempts))
	for _, attempt := range trace.Attempts {
		toolCalls := make([]DebugToolCall, 0, len(attempt.ToolCalls))
		for _, tool := range attempt.ToolCalls {
			toolCalls = append(toolCalls, DebugToolCall{
				CallID:     tool.CallID,
				Name:       tool.Name,
				Arguments:  tool.Arguments,
				Result:     tool.Result,
				Error:      tool.Error,
				DurationMs: tool.DurationMs,
			})
		}
		attempts = append(attempts, DebugGeneration{
			ID:                attempt.ID,
			RequestID:         attempt.RequestID,
			Model:             attempt.Model,
			Provider:          attempt.Provider,
			FinishReason:      attempt.FinishReason,
			PromptTokens:      attempt.PromptTokens,
			CompletionTokens:  attempt.CompletionTokens,
			TotalTokens:       attempt.TotalTokens,
			CachedTokens:      attempt.CachedTokens,
			Cost:              attempt.Cost,
			LatencyMs:         attempt.LatencyMs,
			Input:             attempt.Input,
			Output:            attempt.Output,
			Thinking:          attempt.Thinking,
			ObservationReason: attempt.ObservationReason,
			StartedAt:         attempt.StartedAt,
			CompletedAt:       attempt.CompletedAt,
			ToolCalls:         toolCalls,
		})
	}

	spans := make([]DebugSpan, 0, len(trace.Spans))
	for _, span := range trace.Spans {
		spans = append(spans, DebugSpan{
			ID:           span.ID,
			ParentID:     span.ParentID,
			GenerationID: span.GenerationID,
			Name:         span.Name,
			Type:         span.Type,
			Status:       span.Status,
			Level:        span.Level,
			CallID:       span.CallID,
			ToolName:     span.ToolName,
			Input:        span.Input,
			Output:       span.Output,
			Error:        span.Error,
			DurationMs:   span.DurationMs,
			StartedAt:    span.StartedAt,
			CompletedAt:  span.CompletedAt,
			Attributes:   span.Attributes,
		})
	}

	events := make([]DebugEvent, 0, len(trace.Events))
	for _, event := range trace.Events {
		events = append(events, DebugEvent{
			ID:           event.ID,
			Name:         event.Name,
			Type:         event.Type,
			Level:        event.Level,
			Message:      event.Message,
			Reason:       event.Reason,
			SpanID:       event.SpanID,
			GenerationID: event.GenerationID,
			Timestamp:    event.Timestamp,
			Attributes:   event.Attributes,
		})
	}

	return &DebugTrace{
		SchemaVersion:     trace.SchemaVersion,
		StartedAt:         trace.StartedAt,
		CompletedAt:       trace.CompletedAt,
		Usage:             usage,
		GenerationMs:      trace.GenerationMs,
		Tools:             tools,
		Attempts:          attempts,
		Spans:             spans,
		Events:            events,
		TraceID:           trace.TraceID,
		TraceURL:          trace.TraceURL,
		SessionID:         trace.SessionID,
		Thinking:          trace.Thinking,
		ObservationReason: trace.ObservationReason,
	}
}

func mapCodeOutputs(in []types.CodeInterpreterOutput) []CodeOutput {
	out := make([]CodeOutput, 0, len(in))
	for _, o := range in {
		typ := "text"
		if strings.HasPrefix(o.MimeType, "image/") {
			typ = "image"
		}
		out = append(out, CodeOutput{
			Type:      typ,
			Content:   o.URL,
			Filename:  o.Name,
			MimeType:  o.MimeType,
			SizeBytes: o.Size,
		})
	}
	return out
}

func toArtifactDTO(a domain.Artifact) Artifact {
	out := Artifact{
		ID:          a.ID().String(),
		SessionID:   a.SessionID().String(),
		UploadID:    a.UploadID(),
		Type:        string(a.Type()),
		Name:        a.Name(),
		Description: a.Description(),
		MimeType:    a.MimeType(),
		URL:         a.URL(),
		SizeBytes:   a.SizeBytes(),
		Metadata:    a.Metadata(),
		CreatedAt:   a.CreatedAt().Format(time.RFC3339),
	}
	if mid := a.MessageID(); mid != nil {
		out.MessageID = mid.String()
	}
	return out
}

func requireSessionOwner(ctx context.Context, chatSvc services.ChatService, sessionID uuid.UUID) (domain.Session, error) {
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E("requireSessionOwner", serrors.PermissionDenied, err)
	}
	session, err := chatSvc.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E("requireSessionOwner", err)
	}
	if session.UserID() != int64(user.ID()) {
		return nil, serrors.E("requireSessionOwner", serrors.PermissionDenied, errors.New("access denied"))
	}
	return session, nil
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// pendingQuestionFromMessages scans messages for pending question data
// and builds the DTO with turn ID inference.
func pendingQuestionFromMessages(msgs []types.Message) *PendingQuestion {
	// Find the latest message with pending question data.
	var pendingMsg types.Message
	pendingIndex := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m != nil && m.HasPendingQuestion() {
			pendingMsg = m
			pendingIndex = i
			break
		}
	}
	if pendingMsg == nil {
		return nil
	}
	qd := pendingMsg.QuestionData()

	// Find the turn ID: look backward for the nearest user message
	turnID := pendingMsg.ID().String()
	for i := pendingIndex - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role() == types.RoleUser && msgs[i].CreatedAt().Before(pendingMsg.CreatedAt()) {
			turnID = msgs[i].ID().String()
			break
		}
	}

	questions := make([]PendingQuestionItem, 0, len(qd.Questions))
	for _, q := range qd.Questions {
		options := make([]PendingQuestionOption, 0, len(q.Options))
		for _, opt := range q.Options {
			options = append(options, PendingQuestionOption{ID: opt.ID, Label: opt.Label})
		}
		questions = append(questions, PendingQuestionItem{
			ID: q.ID, Text: q.Text, Type: q.Type, Options: options,
		})
	}

	return &PendingQuestion{
		CheckpointID: qd.CheckpointID,
		AgentName:    qd.AgentName,
		TurnID:       turnID,
		Questions:    questions,
	}
}
