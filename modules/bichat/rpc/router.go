package rpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/applets"
	modulepermissions "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func hasReadAllPermission(ctx context.Context) bool {
	return composables.CanUser(ctx, modulepermissions.BiChatReadAll) == nil
}

func requireSessionAccess(
	ctx context.Context,
	sessionQueries services.SessionQueries,
	sessionID string,
	requireWrite bool,
	requireManageMembers bool,
) (domain.Session, domain.SessionAccess, error) {
	const op serrors.Op = "bichat.rpc.requireSessionAccess"

	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, domain.SessionAccess{}, serrors.E(op, serrors.PermissionDenied, err)
	}

	parsedSessionID, err := parseUUID(sessionID)
	if err != nil {
		return nil, domain.SessionAccess{}, serrors.E(op, serrors.Invalid, err)
	}

	access, err := sessionQueries.ResolveSessionAccess(ctx, parsedSessionID, int64(user.ID()), hasReadAllPermission(ctx))
	if err != nil {
		return nil, domain.SessionAccess{}, serrors.E(op, err)
	}
	if err := access.Require(requireWrite, requireManageMembers); err != nil {
		return nil, domain.SessionAccess{}, serrors.E(op, serrors.PermissionDenied, err)
	}

	session, err := sessionQueries.GetSession(ctx, parsedSessionID)
	if err != nil {
		return nil, domain.SessionAccess{}, serrors.E(op, err)
	}

	return session, access, nil
}

func withSessionMeta(ctx context.Context, sessionQueries services.SessionQueries, session domain.Session, access domain.SessionAccess) Session {
	memberCount := 1
	members, err := sessionQueries.ListSessionMembers(ctx, session.ID())
	if err != nil {
		configuration.Use().Logger().WithError(err).Warn("failed to list session members for session metadata")
	} else {
		memberCount = len(members) + 1
	}

	owner, err := resolveSessionOwner(ctx, sessionQueries, session.UserID())
	if err != nil {
		configuration.Use().Logger().WithError(err).Warn("failed to resolve session owner metadata")
	}
	return toSessionDTOWithMeta(session, &owner, &access, memberCount)
}

func resolveSessionOwner(ctx context.Context, sessionQueries services.SessionQueries, ownerUserID int64) (domain.SessionUser, error) {
	const op serrors.Op = "bichat.rpc.resolveSessionOwner"

	user, err := sessionQueries.GetTenantUser(ctx, ownerUserID)
	if err != nil {
		return domain.SessionUser{ID: ownerUserID}, serrors.E(op, err)
	}

	return user, nil
}

