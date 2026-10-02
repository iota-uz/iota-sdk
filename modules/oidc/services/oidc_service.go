// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type OIDCService struct {
	clientRepo      client.Repository
	authRequestRepo authrequest.Repository
}

func NewOIDCService(
	clientRepo client.Repository,
	authRequestRepo authrequest.Repository,
) *OIDCService {
	return &OIDCService{
		clientRepo:      clientRepo,
		authRequestRepo: authRequestRepo,
	}
}

// CompleteAuthRequest marks an auth request as authenticated
func (s *OIDCService) CompleteAuthRequest(
	ctx context.Context,
	authRequestID string,
	userID int,
	tenantID uuid.UUID,
) error {
	const op serrors.Op = "OIDCService.CompleteAuthRequest"

	// Parse auth request ID
	authID, err := uuid.Parse(authRequestID)
	if err != nil {
		return serrors.New(serrors.Invalid, "invalid auth request ID").WithOp(op).WithCause(err)
	}

	// Get auth request by ID
	authReq, err := s.authRequestRepo.GetByID(ctx, authID)
	if err != nil {
		return serrors.Wrap(op, err)
	}

	// Check if expired
	if authReq.IsExpired() {
		return serrors.New(serrors.Invalid, "auth request has expired").WithOp(op)
	}
	if authReq.IsAuthenticated() || authReq.IsCodeUsed() || authReq.Code() != nil {
		return serrors.New(serrors.Invalid, "auth request has already been consumed").WithOp(op)
	}

	// Complete authentication
	completedReq := authReq.CompleteAuthentication(userID, tenantID)

	// Update via repository
	if err := s.authRequestRepo.Update(ctx, completedReq); err != nil {
		return serrors.Wrap(op, err)
	}

	return nil
}

// GetAuthRequest retrieves an auth request by ID
func (s *OIDCService) GetAuthRequest(ctx context.Context, authRequestID string) (authrequest.AuthRequest, error) {
	const op serrors.Op = "OIDCService.GetAuthRequest"

	// Parse auth request ID
	authID, err := uuid.Parse(authRequestID)
	if err != nil {
		return nil, serrors.New(serrors.Invalid, "invalid auth request ID").WithOp(op).WithCause(err)
	}

	// Get from repository
	authReq, err := s.authRequestRepo.GetByID(ctx, authID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}

	return authReq, nil
}

func (s *OIDCService) ValidateAuthorizationRequest(ctx context.Context, authRequestID string) error {
	const op serrors.Op = "OIDCService.ValidateAuthorizationRequest"
	authReq, err := s.GetAuthRequest(ctx, authRequestID)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if authReq.IsExpired() || authReq.IsAuthenticated() || authReq.IsCodeUsed() || authReq.Code() != nil {
		return serrors.New(serrors.Invalid, "auth request is no longer valid").WithOp(op)
	}
	return nil
}
