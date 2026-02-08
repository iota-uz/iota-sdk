package rpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func Router(chatSvc services.ChatService, artifactSvc services.ArtifactService) *applet.TypedRPCRouter {
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

			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionListResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			requestedLimit := p.Limit
			if requestedLimit <= 0 {
				requestedLimit = 50
			}
			opts := domain.ListOptions{Limit: requestedLimit + 1, Offset: p.Offset, IncludeArchived: p.IncludeArchived}
			list, err := chatSvc.ListUserSessions(ctx, int64(user.ID()), opts)
			if err != nil {
				return SessionListResult{}, serrors.E(op, err)
			}
			hasMore := len(list) > requestedLimit
			if hasMore {
				list = list[:requestedLimit]
			}
			out := make([]Session, 0, len(list))
			for _, s := range list {
				out = append(out, toSessionDTO(s))
			}
			return SessionListResult{Sessions: out, Total: len(out), HasMore: hasMore}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.create", applet.Procedure[SessionCreateParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionCreateParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.create"

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

			s, err := requireSessionOwner(ctx, chatSvc, sessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			msgs, err := chatSvc.GetSessionMessages(ctx, sessionID, domain.ListOptions{Limit: 500})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			pq := pendingQuestionFromMessages(msgs)

			return SessionGetResult{
				Session:         toSessionDTO(s),
				Turns:           buildTurns(msgs),
				PendingQuestion: pq,
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.updateTitle", applet.Procedure[SessionUpdateTitleParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionUpdateTitleParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.updateTitle"

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionClearResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCompactResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCompactResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return OkResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
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

			sessionID, err := parseUUID(p.SessionID)
			if err != nil {
				return SessionArtifactsResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionArtifactsResult{}, serrors.E(op, err)
			}

			requestedLimit := p.Limit
			if requestedLimit <= 0 {
				requestedLimit = 50
			}
			offset := p.Offset
			if offset < 0 {
				offset = 0
			}

			opts := domain.ListOptions{Limit: requestedLimit + 1, Offset: offset}
			list, err := artifactSvc.GetSessionArtifacts(ctx, sessionID, opts)
			if err != nil {
				return SessionArtifactsResult{}, serrors.E(op, err)
			}

			hasMore := len(list) > requestedLimit
			if hasMore {
				list = list[:requestedLimit]
			}

			out := make([]Artifact, 0, len(list))
			for _, a := range list {
				out = append(out, toArtifactDTO(a))
			}
			return SessionArtifactsResult{
				Artifacts:  out,
				HasMore:    hasMore,
				NextOffset: offset + len(out),
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.uploadArtifacts", applet.Procedure[SessionUploadArtifactsParams, SessionUploadArtifactsResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionUploadArtifactsParams) (SessionUploadArtifactsResult, error) {
			const op serrors.Op = "bichat.rpc.session.uploadArtifacts"

			sessionID, err := parseUUID(p.SessionID)
			if err != nil {
				return SessionUploadArtifactsResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionUploadArtifactsResult{}, serrors.E(op, err)
			}
			if len(p.Attachments) == 0 {
				return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, "attachments are required")
			}

			uploads := make([]services.ArtifactUpload, 0, len(p.Attachments))
			for i, attachment := range p.Attachments {
				filename := strings.TrimSpace(attachment.Filename)
				if filename == "" {
					return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].filename is required", i))
				}
				encoded := strings.TrimSpace(attachment.Base64Data)
				if encoded == "" {
					return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].base64Data is required", i))
				}
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].base64Data is invalid", i))
				}
				uploads = append(uploads, services.ArtifactUpload{
					Filename:  filename,
					MimeType:  strings.TrimSpace(attachment.MimeType),
					SizeBytes: int64(len(decoded)),
					Data:      decoded,
				})
			}

			artifacts, err := artifactSvc.UploadSessionArtifacts(ctx, sessionID, uploads)
			if err != nil {
				return SessionUploadArtifactsResult{}, serrors.E(op, err)
			}

			out := make([]Artifact, 0, len(artifacts))
			for _, artifact := range artifacts {
				out = append(out, toArtifactDTO(artifact))
			}
			return SessionUploadArtifactsResult{Artifacts: out}, nil
		},
	})

	applet.AddProcedure(r, "bichat.artifact.update", applet.Procedure[ArtifactUpdateParams, ArtifactResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p ArtifactUpdateParams) (ArtifactResult, error) {
			const op serrors.Op = "bichat.rpc.artifact.update"

			artifactID, err := parseUUID(p.ID)
			if err != nil {
				return ArtifactResult{}, serrors.E(op, serrors.Invalid, err)
			}

			currentArtifact, err := artifactSvc.GetArtifact(ctx, artifactID)
			if err != nil {
				return ArtifactResult{}, serrors.E(op, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, currentArtifact.SessionID()); err != nil {
				return ArtifactResult{}, serrors.E(op, err)
			}

			updatedName := strings.TrimSpace(p.Name)
			if updatedName == "" {
				return ArtifactResult{}, serrors.E(op, serrors.KindValidation, "name is required")
			}

			updatedArtifact, err := artifactSvc.UpdateArtifact(
				ctx,
				artifactID,
				updatedName,
				strings.TrimSpace(p.Description),
			)
			if err != nil {
				return ArtifactResult{}, serrors.E(op, err)
			}

			return ArtifactResult{Artifact: toArtifactDTO(updatedArtifact)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.artifact.delete", applet.Procedure[SessionIDParams, OkResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.artifact.delete"

			artifactID, err := parseUUID(p.ID)
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.Invalid, err)
			}

			artifact, err := artifactSvc.GetArtifact(ctx, artifactID)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, artifact.SessionID()); err != nil {
				return OkResult{}, serrors.E(op, err)
			}

			if err := artifactSvc.DeleteArtifact(ctx, artifactID); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.archive", applet.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.archive"

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := chatSvc.ArchiveSession(ctx, sessionID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.unarchive", applet.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.unarchive"

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := chatSvc.UnarchiveSession(ctx, sessionID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.session.regenerateTitle", applet.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.regenerateTitle"

			sessionID, err := parseUUID(p.ID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			if err := chatSvc.GenerateSessionTitle(ctx, sessionID); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	})

	applet.AddProcedure(r, "bichat.question.submit", applet.Procedure[QuestionSubmitParams, SessionGetResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p QuestionSubmitParams) (SessionGetResult, error) {
			const op serrors.Op = "bichat.rpc.question.submit"

			sessionID, err := parseUUID(p.SessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			_, err = chatSvc.ResumeWithAnswer(ctx, services.ResumeRequest{
				SessionID:    sessionID,
				CheckpointID: p.CheckpointID,
				Answers:      p.Answers,
			})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			// Re-fetch session and messages to return updated state
			s, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			msgs, err := chatSvc.GetSessionMessages(ctx, sessionID, domain.ListOptions{Limit: 500})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			return SessionGetResult{
				Session:         toSessionDTO(s),
				Turns:           buildTurns(msgs),
				PendingQuestion: pendingQuestionFromMessages(msgs),
			}, nil
		},
	})

	applet.AddProcedure(r, "bichat.question.reject", applet.Procedure[QuestionCancelParams, SessionGetResult]{
		RequirePermissions: []string{"bichat.access"},
		Handler: func(ctx context.Context, p QuestionCancelParams) (SessionGetResult, error) {
			const op serrors.Op = "bichat.rpc.question.reject"

			sessionID, err := parseUUID(p.SessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if _, err := requireSessionOwner(ctx, chatSvc, sessionID); err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			_, err = chatSvc.RejectPendingQuestion(ctx, sessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			// Re-fetch to return updated state
			s, err := chatSvc.GetSession(ctx, sessionID)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			msgs, err := chatSvc.GetSessionMessages(ctx, sessionID, domain.ListOptions{Limit: 500})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}
			return SessionGetResult{
				Session:         toSessionDTO(s),
				Turns:           buildTurns(msgs),
				PendingQuestion: pendingQuestionFromMessages(msgs),
			}, nil
		},
	})

	return r
}
