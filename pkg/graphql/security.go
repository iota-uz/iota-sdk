package graphql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/complexity"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Auth directive names. Every root field of Query, Mutation and Subscription
// must carry exactly one of them; see ValidateAuthDirectives.
const (
	DirectivePublic        = "public"
	DirectiveAuthenticated = "authenticated"
	DirectiveCan           = "can"

	// directiveCanPermissionsArg is the argument of @can that lists the
	// permissions a caller must hold (all of them).
	directiveCanPermissionsArg = "permissions"
)

// AuthDirectivesSDL declares the auth directives. Add it to every schema served
// through Handler and mark the directives skip_runtime in gqlgen.yml: Handler
// enforces them itself, so the generated DirectiveRoot does not need them.
//
//	directives:
//	  public: { skip_runtime: true }
//	  authenticated: { skip_runtime: true }
//	  can: { skip_runtime: true }
const AuthDirectivesSDL = `"""Field is reachable without a session."""
directive @public on FIELD_DEFINITION
"""Field requires an authenticated caller."""
directive @authenticated on FIELD_DEFINITION
"""Field requires an authenticated caller that holds every listed permission."""
directive @can(permissions: [String!]!) on FIELD_DEFINITION
`

// Defaults applied by NewHandler unless overridden with options.
const (
	DefaultComplexityLimit = 1000
	DefaultDepthLimit      = 15
	DefaultMaxBodyBytes    = 1 << 20 // 1 MiB for JSON bodies; multipart uses uploadsconfig limits.

	errDepthLimit = "DEPTH_LIMIT_EXCEEDED"
)

const errComplexityLimit = "COMPLEXITY_LIMIT_EXCEEDED"

func init() {
	// Protocol kind: the operation is rejected with HTTP 422.
	errcode.RegisterErrorType(errDepthLimit, errcode.KindProtocol)
	errcode.RegisterErrorType(errComplexityLimit, errcode.KindProtocol)
}

// operationLimitError is the cause of a limit rejection. It is an
// serrors.Invalid error and declares its GraphQL code through the
// serrorgql.PublicCarrier contract, so the presenter shows the code and limit
// instead of masking the rejection as internal.
type operationLimitError struct {
	cause *serrors.Error
	code  string
	limit int
}

func (e operationLimitError) Error() string       { return e.cause.Error() }
func (e operationLimitError) Unwrap() error       { return e.cause }
func (e operationLimitError) GraphQLCode() string { return e.code }
func (e operationLimitError) GraphQLExtensions() map[string]serrors.Value {
	return map[string]serrors.Value{"limit": serrors.Number(int64(e.limit))}
}

func newOperationLimitError(code, message string, limit int) *gqlerror.Error {
	err := &gqlerror.Error{
		Message: message,
		Err:     operationLimitError{cause: serrors.NewInvalid(message), code: code, limit: limit},
	}
	errcode.Set(err, code)
	return err
}