// upsertSessionMember validates member upsert payload and delegates to service.
// Used by both add and updateRole RPC handlers to keep request validation in one place.
func upsertSessionMember(ctx context.Context, sessionCommands services.SessionCommands, session domain.Session, p SessionMembersUpsertParams) error {
	const op serrors.Op = "bichat.rpc.session.members.upsert"

	userID, err := parseUserID(p.UserID)
	if err != nil {
		return serrors.E(op, serrors.Invalid, err)
	}
	if userID == session.UserID() {
		return serrors.E(op, serrors.KindValidation, "owner cannot be added as a member")
	}

	role, err := domain.NewSessionMemberRole(p.Role)
	if err != nil {
		return serrors.E(op, serrors.KindValidation, err)
	}
	command, err := domain.NewSessionMemberUpsert(domain.SessionMemberUpsertSpec{
		SessionID: session.ID(),
		UserID:    userID,
		Role:      role,
	})
	if err != nil {
		return serrors.E(op, serrors.KindValidation, err)
	}
	if err := sessionCommands.UpsertSessionMember(ctx, command); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func Router(
	sessionCommands services.SessionCommands,
	sessionQueries services.SessionQueries,
	turnCommands services.TurnCommands,
	turnQueries services.TurnQueries,
	hitlCommands services.HITLCommands,
	artifactSvc services.ArtifactService,
) *applets.TypedRPCRouter {
	// Reserved for dedicated non-streaming turn command RPC procedures.
	_ = turnCommands
	r := applets.NewTypedRPCRouter()
	mustAdd := func(err error) {
		if err != nil {
			configuration.Use().Logger().WithError(err).Error("failed to register BiChat RPC procedure")
		}
	}
	mustAdd(applets.AddProcedure(r, "bichat.ping", applets.Procedure[PingParams, PingResult]{
		RequirePermissions: []string{"BiChat.Access"},
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
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.list", applets.Procedure[SessionListParams, SessionListResult]{
		RequirePermissions: []string{"BiChat.Access"},
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
			list, err := sessionQueries.ListAccessibleSessions(ctx, int64(user.ID()), opts)
			if err != nil {
				return SessionListResult{}, serrors.E(op, err)
			}
			// Total = full count matching filter (for pagination), not page size
			total, err := sessionQueries.CountAccessibleSessions(ctx, int64(user.ID()), domain.ListOptions{IncludeArchived: p.IncludeArchived})
			if err != nil {
				return SessionListResult{}, serrors.E(op, err)
			}
			hasMore := len(list) > requestedLimit
			if hasMore {
				list = list[:requestedLimit]
			}
			out := make([]Session, 0, len(list))
			for _, s := range list {
				out = append(out, toSessionDTOFromSummary(s))
			}
			return SessionListResult{Sessions: out, Total: total, HasMore: hasMore}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.listAll", applets.Procedure[SessionListAllParams, SessionListAllResult]{
		RequirePermissions: []string{"BiChat.ReadAll"},
		Handler: func(ctx context.Context, p SessionListAllParams) (SessionListAllResult, error) {
			const op serrors.Op = "bichat.rpc.session.listAll"

			user, err := composables.UseUser(ctx)
			if err != nil {
				return SessionListAllResult{}, serrors.E(op, serrors.PermissionDenied, err)
			}

			requestedLimit := p.Limit
			if requestedLimit <= 0 {
				requestedLimit = 50
			}

			var ownerUserID *int64
			if p.UserID != nil {
				value := strings.TrimSpace(*p.UserID)
				if value != "" {
					parsed, parseErr := strconv.ParseInt(value, 10, 64)
					if parseErr != nil {
						return SessionListAllResult{}, serrors.E(op, serrors.Invalid, parseErr)
					}
					ownerUserID = &parsed
				}
			}

			opts := domain.ListOptions{Limit: requestedLimit + 1, Offset: p.Offset, IncludeArchived: p.IncludeArchived}
			list, err := sessionQueries.ListAllSessions(ctx, int64(user.ID()), opts, ownerUserID)
			if err != nil {
				return SessionListAllResult{}, serrors.E(op, err)
			}
			total, err := sessionQueries.CountAllSessions(ctx, domain.ListOptions{IncludeArchived: p.IncludeArchived}, ownerUserID)
			if err != nil {
				return SessionListAllResult{}, serrors.E(op, err)
			}
			hasMore := len(list) > requestedLimit
			if hasMore {
				list = list[:requestedLimit]
			}

			out := make([]Session, 0, len(list))
			for _, summary := range list {
				out = append(out, toSessionDTOFromSummary(summary))
			}

			return SessionListAllResult{Sessions: out, Total: total, HasMore: hasMore}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.user.list", applets.Procedure[UserListParams, UserListResult]{
		RequirePermissions: []string{"BiChat.ReadAll"},
		Handler: func(ctx context.Context, _ UserListParams) (UserListResult, error) {
			const op serrors.Op = "bichat.rpc.user.list"

			users, err := sessionQueries.ListTenantUsers(ctx)
			if err != nil {
				return UserListResult{}, serrors.E(op, err)
			}
			out := make([]SessionUser, 0, len(users))
			for _, user := range users {
				out = append(out, toSessionUserDTO(user))
			}
			return UserListResult{Users: out}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.create", applets.Procedure[SessionCreateParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
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

			s, err := sessionCommands.CreateSession(ctx, tenantID, int64(user.ID()), p.Title)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.get", applets.Procedure[SessionGetParams, SessionGetResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionGetParams) (SessionGetResult, error) {
			const op serrors.Op = "bichat.rpc.session.get"
			s, access, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, false)
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			msgs, err := turnQueries.GetSessionMessages(ctx, s.ID(), domain.ListOptions{Limit: 500})
			if err != nil {
				return SessionGetResult{}, serrors.E(op, err)
			}

			pq := pendingQuestionFromMessages(msgs)

			return SessionGetResult{
				Session:         withSessionMeta(ctx, sessionQueries, s, access),
				Turns:           buildTurns(msgs),
				PendingQuestion: pq,
			}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.updateTitle", applets.Procedure[SessionUpdateTitleParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionUpdateTitleParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.updateTitle"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}

			s, err := sessionCommands.UpdateSessionTitle(ctx, session.ID(), p.Title)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.clear", applets.Procedure[SessionIDParams, SessionClearResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionClearResult, error) {
			const op serrors.Op = "bichat.rpc.session.clear"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionClearResult{}, serrors.E(op, err)
			}

			result, err := sessionCommands.ClearSessionHistory(ctx, session.ID())
			if err != nil {
				return SessionClearResult{}, serrors.E(op, err)
			}

			return SessionClearResult{
				Success:          result.Success,
				DeletedMessages:  result.DeletedMessages,
				DeletedArtifacts: result.DeletedArtifacts,
			}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.compact", applets.Procedure[SessionIDParams, AsyncRunAcceptedResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (AsyncRunAcceptedResult, error) {
			const op serrors.Op = "bichat.rpc.session.compact"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}

			result, err := sessionCommands.CompactSessionHistoryAsync(ctx, session.ID())
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}
			response := AsyncRunAcceptedResult{
				Accepted:  result.Accepted,
				Operation: string(result.Operation),
			}
			if result.Accepted {
				response.SessionID = result.SessionID.String()
				response.RunID = result.RunID.String()
				response.StartedAt = result.StartedAt.UnixMilli()
			}
			return response, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.delete", applets.Procedure[SessionIDParams, OkResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.session.delete"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			if err := sessionCommands.DeleteSession(ctx, session.ID()); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.pin", applets.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.pin"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := sessionCommands.PinSession(ctx, session.ID())
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.unpin", applets.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.unpin"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := sessionCommands.UnpinSession(ctx, session.ID())
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.artifacts", applets.Procedure[SessionArtifactsParams, SessionArtifactsResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionArtifactsParams) (SessionArtifactsResult, error) {
			const op serrors.Op = "bichat.rpc.session.artifacts"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, false, false)
			if err != nil {
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
			list, err := artifactSvc.GetSessionArtifacts(ctx, session.ID(), opts)
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
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.uploadArtifacts", applets.Procedure[SessionUploadArtifactsParams, SessionUploadArtifactsResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionUploadArtifactsParams) (SessionUploadArtifactsResult, error) {
			const op serrors.Op = "bichat.rpc.session.uploadArtifacts"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, true, false)
			if err != nil {
				return SessionUploadArtifactsResult{}, serrors.E(op, err)
			}
			if len(p.Attachments) == 0 {
				return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, "attachments are required")
			}
			const maxAttachments = 10
			if len(p.Attachments) > maxAttachments {
				return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("too many attachments: max %d", maxAttachments))
			}

			uploads := make([]services.ArtifactUpload, 0, len(p.Attachments))
			for i, attachment := range p.Attachments {
				if attachment.UploadID == nil || *attachment.UploadID <= 0 {
					return SessionUploadArtifactsResult{}, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].uploadId is required", i))
				}
				uploads = append(uploads, services.ArtifactUpload{
					UploadID: *attachment.UploadID,
				})
			}

			artifacts, err := artifactSvc.UploadSessionArtifacts(ctx, session.ID(), uploads)
			if err != nil {
				return SessionUploadArtifactsResult{}, serrors.E(op, err)
			}

			out := make([]Artifact, 0, len(artifacts))
			for _, artifact := range artifacts {
				out = append(out, toArtifactDTO(artifact))
			}
			return SessionUploadArtifactsResult{Artifacts: out}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.artifact.update", applets.Procedure[ArtifactUpdateParams, ArtifactResult]{
		RequirePermissions: []string{"BiChat.Access"},
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
			if _, _, err := requireSessionAccess(ctx, sessionQueries, currentArtifact.SessionID().String(), true, false); err != nil {
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
	}))

	mustAdd(applets.AddProcedure(r, "bichat.artifact.delete", applets.Procedure[ArtifactIDParams, OkResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p ArtifactIDParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.artifact.delete"

			artifactID, err := parseUUID(p.ID)
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.Invalid, err)
			}

			artifact, err := artifactSvc.GetArtifact(ctx, artifactID)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			if _, _, err := requireSessionAccess(ctx, sessionQueries, artifact.SessionID().String(), true, false); err != nil {
				return OkResult{}, serrors.E(op, err)
			}

			if err := artifactSvc.DeleteArtifact(ctx, artifactID); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.archive", applets.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.archive"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := sessionCommands.ArchiveSession(ctx, session.ID())
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.unarchive", applets.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.unarchive"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := sessionCommands.UnarchiveSession(ctx, session.ID())
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.regenerateTitle", applets.Procedure[SessionIDParams, SessionCreateResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionIDParams) (SessionCreateResult, error) {
			const op serrors.Op = "bichat.rpc.session.regenerateTitle"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.ID, false, true)
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			if err := sessionCommands.GenerateSessionTitle(ctx, session.ID()); err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			s, err := sessionQueries.GetSession(ctx, session.ID())
			if err != nil {
				return SessionCreateResult{}, serrors.E(op, err)
			}
			return SessionCreateResult{Session: toSessionDTO(s)}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.question.submit", applets.Procedure[QuestionSubmitParams, AsyncRunAcceptedResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p QuestionSubmitParams) (AsyncRunAcceptedResult, error) {
			const op serrors.Op = "bichat.rpc.question.submit"

			checkpointID := strings.TrimSpace(p.CheckpointID)
			if checkpointID == "" {
				return AsyncRunAcceptedResult{}, serrors.E(op, serrors.KindValidation, "checkpointId is required")
			}
			if len(p.Answers) == 0 {
				return AsyncRunAcceptedResult{}, serrors.E(op, serrors.KindValidation, "answers are required")
			}

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, true, false)
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}

			result, err := hitlCommands.ResumeWithAnswerAsync(ctx, services.ResumeRequest{
				SessionID:    session.ID(),
				CheckpointID: checkpointID,
				Answers:      p.Answers,
			})
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}
			response := AsyncRunAcceptedResult{
				Accepted:  result.Accepted,
				Operation: string(result.Operation),
			}
			if result.Accepted {
				response.SessionID = result.SessionID.String()
				response.RunID = result.RunID.String()
				response.StartedAt = result.StartedAt.UnixMilli()
			}
			return response, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.question.reject", applets.Procedure[QuestionCancelParams, AsyncRunAcceptedResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p QuestionCancelParams) (AsyncRunAcceptedResult, error) {
			const op serrors.Op = "bichat.rpc.question.reject"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, true, false)
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}
			result, err := hitlCommands.RejectPendingQuestionAsync(ctx, session.ID())
			if err != nil {
				return AsyncRunAcceptedResult{}, serrors.E(op, err)
			}
			response := AsyncRunAcceptedResult{
				Accepted:  result.Accepted,
				Operation: string(result.Operation),
			}
			if result.Accepted {
				response.SessionID = result.SessionID.String()
				response.RunID = result.RunID.String()
				response.StartedAt = result.StartedAt.UnixMilli()
			}
			return response, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.members.list", applets.Procedure[SessionMembersListParams, SessionMembersListResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionMembersListParams) (SessionMembersListResult, error) {
			const op serrors.Op = "bichat.rpc.session.members.list"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, false, false)
			if err != nil {
				return SessionMembersListResult{}, serrors.E(op, err)
			}

			members, err := sessionQueries.ListSessionMembers(ctx, session.ID())
			if err != nil {
				return SessionMembersListResult{}, serrors.E(op, err)
			}

			owner, listErr := resolveSessionOwner(ctx, sessionQueries, session.UserID())
			if listErr != nil {
				return SessionMembersListResult{}, serrors.E(op, listErr)
			}

			out := make([]SessionMember, 0, len(members)+1)
			out = append(out, SessionMember{
				User:      toSessionUserDTO(owner),
				Role:      strings.ToLower(domain.SessionMemberRoleOwner.String()),
				CreatedAt: session.CreatedAt().Format(time.RFC3339),
				UpdatedAt: session.UpdatedAt().Format(time.RFC3339),
			})
			for _, member := range members {
				out = append(out, SessionMember{
					User:      toSessionUserDTO(member.User),
					Role:      strings.ToLower(member.Role.String()),
					CreatedAt: member.CreatedAt.Format(time.RFC3339),
					UpdatedAt: member.UpdatedAt.Format(time.RFC3339),
				})
			}

			return SessionMembersListResult{Members: out}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.members.add", applets.Procedure[SessionMembersUpsertParams, OkResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionMembersUpsertParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.session.members.add"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, false, true)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			if err := upsertSessionMember(ctx, sessionCommands, session, p); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.members.updateRole", applets.Procedure[SessionMembersUpsertParams, OkResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionMembersUpsertParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.session.members.updateRole"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, false, true)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			if err := upsertSessionMember(ctx, sessionCommands, session, p); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	}))

	mustAdd(applets.AddProcedure(r, "bichat.session.members.remove", applets.Procedure[SessionMembersRemoveParams, OkResult]{
		RequirePermissions: []string{"BiChat.Access"},
		Handler: func(ctx context.Context, p SessionMembersRemoveParams) (OkResult, error) {
			const op serrors.Op = "bichat.rpc.session.members.remove"

			session, _, err := requireSessionAccess(ctx, sessionQueries, p.SessionID, false, true)
			if err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			userID, err := parseUserID(p.UserID)
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.Invalid, err)
			}
			if userID == session.UserID() {
				return OkResult{}, serrors.E(op, serrors.KindValidation, "cannot remove the session owner")
			}
			command, err := domain.NewSessionMemberRemoval(domain.SessionMemberRemovalSpec{
				SessionID: session.ID(),
				UserID:    userID,
			})
			if err != nil {
				return OkResult{}, serrors.E(op, serrors.KindValidation, err)
			}
			if err := sessionCommands.RemoveSessionMember(ctx, command); err != nil {
				return OkResult{}, serrors.E(op, err)
			}
			return OkResult{Ok: true}, nil
		},
	}))

	return r
}
