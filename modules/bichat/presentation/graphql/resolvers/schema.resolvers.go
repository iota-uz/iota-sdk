package resolvers

// Custom resolver implementations live here. generated/generated.go is fully
// overwritten by gqlgen, so artifact (and any other custom) resolvers are kept in this file.

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/model"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DeleteArtifact is the resolver for the deleteArtifact field.
func (r *mutationResolver) DeleteArtifact(ctx context.Context, id string) (bool, error) {
	const op serrors.Op = "Resolver.DeleteArtifact"
	artifactID, err := uuid.Parse(id)
	if err != nil {
		return false, serrors.E(op, serrors.KindValidation, "invalid artifact ID", err)
	}

	// Verify session ownership before deletion
	a, err := r.artifactService.GetArtifact(ctx, artifactID)
	if err != nil {
		return false, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return false, serrors.E(op, serrors.PermissionDenied, "unauthorized", err)
	}
	session, err := r.chatService.GetSession(ctx, a.SessionID())
	if err != nil {
		return false, serrors.E(op, err)
	}
	if session.UserID() != int64(user.ID()) {
		return false, serrors.E(op, serrors.PermissionDenied, "artifact does not belong to user")
	}

	if err := r.artifactService.DeleteArtifact(ctx, artifactID); err != nil {
		return false, serrors.E(op, err)
	}
	return true, nil
}

// UpdateArtifact is the resolver for the updateArtifact field.
func (r *mutationResolver) UpdateArtifact(ctx context.Context, id string, name *string, description *string) (*model.Artifact, error) {
	const op serrors.Op = "Resolver.UpdateArtifact"
	artifactID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid artifact ID", err)
	}

	// Verify session ownership before update
	a, err := r.artifactService.GetArtifact(ctx, artifactID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "unauthorized", err)
	}
	session, err := r.chatService.GetSession(ctx, a.SessionID())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID() != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "artifact does not belong to user")
	}

	n, d := "", ""
	if name != nil {
		n = *name
	}
	if description != nil {
		d = *description
	}
	a, err = r.artifactService.UpdateArtifact(ctx, artifactID, n, d)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return toGraphQLArtifact(a), nil
}

// Artifact is the resolver for the artifact field.
func (r *queryResolver) Artifact(ctx context.Context, id string) (*model.Artifact, error) {
	const op serrors.Op = "Resolver.Artifact"
	artifactID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid artifact ID", err)
	}

	// Get artifact
	a, err := r.artifactService.GetArtifact(ctx, artifactID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Verify session ownership
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "unauthorized", err)
	}
	session, err := r.chatService.GetSession(ctx, a.SessionID())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID() != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "artifact does not belong to user")
	}

	return toGraphQLArtifact(a), nil
}

// Artifacts is the resolver for the Session.artifacts field.
func (r *sessionResolver) Artifacts(ctx context.Context, obj *model.Session, limit *int, offset *int, types []string) ([]*model.Artifact, error) {
	const op serrors.Op = "Resolver.Artifacts"
	sessionID, err := uuid.Parse(obj.ID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Verify session ownership
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "unauthorized", err)
	}
	session, err := r.chatService.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID() != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "session does not belong to user")
	}

	l, o := 50, 0
	if limit != nil && *limit > 0 {
		l = *limit
	}
	if offset != nil && *offset >= 0 {
		o = *offset
	}

	// Convert []string types to []ArtifactType
	var artifactTypes []domain.ArtifactType
	for _, t := range types {
		artifactTypes = append(artifactTypes, domain.ArtifactType(t))
	}

	opts := domain.ListOptions{Limit: l, Offset: o, Types: artifactTypes}
	list, err := r.artifactService.GetSessionArtifacts(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	result := make([]*model.Artifact, len(list))
	for i, a := range list {
		result[i] = toGraphQLArtifact(a)
	}
	return result, nil
}
