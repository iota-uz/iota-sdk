package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var errSessionAccessRepoNotConfigured = errors.New(sessionAccessRepoNotConfiguredError)

// CreateSession creates a new chat session.
func (s *chatServiceImpl) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.CreateSession"

	session := domain.NewSession(
		domain.WithTenantID(tenantID),
		domain.WithUserID(userID),
		domain.WithTitle(title),
	)
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, serrors.E(op, err)
	}
	return session, nil
}

// GetSession retrieves a session by ID.
func (s *chatServiceImpl) GetSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.GetSession"
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return session, nil
}

// ListUserSessions lists all sessions for a user.
func (s *chatServiceImpl) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ListUserSessions"

	summaries, err := s.ListAccessibleSessions(ctx, userID, opts)
	if err == nil {
		sessions := make([]domain.Session, 0, len(summaries))
		for _, summary := range summaries {
			sessions = append(sessions, summary.Session)
		}
		return sessions, nil
	}
	if !isSessionAccessRepoNotConfiguredErr(err) {
		return nil, serrors.E(op, err)
	}

	configuration.Use().Logger().WithError(err).Warn("session access repository not configured; falling back to legacy session list")

	sessions, fallbackErr := s.chatRepo.ListUserSessions(ctx, userID, opts)
	if fallbackErr != nil {
		return nil, serrors.E(op, fallbackErr)
	}
	return sessions, nil
}

// CountUserSessions returns the total number of sessions for a user matching the same filter as ListUserSessions.
func (s *chatServiceImpl) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountUserSessions"

	count, err := s.CountAccessibleSessions(ctx, userID, opts)
	if err == nil {
		return count, nil
	}
	if !isSessionAccessRepoNotConfiguredErr(err) {
		return 0, serrors.E(op, err)
	}

	configuration.Use().Logger().WithError(err).Warn("session access repository not configured; falling back to legacy session count")

	count, fallbackErr := s.chatRepo.CountUserSessions(ctx, userID, opts)
	if fallbackErr != nil {
		return 0, serrors.E(op, fallbackErr)
	}
	return count, nil
}

func isSessionAccessRepoNotConfiguredErr(err error) bool {
	if errors.Is(err, errSessionAccessRepoNotConfigured) {
		return true
	}
	var sErr *serrors.Error
	if !errors.As(err, &sErr) {
		return false
	}
	return sErr.Kind == serrors.KindValidation && strings.EqualFold(sErr.Context, sessionAccessRepoNotConfiguredError)
}

func (s *chatServiceImpl) sessionAccessRepo() (domain.SessionAccessRepository, bool) {
	repo, ok := s.chatRepo.(domain.SessionAccessRepository)
	return repo, ok
}

