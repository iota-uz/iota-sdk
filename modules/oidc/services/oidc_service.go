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
		return serrors.E(op, serrors.KindValidation, "invalid auth request ID", err)
	}

	// Get auth request by ID
	authReq, err := s.authRequestRepo.GetByID(ctx, authID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Check if expired
	if authReq.IsExpired() {
		return serrors.E(op, serrors.KindValidation, "auth request has expired")
	}

	// Complete authentication
	completedReq := authReq.CompleteAuthentication(userID, tenantID)

	// Update via repository
	if err := s.authRequestRepo.Update(ctx, completedReq); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// GetAuthRequest retrieves an auth request by ID
func (s *OIDCService) GetAuthRequest(ctx context.Context, authRequestID string) (authrequest.AuthRequest, error) {
	const op serrors.Op = "OIDCService.GetAuthRequest"

	// Parse auth request ID
	authID, err := uuid.Parse(authRequestID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid auth request ID", err)
	}

	// Get from repository
	authReq, err := s.authRequestRepo.GetByID(ctx, authID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return authReq, nil
}
