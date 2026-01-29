package authorizers

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// DefaultUsersAuthorizer implements the UsersAuthorizer interface with basic
// user existence checks.
//
// This is the default implementation used by the SDK. Child projects can replace this
// with custom authorizers to implement row-level security, departmental access control,
// or integrate with external authorization services.
//
// The default implementation allows any authenticated user to query users, with the
// assumption that business logic in the service layer will enforce appropriate filtering.
type DefaultUsersAuthorizer struct {
	userService *services.UserService
}

// Verify DefaultUsersAuthorizer implements UsersAuthorizer interface at compile time.
var _ types.UsersAuthorizer = (*DefaultUsersAuthorizer)(nil)

// NewDefaultUsersAuthorizer creates a new instance of DefaultUsersAuthorizer.
func NewDefaultUsersAuthorizer(userService *services.UserService) *DefaultUsersAuthorizer {
	return &DefaultUsersAuthorizer{
		userService: userService,
	}
}

// CanQueryUser checks if the requested user exists.
// This default implementation does not enforce permission checks, allowing any authenticated
// user to query other users. Child projects should override this method to implement
// custom authorization logic (e.g., department-level access, privacy policies).
func (a *DefaultUsersAuthorizer) CanQueryUser(ctx context.Context, id int64) error {
	const op serrors.Op = "DefaultUsersAuthorizer.CanQueryUser"

	// Check if user exists
	_, err := a.userService.GetByID(ctx, uint(id))
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// CanQueryUsers checks if the current user can list users.
// This default implementation does not enforce permission checks, allowing any authenticated
// user to query the user list. Child projects should override this method to implement
// custom authorization logic (e.g., RBAC, row-level security).
func (a *DefaultUsersAuthorizer) CanQueryUsers(ctx context.Context) error {
	// No authorization check by default
	// Child projects can override this to enforce permissions
	return nil
}