func (s *chatServiceImpl) ListAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.SessionSummary, error) {
	const op serrors.Op = "chatServiceImpl.ListAccessibleSessions"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return nil, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	out, err := repo.ListAccessibleSessionSummaries(ctx, userID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (s *chatServiceImpl) CountAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountAccessibleSessions"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return 0, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	count, err := repo.CountAccessibleSessions(ctx, userID, opts)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (s *chatServiceImpl) ListAllSessions(ctx context.Context, requestingUserID int64, opts domain.ListOptions, ownerUserID *int64) ([]domain.SessionSummary, error) {
	const op serrors.Op = "chatServiceImpl.ListAllSessions"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return nil, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	out, err := repo.ListAllSessionSummaries(ctx, requestingUserID, opts, ownerUserID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (s *chatServiceImpl) CountAllSessions(ctx context.Context, opts domain.ListOptions, ownerUserID *int64) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountAllSessions"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return 0, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	count, err := repo.CountAllSessions(ctx, opts, ownerUserID)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (s *chatServiceImpl) ResolveSessionAccess(ctx context.Context, sessionID uuid.UUID, userID int64, allowReadAll bool) (domain.SessionAccess, error) {
	const op serrors.Op = "chatServiceImpl.ResolveSessionAccess"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		session, err := s.chatRepo.GetSession(ctx, sessionID)
		if err != nil {
			return domain.SessionAccess{}, serrors.E(op, err)
		}
		if session.UserID() == userID {
			return domain.NewSessionAccess(
				domain.SessionMemberRoleOwner,
				domain.SessionAccessSourceOwner,
			), nil
		}
		if allowReadAll {
			return domain.NewSessionAccess(
				domain.SessionMemberRoleReadAll,
				domain.SessionAccessSourcePermission,
			), nil
		}
		return domain.NewSessionAccess(
			domain.SessionMemberRoleNone,
			domain.SessionAccessSourceNone,
		), nil
	}

	access, err := repo.ResolveSessionAccess(ctx, sessionID, userID)
	if err != nil {
		return domain.SessionAccess{}, serrors.E(op, err)
	}
	if access.CanRead {
		return access, nil
	}
	if allowReadAll {
		return domain.NewSessionAccess(
			domain.SessionMemberRoleReadAll,
			domain.SessionAccessSourcePermission,
		), nil
	}
	return access, nil
}

func (s *chatServiceImpl) ListSessionMembers(ctx context.Context, sessionID uuid.UUID) ([]domain.SessionMember, error) {
	const op serrors.Op = "chatServiceImpl.ListSessionMembers"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return nil, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	members, err := repo.ListSessionMembers(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return members, nil
}

func (s *chatServiceImpl) UpsertSessionMember(ctx context.Context, sessionID uuid.UUID, userID int64, role domain.SessionMemberRole) error {
	const op serrors.Op = "chatServiceImpl.UpsertSessionMember"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}

	if !role.ValidMemberRole() {
		return serrors.E(op, serrors.KindValidation, "invalid member role")
	}
	if err := repo.UpsertSessionMember(ctx, sessionID, userID, role); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) RemoveSessionMember(ctx context.Context, sessionID uuid.UUID, userID int64) error {
	const op serrors.Op = "chatServiceImpl.RemoveSessionMember"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}
	if err := repo.RemoveSessionMember(ctx, sessionID, userID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error) {
	const op serrors.Op = "chatServiceImpl.ListTenantUsers"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return nil, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}
	users, err := repo.ListTenantUsers(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return users, nil
}

func (s *chatServiceImpl) GetTenantUser(ctx context.Context, userID int64) (domain.SessionUser, error) {
	const op serrors.Op = "chatServiceImpl.GetTenantUser"

	repo, ok := s.sessionAccessRepo()
	if !ok {
		return domain.SessionUser{}, serrors.E(op, serrors.KindValidation, errSessionAccessRepoNotConfigured)
	}
	user, err := repo.GetTenantUser(ctx, userID)
	if err != nil {
		return domain.SessionUser{}, serrors.E(op, err)
	}
	return user, nil
}

// ArchiveSession archives a session.
func (s *chatServiceImpl) ArchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.ArchiveSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateStatus(domain.SessionStatusArchived)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UnarchiveSession unarchives a session.
func (s *chatServiceImpl) UnarchiveSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UnarchiveSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateStatus(domain.SessionStatusActive)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// PinSession pins a session.
func (s *chatServiceImpl) PinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.PinSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdatePinned(true)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UnpinSession unpins a session.
func (s *chatServiceImpl) UnpinSession(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UnpinSession"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdatePinned(false)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UpdateSessionTitle updates the title of a session.
func (s *chatServiceImpl) UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UpdateSessionTitle"

	if title == "" {
		return nil, serrors.E(op, serrors.KindValidation, "title cannot be empty")
	}

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated := session.UpdateTitle(title)
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// DeleteSession deletes a session and all its messages.
func (s *chatServiceImpl) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.DeleteSession"

	// Repository handles cascade deletion of messages and attachments
	err := s.chatRepo.DeleteSession(ctx, sessionID)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}
