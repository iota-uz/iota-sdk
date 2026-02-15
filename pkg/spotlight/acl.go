package spotlight

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

type Principal struct {
	UserID      string
	Roles       []string
	Permissions []string
}

type PrincipalResolver interface {
	Resolve(ctx context.Context, req SearchRequest) (Principal, error)
}

type ComposablesPrincipalResolver struct{}

func NewComposablesPrincipalResolver() *ComposablesPrincipalResolver {
	return &ComposablesPrincipalResolver{}
}

func (r *ComposablesPrincipalResolver) Resolve(ctx context.Context, req SearchRequest) (Principal, error) {
	principal := Principal{
		UserID: req.UserID,
	}
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return principal, err
	}
	if principal.UserID == "" {
		principal.UserID = fmt.Sprintf("%d", u.ID())
	}
	for _, role := range u.Roles() {
		principal.Roles = append(principal.Roles, role.Name())
	}
	for _, permission := range u.Permissions() {
		principal.Permissions = append(principal.Permissions, permission.Name())
	}
	return principal, nil
}

type ACLEvaluator interface {
	CanRead(ctx context.Context, req SearchRequest, hit SearchHit) bool
}

type StrictACLEvaluator struct {
	resolver PrincipalResolver
}

func NewStrictACLEvaluator(resolver PrincipalResolver) *StrictACLEvaluator {
	if resolver == nil {
		resolver = NewComposablesPrincipalResolver()
	}
	return &StrictACLEvaluator{resolver: resolver}
}

func (e *StrictACLEvaluator) CanRead(ctx context.Context, req SearchRequest, hit SearchHit) bool {
	const op serrors.Op = "spotlight.StrictACLEvaluator.CanRead"

	if hit.Document.TenantID != req.TenantID {
		return false
	}
	policy := hit.Document.Access
	if policy.Visibility == VisibilityPublic {
		return true
	}
	principal, err := e.resolver.Resolve(ctx, req)
	if err != nil {
		logrus.WithError(serrors.E(op, err)).WithFields(logrus.Fields{
			"tenant_id": req.TenantID.String(),
			"user_id":   req.UserID,
			"doc_id":    hit.Document.ID,
		}).Warn("spotlight principal resolver failed")
	}
	if policy.Visibility == VisibilityOwner {
		return principal.UserID != "" && policy.OwnerID != "" && policy.OwnerID == principal.UserID
	}

	for _, userID := range policy.AllowedUsers {
		if userID == principal.UserID {
			return true
		}
	}

	if len(policy.AllowedRoles) > 0 {
		for _, role := range principal.Roles {
			for _, allowedRole := range policy.AllowedRoles {
				if role == allowedRole {
					return true
				}
			}
		}
	}
	if len(policy.AllowedPermissions) > 0 {
		for _, permission := range principal.Permissions {
			for _, allowedPermission := range policy.AllowedPermissions {
				if permission == allowedPermission {
					return true
				}
			}
		}
	}

	return false
}
