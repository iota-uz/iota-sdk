package evaluation

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"time"
)

// EvaluationContext provides context for dashboard evaluation
type EvaluationContext struct {
	TimeRange lens.TimeRange
	Variables map[string]any
	User      UserContext
	Options   EvaluationOptions
}

// UserContext represents user information for permission-based filtering
type UserContext struct {
	ID          string
	Username    string
	Roles       []string
	Permissions []string
	Tenant      string
}

// EvaluationOptions contains options for evaluation
type EvaluationOptions struct {
	InterpolateVariables bool
	ValidateQueries      bool
	CalculateLayout      bool
	EnableCaching        bool
	CacheTTL             time.Duration
}

// DefaultEvaluationOptions returns default evaluation options
func DefaultEvaluationOptions() EvaluationOptions {
	return EvaluationOptions{
		InterpolateVariables: true,
		ValidateQueries:      true,
		CalculateLayout:      true,
		EnableCaching:        false,
		CacheTTL:             5 * time.Minute,
	}
}

// NewEvaluationContext creates a new evaluation context
func NewEvaluationContext(timeRange lens.TimeRange, variables map[string]any) *EvaluationContext {
	return &EvaluationContext{
		TimeRange: timeRange,
		Variables: variables,
		User:      UserContext{},
		Options:   DefaultEvaluationOptions(),
	}
}

// WithUser sets the user context
func (ctx *EvaluationContext) WithUser(user UserContext) *EvaluationContext {
	ctx.User = user
	return ctx
}

// WithOptions sets the evaluation options
func (ctx *EvaluationContext) WithOptions(options EvaluationOptions) *EvaluationContext {
	ctx.Options = options
	return ctx
}

// GetVariable returns a variable value by name
func (ctx *EvaluationContext) GetVariable(name string) (any, bool) {
	value, exists := ctx.Variables[name]
	return value, exists
}

// SetVariable sets a variable value
func (ctx *EvaluationContext) SetVariable(name string, value any) {
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]any)
	}
	ctx.Variables[name] = value
}

// HasPermission checks if the user has a specific permission
func (ctx *EvaluationContext) HasPermission(permission string) bool {
	for _, p := range ctx.User.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasRole checks if the user has a specific role
func (ctx *EvaluationContext) HasRole(role string) bool {
	for _, r := range ctx.User.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Clone creates a copy of the evaluation context
func (ctx *EvaluationContext) Clone() *EvaluationContext {
	variables := make(map[string]any)
	for k, v := range ctx.Variables {
		variables[k] = v
	}

	permissions := make([]string, len(ctx.User.Permissions))
	copy(permissions, ctx.User.Permissions)

	roles := make([]string, len(ctx.User.Roles))
	copy(roles, ctx.User.Roles)

	return &EvaluationContext{
		TimeRange: ctx.TimeRange,
		Variables: variables,
		User: UserContext{
			ID:          ctx.User.ID,
			Username:    ctx.User.Username,
			Roles:       roles,
			Permissions: permissions,
			Tenant:      ctx.User.Tenant,
		},
		Options: ctx.Options,
	}
}
