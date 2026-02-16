package spotlight

import (
	"context"
	"errors"
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
	u, err := composables.UseUser(ctx)
	if err != nil {
		return Principal{}, err
	}
	if u == nil {
		return Principal{}, errors.New("spotlight principal resolver: missing authenticated user in context")
	}

	principal := Principal{UserID: fmt.Sprintf("%d", u.ID())}
	if req.UserID != "" && req.UserID != principal.UserID {
		logrus.WithFields(logrus.Fields{
			"request_user_id":       req.UserID,
			"authenticated_user_id": principal.UserID,
		}).Warn("spotlight request user id mismatch")
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

type BatchACLEvaluator interface {
	ACLEvaluator
	FilterAuthorized(ctx context.Context, req SearchRequest, hits []SearchHit) []SearchHit
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

	if principal, ok := principalFromRequest(req); ok {
		return canReadPolicy(policy, principal)
	}
	principal, err := e.resolver.Resolve(ctx, req)
	if err != nil {
		logrus.WithError(serrors.E(op, err)).WithFields(logrus.Fields{
			"tenant_id": req.TenantID.String(),
			"user_id":   req.UserID,
			"doc_id":    hit.Document.ID,
		}).Warn("spotlight principal resolver failed")
		return false
	}
	return canReadPolicy(policy, principal)
}

func (e *StrictACLEvaluator) FilterAuthorized(ctx context.Context, req SearchRequest, hits []SearchHit) []SearchHit {
	const op serrors.Op = "spotlight.StrictACLEvaluator.FilterAuthorized"

	if len(hits) == 0 {
		return []SearchHit{}
	}
	filtered := make([]SearchHit, 0, len(hits))
	principal, principalResolved := principalFromRequest(req)
	resolveFailed := false

	for _, hit := range hits {
		if hit.Document.TenantID != req.TenantID {
			continue
		}
		policy := hit.Document.Access
		if policy.Visibility == VisibilityPublic {
			filtered = append(filtered, hit)
			continue
		}
		if !principalResolved && !resolveFailed {
			resolved, err := e.resolver.Resolve(ctx, req)
			if err != nil {
				resolveFailed = true
				logrus.WithError(serrors.E(op, err)).WithFields(logrus.Fields{
					"tenant_id": req.TenantID.String(),
					"user_id":   req.UserID,
				}).Warn("spotlight principal resolver failed")
				continue
			}
			principal = resolved
			principalResolved = true
		}
		if !principalResolved {
			continue
		}
		if canReadPolicy(policy, principal) {
			filtered = append(filtered, hit)
		}
	}
	return filtered
}

func principalFromRequest(req SearchRequest) (Principal, bool) {
	if req.UserID == "" && len(req.Roles) == 0 && len(req.Permissions) == 0 {
		return Principal{}, false
	}
	return Principal{
		UserID:      req.UserID,
		Roles:       req.Roles,
		Permissions: req.Permissions,
	}, true
}

func canReadPolicy(policy AccessPolicy, principal Principal) bool {
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
