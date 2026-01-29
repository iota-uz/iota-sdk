package authorizers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// DefaultUploadsAuthorizer implements the UploadsAuthorizer interface with standard
// SDK permission checks using the RBAC system.
//
// This is the default implementation used by the SDK. Child projects can replace this
// with custom authorizers to implement tenant-specific upload policies, storage quotas,
// or integrate with external authorization services.
type DefaultUploadsAuthorizer struct{}

// Verify DefaultUploadsAuthorizer implements UploadsAuthorizer interface at compile time.
var _ types.UploadsAuthorizer = (*DefaultUploadsAuthorizer)(nil)

// NewDefaultUploadsAuthorizer creates a new instance of DefaultUploadsAuthorizer.
func NewDefaultUploadsAuthorizer() *DefaultUploadsAuthorizer {
	return &DefaultUploadsAuthorizer{}
}

// CanQueryUploads checks if the current user has permissions.UploadRead permission.
func (a *DefaultUploadsAuthorizer) CanQueryUploads(ctx context.Context) error {
	const op serrors.Op = "DefaultUploadsAuthorizer.CanQueryUploads"

	_, err := composables.UseUser(ctx)
	if err != nil {
		graphql.AddError(ctx, serrors.UnauthorizedGQLError(graphql.GetPath(ctx)))
		return serrors.E(op, err)
	}

	if err := composables.CanUser(ctx, permissions.UploadRead); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// CanUploadFile checks if the current user has permissions.UploadCreate permission.
// Note: This method does not check authentication as UploadFile mutation may be used
// for public uploads. Child projects should override this method if authentication
// is required.
func (a *DefaultUploadsAuthorizer) CanUploadFile(ctx context.Context) error {
	// No authorization check for public uploads
	// Child projects can override this to enforce authentication/permissions
	return nil
}

// CanUploadFileWithSlug checks if the current user has permissions.UploadCreate permission.
// This mutation requires authentication as it allows custom slugs.
func (a *DefaultUploadsAuthorizer) CanUploadFileWithSlug(ctx context.Context) error {
	const op serrors.Op = "DefaultUploadsAuthorizer.CanUploadFileWithSlug"

	_, err := composables.UseUser(ctx)
	if err != nil {
		graphql.AddError(ctx, serrors.UnauthorizedGQLError(graphql.GetPath(ctx)))
		return serrors.E(op, err)
	}

	if err := composables.CanUser(ctx, permissions.UploadCreate); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// CanDeleteUpload checks if the current user has permissions.UploadDelete permission.
func (a *DefaultUploadsAuthorizer) CanDeleteUpload(ctx context.Context, id int64) error {
	const op serrors.Op = "DefaultUploadsAuthorizer.CanDeleteUpload"

	_, err := composables.UseUser(ctx)
	if err != nil {
		graphql.AddError(ctx, serrors.UnauthorizedGQLError(graphql.GetPath(ctx)))
		return serrors.E(op, err)
	}

	if err := composables.CanUser(ctx, permissions.UploadDelete); err != nil {
		return serrors.E(op, err)
	}

	return nil
}
