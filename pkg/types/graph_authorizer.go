package types

import "context"

// UploadsAuthorizer defines authorization logic for Upload-related GraphQL operations.
// This interface enables child projects to override authorization behavior for file uploads
// without modifying SDK code.
//
// # Usage Pattern for Child Projects
//
// Child projects can implement custom authorization logic by creating a custom authorizer:
//
//	type CustomUploadsAuthorizer struct {
//	    base types.UploadsAuthorizer
//	    tenantConfig TenantConfig
//	}
//
//	// Override CanUploadFile to add custom checks (e.g., storage quota)
//	func (a *CustomUploadsAuthorizer) CanUploadFile(ctx context.Context) error {
//	    // Check storage quota
//	    if err := a.checkStorageQuota(ctx); err != nil {
//	        return err
//	    }
//	    // Fall back to base implementation
//	    return a.base.CanUploadFile(ctx)
//	}
//
//	// Override CanUploadFileWithSlug to enforce slug naming conventions
//	func (a *CustomUploadsAuthorizer) CanUploadFileWithSlug(ctx context.Context) error {
//	    // Custom slug validation
//	    if err := a.validateSlugFormat(ctx); err != nil {
//	        return err
//	    }
//	    return a.base.CanUploadFileWithSlug(ctx)
//	}
//
// The interface allows:
// - Custom permission checks beyond RBAC
// - Tenant-specific upload policies (quotas, file types, size limits)
// - Integration with external authorization services
// - Backward compatibility with SDK defaults
type UploadsAuthorizer interface {
	// CanQueryUploads checks if the current user can list/query uploads.
	// Returns error if unauthorized.
	CanQueryUploads(ctx context.Context) error

	// CanUploadFile checks if the current user can upload a file.
	// Returns error if unauthorized.
	CanUploadFile(ctx context.Context) error

	// CanUploadFileWithSlug checks if the current user can upload a file with a custom slug.
	// Returns error if unauthorized.
	CanUploadFileWithSlug(ctx context.Context) error

	// CanDeleteUpload checks if the current user can delete an upload by ID.
	// Returns error if unauthorized.
	CanDeleteUpload(ctx context.Context, id int64) error
}

// UsersAuthorizer defines authorization logic for User-related GraphQL operations.
// This interface enables child projects to override authorization behavior for user queries
// without modifying SDK code.
//
// # Usage Pattern for Child Projects
//
// Child projects can implement custom authorization logic by creating a custom authorizer:
//
//	type CustomUsersAuthorizer struct {
//	    base types.UsersAuthorizer
//	    privacyPolicy PrivacyPolicy
//	}
//
//	// Override CanQueryUser to enforce privacy rules (e.g., only view users in same department)
//	func (a *CustomUsersAuthorizer) CanQueryUser(ctx context.Context, id int64) error {
//	    currentUser := composables.UseUser(ctx)
//	    targetUser, _ := a.userService.GetByID(ctx, uint(id))
//
//	    // Check department-level access
//	    if currentUser.DepartmentID != targetUser.DepartmentID {
//	        return serrors.E(op, serrors.KindPermission, "cannot view users outside your department")
//	    }
//
//	    return a.base.CanQueryUser(ctx, id)
//	}
//
//	// Override CanQueryUsers to enforce row-level security
//	func (a *CustomUsersAuthorizer) CanQueryUsers(ctx context.Context) error {
//	    // Custom access control logic
//	    if err := a.checkDataAccessPolicy(ctx); err != nil {
//	        return err
//	    }
//	    return a.base.CanQueryUsers(ctx)
//	}
//
// The interface allows:
// - Row-level security policies
// - Hierarchical/departmental access control
// - Privacy-preserving data access
// - Integration with external identity providers
// - Backward compatibility with SDK defaults
type UsersAuthorizer interface {
	// CanQueryUser checks if the current user can query a specific user by ID.
	// Returns error if unauthorized.
	CanQueryUser(ctx context.Context, id int64) error

	// CanQueryUsers checks if the current user can list/query users.
	// Returns error if unauthorized.
	CanQueryUsers(ctx context.Context) error
}
