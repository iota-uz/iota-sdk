package mcp

import (
	"context"
	"slices"
	"strconv"

	"github.com/google/uuid"
)

// Principal is the caller identity established by a trusted transport (an
// OAuth access token, a chat session). Model arguments never populate it.
type Principal struct {
	TenantID uuid.UUID
	UserID   uint
	// Service names a non-user subject; it is exclusive with UserID.
	Service  string
	ClientID string
	Scopes   Scopes
}

func (p Principal) Valid() bool {
	return p.TenantID != uuid.Nil && (p.UserID != 0) != (p.Service != "")
}

func (p Principal) Subject() string {
	if p.Service != "" {
		return "service:" + p.Service
	}
	return "user:" + strconv.FormatUint(uint64(p.UserID), 10)
}

// Scopes is what the grant allows; it narrows RBAC and never widens it.
// First-party sessions carry AllScopes and are limited by RBAC alone.
type Scopes struct {
	all     bool
	granted []string
}

func AllScopes() Scopes { return Scopes{all: true} }

func GrantedScopes(scopes ...string) Scopes { return Scopes{granted: slices.Clone(scopes)} }

func (s Scopes) All() bool { return s.all }

func (s Scopes) Granted() []string { return slices.Clone(s.granted) }

// Missing returns the required scopes the grant lacks.
func (s Scopes) Missing(required []string) []string {
	if s.all {
		return nil
	}
	var missing []string
	for _, scope := range required {
		if !slices.Contains(s.granted, scope) {
			missing = append(missing, scope)
		}
	}
	return missing
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.Valid()
}

type requestIDKey struct{}

// WithRequestID tags the transport request so audit entries of nested calls
// can be joined to it.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
