package rpc

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
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
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
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
	Content     string       `json:"content"`
	Explanation string       `json:"explanation,omitempty"`
	Citations   []Citation   `json:"citations"`
	Artifacts   []any        `json:"artifacts"` // kept for UI compatibility; populated via separate RPC call
	CodeOutputs []CodeOutput `json:"codeOutputs"`
	CreatedAt   string       `json:"createdAt"`
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
	Artifacts []Artifact `json:"artifacts"`
}

func Router() *applet.TypedRPCRouter {
	r := applet.NewTypedRPCRouter()
	applet.AddProcedure(r, "bichat.ping", applet.Procedure[PingParams, PingResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, _ PingParams) (PingResult, error) {
			const op serrors.Op = "bichat.rpc.ping"

			tenantID, err := composables.UseTenantID(ctx)
			if err != nil {
				return PingResult{}, serrors.E(op, serrors.Internal, err)
			}

			return PingResult{
				Ok:       true,
				TenantID: tenantID.String(),
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.list", applet.Procedure[SessionListParams, SessionListResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionListParams) (SessionListResult, error) {
			const op serrors.Op = "bichat.rpc.session.list"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionListResult{}, serrors.E(op, err)
			}
			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionListResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			opts := domain.ListOptions{Limit: p.Limit, Offset: p.Offset}
			list, err := chatSvc.ListUserSessions(ctx, int64(user.ID()), opts)
			if err != nil {
				return SessionListResult{}, serrors.E(op, err)
			}
			out := make([]Session, 0, len(list))
			for _, s := range list {
				out = append(out, toSessionDTO(s))
			}
			return SessionListResult{Sessions: out}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.create", applet.Procedure[SessionCreateParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionCreateParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.create"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}
			tenantID, err := composables.UseTenantID(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}

			s, err := chatSvc.CreateSession(ctx, tenantID, int64(user.ID()), p.Title)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.get", applet.Procedure[SessionGetParams, SessionGetResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionGetParams) (SessionGetResult, error) {
			const op serrors.Op = "bichat.rpc.session.get"

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, serrors.Invalid, err)
			}

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			s, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			if s.UserID() != int64(user.ID()) {
				return SessionGetResult{}, serrors.E(op, serrors.PermissionDenied, errors.New("access denied"))
			}

			msgs, err := chatSvc.GetSessionMessages(ctx, sessionID, domain.ListOptions{Limit: 500})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			return SessionGetResult{
				Session:         toSessionDTO(s),
				Turns:           buildTurns(msgs),
				PendingQuestion: nil,
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.updateTitle", applet.Procedure[SessionUpdateTitleParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionUpdateTitleParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.updateTitle"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}

			s, err := chatSvc.UpdateSessionTitle(ctx, sessionID, p.Title)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.clear", applet.Procedure[SessionIDParams, SessionClearResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionClearResult, error) {
			const op serrors.Op = "bichat.rpc.session.clear"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, serrors.Invalid, err)
			}
			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			session, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, err)
			}
			if session.UserID() != int64(user.ID()) {
				return SessionClearResult{}, serrors.E(op, serrors.PermissionDenied, errors.New("access denied"))
			}

			result, err := chatSvc.ClearSessionHistory(ctx, sessionID)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, err)
			}

			return SessionClearResult{
				Success:          result.Success,
				DeletedMessages:  result.DeletedMessages,
				DeletedArtifacts: result.DeletedArtifacts,
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.compact", applet.Procedure[SessionIDParams, SessionCompactResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCompactResult, error) {
			const op serrors.Op = "bichat.rpc.session.compact"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, serrors.Invalid, err)
			}
			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			session, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, err)
			}
			if session.UserID() != int64(user.ID()) {
				return SessionCompactResult{}, serrors.E(op, serrors.PermissionDenied, errors.New("access denied"))
			}

			result, err := chatSvc.CompactSessionHistory(ctx, sessionID)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, err)
			}

			return SessionCompactResult{
				Success:          result.Success,
				Summary:          result.Summary,
				DeletedMessages:  result.DeletedMessages,
				DeletedArtifacts: result.DeletedArtifacts,
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.delete", applet.Procedure[SessionIDParams, OkResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.session.delete"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if err := chatSvc.DeleteSession(ctx, sessionID); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.pin", applet.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.pin"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			s, err := chatSvc.PinSession(ctx, sessionID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.unpin", applet.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.unpin"

			chatSvc, err := resolveChatService(ctx)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			s, err := chatSvc.UnpinSession(ctx, sessionID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.artifacts", applet.Procedure[SessionArtifactsParams, SessionArtifactsResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionArtifactsParams) (SessionArtifactsResult, error) {
			const op serrors.Op = "bichat.rpc.session.artifacts"

			artifactSvc, err := resolveArtifactService(ctx)
			if err != nil {
				return SessionArtifactsResult{}, serrors.E(op, err)
			}
			sessionID, err := parseUUID(p.SessionID)
			if err != nil {
				return SessionArtifactsResult{}, serrors.E(op, serrors.Invalid, err)
			}

			opts := domain.ListOptions{Limit: p.Limit, Offset: p.Offset}
			list, err := artifactSvc.GetSessionArtifacts(ctx, sessionID, opts)
			if err != nil {
				return SessionArtifactsResult{}, serrors.E(op, err)
			}
			out := make([]Artifact, 0, len(list))
			for _, a := range list {
				out = append(out, toArtifactDTO(a))
			}
			return SessionArtifactsResult{Artifacts: out}, nil
		},
	})

	return r
}

func resolveChatService(ctx context.Context) (services.ChatService, error) {
	app, err := application.UseApp(ctx)
	if err != nil {
		return nil, err
	}
	for _, svc := range app.Services() {
		if s, ok := svc.(services.ChatService); ok {
			return s, nil
		}
	}
	return nil, errors.New("chat service not registered")
}

func resolveArtifactService(ctx context.Context) (services.ArtifactService, error) {
	app, err := application.UseApp(ctx)
	if err != nil {
		return nil, err
	}
	for _, svc := range app.Services() {
		if s, ok := svc.(services.ArtifactService); ok {
			return s, nil
		}
	}
	return nil, errors.New("artifact service not registered")
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
		case types.RoleAssistant:
			if current == nil || current.AssistantTurn != nil {
				continue
			}
			current.AssistantTurn = &AssistantTurn{
				ID:          m.ID().String(),
				Content:     m.Content(),
				Citations:   mapCitations(m.Citations()),
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

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}
