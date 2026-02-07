package rpc

import (
	"context"
	"encoding/base64"
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

type SessionListResult struct {
	Sessions []Session `json:"sessions"`
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
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mimeType"`
	SizeBytes  int64  `json:"sizeBytes"`
	Base64Data string `json:"base64Data,omitempty"`
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

type DebugTrace struct {
	Usage        *DebugUsage     `json:"usage,omitempty"`
	GenerationMs int64           `json:"generationMs,omitempty"`
	Tools        []DebugToolCall `json:"tools,omitempty"`
}

type ConversationTurn struct {
	ID            string         `json:"id"`
	SessionID     string         `json:"sessionId"`
	UserTurn      UserTurn       `json:"userTurn"`
	AssistantTurn *AssistantTurn `json:"assistantTurn,omitempty"`
	CreatedAt     string         `json:"createdAt"`
}

type SessionGetResult struct {
	Session         Session            `json:"session"`
	Turns           []ConversationTurn `json:"turns"`
	PendingQuestion any                `json:"pendingQuestion"` // currently not implemented
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
		case types.RoleAssistant:
			if current == nil || current.AssistantTurn != nil {
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
			Filename:  a.FileName,
			MimeType:  a.MimeType,
			SizeBytes: a.SizeBytes,
		}
		if len(a.Data) > 0 {
			dto.Base64Data = base64.StdEncoding.EncodeToString(a.Data)
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

	return &DebugTrace{
		Usage:        usage,
		GenerationMs: trace.GenerationMs,
		Tools:        tools,
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
