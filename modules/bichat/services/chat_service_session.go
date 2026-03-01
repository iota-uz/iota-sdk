// Package services provides this package.
package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// CreateSession creates a new chat session.
func (s *chatServiceImpl) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.CreateSession"

	title = strings.TrimSpace(title)
	spec := domain.SessionSpec{
		TenantID:    tenantID,
		OwnerUserID: userID,
		Title:       title,
	}

	var (
		session domain.Session
		err     error
	)
	if title == "" {
		session, err = domain.NewUntitledSession(spec)
	} else {
		session, err = domain.NewSession(spec)
	}
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

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
	if err != nil {
		return nil, serrors.E(op, err)
	}

	sessions := make([]domain.Session, 0, len(summaries))
	for _, summary := range summaries {
		sessions = append(sessions, summary.Session)
	}
	return sessions, nil
}

// CountUserSessions returns the total number of sessions for a user matching the same filter as ListUserSessions.
func (s *chatServiceImpl) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountUserSessions"

	count, err := s.CountAccessibleSessions(ctx, userID, opts)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (s *chatServiceImpl) ListAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.SessionSummary, error) {
	const op serrors.Op = "chatServiceImpl.ListAccessibleSessions"

	out, err := s.sessionAccess.ListAccessibleSessionSummaries(ctx, userID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (s *chatServiceImpl) CountAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountAccessibleSessions"

	count, err := s.sessionAccess.CountAccessibleSessions(ctx, userID, opts)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (s *chatServiceImpl) ListAllSessions(ctx context.Context, requestingUserID int64, opts domain.ListOptions, ownerUserID *int64) ([]domain.SessionSummary, error) {
	const op serrors.Op = "chatServiceImpl.ListAllSessions"

	out, err := s.sessionAccess.ListAllSessionSummaries(ctx, requestingUserID, opts, ownerUserID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (s *chatServiceImpl) CountAllSessions(ctx context.Context, opts domain.ListOptions, ownerUserID *int64) (int, error) {
	const op serrors.Op = "chatServiceImpl.CountAllSessions"

	count, err := s.sessionAccess.CountAllSessions(ctx, opts, ownerUserID)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

func (s *chatServiceImpl) ResolveSessionAccess(ctx context.Context, sessionID uuid.UUID, userID int64, allowReadAll bool) (domain.SessionAccess, error) {
	const op serrors.Op = "chatServiceImpl.ResolveSessionAccess"

	access, err := s.sessionAccess.ResolveSessionAccess(ctx, sessionID, userID)
	if err != nil {
		return domain.SessionAccess{}, serrors.E(op, err)
	}
	if access.CanRead {
		return access, nil
	}
	if allowReadAll {
		elevated, elevateErr := access.GrantReadAll()
		if elevateErr != nil {
			return domain.SessionAccess{}, serrors.E(op, elevateErr)
		}
		return elevated, nil
	}
	return access, nil
}

func (s *chatServiceImpl) ListSessionMembers(ctx context.Context, sessionID uuid.UUID) ([]domain.SessionMember, error) {
	const op serrors.Op = "chatServiceImpl.ListSessionMembers"

	members, err := s.sessionAccess.ListSessionMembers(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return members, nil
}

func (s *chatServiceImpl) UpsertSessionMember(ctx context.Context, command domain.SessionMemberUpsert) error {
	const op serrors.Op = "chatServiceImpl.UpsertSessionMember"

	if err := s.sessionAccess.UpsertSessionMember(ctx, command); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) RemoveSessionMember(ctx context.Context, command domain.SessionMemberRemoval) error {
	const op serrors.Op = "chatServiceImpl.RemoveSessionMember"

	if err := s.sessionAccess.RemoveSessionMember(ctx, command); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error) {
	const op serrors.Op = "chatServiceImpl.ListTenantUsers"

	users, err := s.sessionAccess.ListTenantUsers(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return users, nil
}

func (s *chatServiceImpl) GetTenantUser(ctx context.Context, userID int64) (domain.SessionUser, error) {
	const op serrors.Op = "chatServiceImpl.GetTenantUser"

	user, err := s.sessionAccess.GetTenantUser(ctx, userID)
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
	updated, err := session.Archive(time.Now())
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
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
	updated, err := session.Unarchive(time.Now())
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
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
	updated, err := session.Pin(time.Now())
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
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
	updated, err := session.Unpin(time.Now())
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// UpdateSessionTitle updates the title of a session.
func (s *chatServiceImpl) UpdateSessionTitle(ctx context.Context, sessionID uuid.UUID, title string) (domain.Session, error) {
	const op serrors.Op = "chatServiceImpl.UpdateSessionTitle"

	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	updated, err := session.Rename(title, time.Now())
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
	if err := s.chatRepo.UpdateSession(ctx, updated); err != nil {
		return nil, serrors.E(op, err)
	}
	return updated, nil
}

// DeleteSession deletes a session and all its messages.
func (s *chatServiceImpl) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.DeleteSession"

	if err := s.chatRepo.DeleteSession(ctx, sessionID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