// complexityLimit mirrors gqlgen's FixedComplexityLimit but reports through
// newOperationLimitError.
type complexityLimit struct {
	limit int
	es    graphql.ExecutableSchema
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = &complexityLimit{}

func (*complexityLimit) ExtensionName() string { return "ComplexityLimit" }

func (c *complexityLimit) Validate(schema graphql.ExecutableSchema) error {
	if c.limit <= 0 {
		return errors.New("complexity limit must be positive")
	}
	c.es = schema
	return nil
}

func (c *complexityLimit) MutateOperationContext(_ context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	if rc == nil || rc.Operation == nil {
		return nil
	}
	if got := complexity.Calculate(c.es, rc.Operation, rc.Variables); got > c.limit {
		return newOperationLimitError(errComplexityLimit,
			fmt.Sprintf("operation has complexity %d, which exceeds the limit of %d", got, c.limit), c.limit)
	}
	return nil
}

// AuthDirectivesMode controls how a schema without auth directives is treated.
type AuthDirectivesMode int

const (
	// AuthDirectivesEnforce is the default: a root field without an auth
	// directive makes NewHandler/AddExecutor panic, and such a field is denied
	// at runtime.
	AuthDirectivesEnforce AuthDirectivesMode = iota
	// AuthDirectivesWarn is a transition mode: missing directives are logged
	// once when the executor is registered and undirected root fields stay
	// reachable. Fields that do carry a directive are still enforced.
	AuthDirectivesWarn
)

// Authorizer decides whether the caller in ctx may resolve a field guarded by
// @authenticated or @can. Return a serrors.Unauthenticated / PermissionDenied
// error (or any error) to deny.
type Authorizer interface {
	Authenticated(ctx context.Context) error
	Can(ctx context.Context, permissions []string) error
}

// ObjectAuthorizer is the object-level (ownership) hook. It runs after a
// protected field resolved to a non-nil value and may deny access to that
// particular object, e.g. a record that belongs to another tenant or customer.
// Protected fields are root fields that are not @public and nested fields that
// carry @authenticated or @can.
type ObjectAuthorizer interface {
	AuthorizeObject(ctx context.Context, field *graphql.FieldContext, obj any) error
}

// ObjectAuthorizerFunc adapts a function to ObjectAuthorizer.
type ObjectAuthorizerFunc func(ctx context.Context, field *graphql.FieldContext, obj any) error

func (f ObjectAuthorizerFunc) AuthorizeObject(ctx context.Context, field *graphql.FieldContext, obj any) error {
	return f(ctx, field, obj)
}

// Logger receives warnings produced in AuthDirectivesWarn mode.
type Logger interface {
	Warnf(format string, args ...any)
}

// WithAuthorizer sets the Authorizer behind @authenticated and @can. Without
// one every guarded field is denied.
func WithAuthorizer(a Authorizer) HandlerOption {
	return func(h *Handler) { h.authorizer = a }
}

// WithObjectAuthorizer sets the object-level ownership hook.
func WithObjectAuthorizer(a ObjectAuthorizer) HandlerOption {
	return func(h *Handler) { h.objectAuthorizer = a }
}

// WithAuthDirectivesMode switches between enforcing and transitional handling
// of root fields without auth directives.
func WithAuthDirectivesMode(mode AuthDirectivesMode) HandlerOption {
	return func(h *Handler) { h.authMode = mode }
}

// WithLogger sets the logger for AuthDirectivesWarn warnings.
func WithLogger(l Logger) HandlerOption {
	return func(h *Handler) { h.logger = l }
}

// WithComplexityLimit caps operation complexity. Zero or negative disables it.
func WithComplexityLimit(limit int) HandlerOption {
	return func(h *Handler) { h.complexityLimit = limit }
}

// WithDepthLimit caps selection depth (introspection subtrees excluded). Zero
// or negative disables it.
func WithDepthLimit(limit int) HandlerOption {
	return func(h *Handler) { h.depthLimit = limit }
}

// WithMaxBodyBytes caps JSON request bodies. Zero or negative disables it.
func WithMaxBodyBytes(limit int64) HandlerOption {
	return func(h *Handler) { h.maxBodyBytes = limit }
}

// WithGETTransport enables queries over HTTP GET. It is off by default: GET
// requests skip the JSON content-type CORS preflight and leak queries into logs.
func WithGETTransport() HandlerOption {
	return func(h *Handler) { h.enableGET = true }
}

// MissingAuthDirectivesError lists root fields without an auth directive and
// fields with conflicting or malformed ones.
type MissingAuthDirectivesError struct {
	Missing []string
	Invalid []string
}

func (e *MissingAuthDirectivesError) Error() string {
	var parts []string
	if len(e.Missing) > 0 {
		parts = append(parts, fmt.Sprintf(
			"root fields without @public, @authenticated or @can: %s", strings.Join(e.Missing, ", "),
		))
	}
	if len(e.Invalid) > 0 {
		parts = append(parts, "invalid auth directives: "+strings.Join(e.Invalid, "; "))
	}
	return "graphql: " + strings.Join(parts, "; ")
}

// ValidateAuthDirectives checks that every root field of Query, Mutation and
// Subscription carries exactly one auth directive and that @can lists at least
// one permission. Use it in a unit test to fail CI before deploy.
func ValidateAuthDirectives(schema *ast.Schema) error {
	if schema == nil {
		return nil
	}
	result := &MissingAuthDirectivesError{}
	for _, root := range []*ast.Definition{schema.Query, schema.Mutation, schema.Subscription} {
		if root == nil {
			continue
		}
		for _, field := range root.Fields {
			if strings.HasPrefix(field.Name, "__") {
				continue
			}
			name := root.Name + "." + field.Name
			rule, err := authRuleFor(field)
			switch {
			case err != nil:
				result.Invalid = append(result.Invalid, name+": "+err.Error())
			case rule.kind == authNone:
				result.Missing = append(result.Missing, name)
			}
		}
	}
	// Nested fields may carry directives too; they must be well-formed.
	for _, def := range schema.Types {
		if def == schema.Query || def == schema.Mutation || def == schema.Subscription {
			continue
		}
		for _, field := range def.Fields {
			if _, err := authRuleFor(field); err != nil {
				result.Invalid = append(result.Invalid, def.Name+"."+field.Name+": "+err.Error())
			}
		}
	}
	if len(result.Missing) == 0 && len(result.Invalid) == 0 {
		return nil
	}
	sort.Strings(result.Missing)
	sort.Strings(result.Invalid)
	return result
}

type authKind int

const (
	authNone authKind = iota
	authPublic
	authAuthenticated
	authCan
)

type authRule struct {
	kind        authKind
	permissions []string
}

func authRuleFor(field *ast.FieldDefinition) (authRule, error) {
	if field == nil {
		return authRule{}, nil
	}
	rule := authRule{}
	found := 0
	for _, d := range field.Directives {
		switch d.Name {
		case DirectivePublic:
			rule.kind = authPublic
		case DirectiveAuthenticated:
			rule.kind = authAuthenticated
		case DirectiveCan:
			rule.kind = authCan
			perms, err := canPermissions(d)
			if err != nil {
				return authRule{}, err
			}
			rule.permissions = perms
		default:
			continue
		}
		found++
	}
	if found > 1 {
		return authRule{}, errors.New("more than one of @public, @authenticated, @can")
	}
	return rule, nil
}

func canPermissions(d *ast.Directive) ([]string, error) {
	arg := d.Arguments.ForName(directiveCanPermissionsArg)
	if arg == nil || arg.Value == nil {
		return nil, errors.New("@can requires permissions")
	}
	var perms []string
	if kind := arg.Value.Kind; kind == ast.ListValue {
		for _, child := range arg.Value.Children {
			if child.Value == nil || child.Value.Kind != ast.StringValue || child.Value.Raw == "" {
				return nil, errors.New("@can permissions must be non-empty string literals")
			}
			perms = append(perms, child.Value.Raw)
		}
	} else if kind == ast.StringValue {
		// GraphQL input coercion allows a single value for a list argument.
		if arg.Value.Raw != "" {
			perms = append(perms, arg.Value.Raw)
		}
	} else {
		return nil, errors.New("@can permissions must be string literals")
	}
	if len(perms) == 0 {
		return nil, errors.New("@can requires at least one permission")
	}
	return perms, nil
}

// authExtension enforces auth directives on every resolved field.
type authExtension struct {
	authorizer       Authorizer
	objectAuthorizer ObjectAuthorizer
	mode             AuthDirectivesMode
	logger           Logger
}

var _ interface {
	graphql.HandlerExtension
	graphql.FieldInterceptor
} = authExtension{}

func (authExtension) ExtensionName() string { return "AuthDirectives" }

func (a authExtension) Validate(schema graphql.ExecutableSchema) error {
	err := ValidateAuthDirectives(schema.Schema())
	if err == nil {
		return nil
	}
	var missing *MissingAuthDirectivesError
	if a.mode == AuthDirectivesWarn && errors.As(err, &missing) && len(missing.Invalid) == 0 {
		if a.logger != nil {
			a.logger.Warnf("%v (AuthDirectivesWarn: these fields stay public until annotated)", err)
		}
		return nil
	}
	return err
}

func (a authExtension) InterceptField(ctx context.Context, next graphql.Resolver) (any, error) {
	fc := graphql.GetFieldContext(ctx)
	if fc == nil || fc.Field.Field == nil || fc.Field.Definition == nil || strings.HasPrefix(fc.Field.Name, "__") {
		return next(ctx)
	}
	isRoot := isRootField(fc)
	rule, err := authRuleFor(fc.Field.Definition)
	if err != nil {
		return nil, serrors.NewInternal(err.Error())
	}

	switch rule.kind {
	case authPublic:
		return next(ctx)
	case authNone:
		if !isRoot {
			return next(ctx)
		}
		if a.mode != AuthDirectivesWarn {
			return nil, serrors.NewPermissionDenied("field has no auth directive")
		}
	case authAuthenticated:
		if err := a.authenticated(ctx); err != nil {
			return nil, err
		}
	case authCan:
		if err := a.can(ctx, rule.permissions); err != nil {
			return nil, err
		}
	}

	res, err := next(ctx)
	if err != nil || a.objectAuthorizer == nil || isNil(res) {
		return res, err
	}
	if err := a.objectAuthorizer.AuthorizeObject(ctx, fc, res); err != nil {
		return nil, err
	}
	return res, nil
}

// isRootField reports whether fc is a field of the operation's root type. List
// element contexts have no field, so they are skipped while walking up.
func isRootField(fc *graphql.FieldContext) bool {
	for p := fc.Parent; p != nil; p = p.Parent {
		if p.Field.Field != nil {
			return false
		}
	}
	return true
}

func (a authExtension) authenticated(ctx context.Context) error {
	if a.authorizer == nil {
		return serrors.NewUnauthenticated("no authorizer configured")
	}
	return a.authorizer.Authenticated(ctx)
}

func (a authExtension) can(ctx context.Context, permissions []string) error {
	if err := a.authenticated(ctx); err != nil {
		return err
	}
	return a.authorizer.Can(ctx, permissions)
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	nillable := map[reflect.Kind]bool{
		reflect.Chan: true, reflect.Func: true, reflect.Interface: true,
		reflect.Map: true, reflect.Pointer: true, reflect.Slice: true,
	}
	return nillable[rv.Kind()] && rv.IsNil()
}

// depthLimit rejects operations whose selection depth exceeds limit before any
// resolver runs. Introspection subtrees (__schema, __type) are not counted.
type depthLimit struct {
	limit int
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = depthLimit{}

func (depthLimit) ExtensionName() string { return "DepthLimit" }

func (d depthLimit) Validate(graphql.ExecutableSchema) error {
	if d.limit <= 0 {
		return errors.New("depth limit must be positive")
	}
	return nil
}

func (d depthLimit) MutateOperationContext(_ context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	if rc == nil || rc.Operation == nil {
		return nil
	}
	depth := selectionDepth(rc.Operation.SelectionSet, rc.Doc, map[string]bool{}, d.limit)
	if depth > d.limit {
		return newOperationLimitError(errDepthLimit, fmt.Sprintf("operation exceeds the depth limit of %d", d.limit), d.limit)
	}
	return nil
}

// selectionDepth returns the depth of set, stopping early once it exceeds limit.
// visiting guards against fragment cycles (validation rejects them, but the
// limit must not rely on it).
func selectionDepth(set ast.SelectionSet, doc *ast.QueryDocument, visiting map[string]bool, limit int) int {
	maxDepth := 0
	for _, sel := range set {
		var depth int
		switch s := sel.(type) {
		case *ast.Field:
			if strings.HasPrefix(s.Name, "__") {
				continue
			}
			depth = 1
			if len(s.SelectionSet) > 0 {
				depth += selectionDepth(s.SelectionSet, doc, visiting, limit-1)
			}
		case *ast.InlineFragment:
			depth = selectionDepth(s.SelectionSet, doc, visiting, limit)
		case *ast.FragmentSpread:
			if visiting[s.Name] || doc == nil {
				continue
			}
			frag := doc.Fragments.ForName(s.Name)
			if frag == nil {
				continue
			}
			visiting[s.Name] = true
			depth = selectionDepth(frag.SelectionSet, doc, visiting, limit)
			delete(visiting, s.Name)
		}
		if depth > maxDepth {
			maxDepth = depth
		}
		if maxDepth > limit {
			return maxDepth
		}
	}
	return maxDepth
}
