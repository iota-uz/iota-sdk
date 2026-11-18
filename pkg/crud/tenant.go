package crud

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DefaultTenantField is the column name used by TenantScoped when no
// explicit column is given.
const DefaultTenantField = "tenant_id"

// TenantScopedSchema is an optional interface implemented by schemas created
// with TenantScoped or WithTenantField. When a schema is tenant scoped, the
// default repository:
//   - adds `<tenant field> = <tenant from context>` to every SELECT, UPDATE and
//     DELETE WHERE clause (Get, Exists, GetAll, List, Count, Update, Delete);
//   - always writes the tenant from context into the tenant column on Create,
//     ignoring whatever value the entity carries;
//   - never changes the tenant column on Update.
//
// The tenant is read with composables.UseTenantID; operations fail when the
// context carries no tenant.
type TenantScopedSchema interface {
	// TenantField returns the tenant column name, or "" when the schema is
	// not tenant scoped.
	TenantField() string
}

// TenantScoped marks the schema as tenant scoped by the DefaultTenantField
// ("tenant_id") column. See TenantScopedSchema.
func TenantScoped[TEntity any]() SchemaOption[TEntity] {
	return WithTenantField[TEntity](DefaultTenantField)
}

// WithTenantField marks the schema as tenant scoped by the given column.
// The column must be declared in the schema fields. See TenantScopedSchema.
func WithTenantField[TEntity any](fieldName string) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.tenantField = fieldName
	}
}

// SchemaTenantField returns the tenant column of a tenant-scoped schema, or ""
// when the schema is not tenant scoped.
func SchemaTenantField[TEntity any](s Schema[TEntity]) string {
	if ts, ok := s.(TenantScopedSchema); ok {
		return ts.TenantField()
	}
	return ""
}

// tenantScope resolves the tenant field and the tenant ID from context.
// It returns a nil field when the schema is not tenant scoped.
func tenantScope[TEntity any](ctx context.Context, s Schema[TEntity]) (Field, any, error) {
	const op serrors.Op = "crud.tenantScope"

	name := SchemaTenantField(s)
	if name == "" {
		return nil, nil, nil
	}
	field, err := s.Fields().Field(name)
	if err != nil {
		return nil, nil, serrors.NewInvalid("tenant field is not declared in schema " + s.Name()).WithOp(op).WithCause(err)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, nil, serrors.NewPermissionDenied("tenant-scoped schema " + s.Name() + " requires a tenant in context").WithOp(op).WithCause(err)
	}
	if field.Type() == StringFieldType {
		return field, tenantID.String(), nil
	}
	return field, tenantID, nil
}

// scopeKeyValues appends the context tenant to key values used in a WHERE
// clause. Any caller-supplied value for the tenant field is replaced.
func (r *repository[TEntity]) scopeKeyValues(ctx context.Context, keyValues []FieldValue) ([]FieldValue, error) {
	field, tenantID, err := tenantScope(ctx, r.schema)
	if err != nil || field == nil {
		return keyValues, err
	}
	scoped := make([]FieldValue, 0, len(keyValues)+1)
	for _, kv := range keyValues {
		if kv.Field().Name() != field.Name() {
			scoped = append(scoped, kv)
		}
	}
	return append(scoped, field.Value(tenantID)), nil
}

// scopeValues forces the tenant field in values written by Create to the
// context tenant, adding it when absent.
func (r *repository[TEntity]) scopeValues(ctx context.Context, values []FieldValue) ([]FieldValue, error) {
	return r.scopeKeyValues(ctx, values)
}

// scopeParams returns a copy of params with an equality filter on the tenant
// field.
func (r *repository[TEntity]) scopeParams(ctx context.Context, params *FindParams) (*FindParams, error) {
	if params == nil {
		params = &FindParams{}
	}
	field, tenantID, err := tenantScope(ctx, r.schema)
	if err != nil || field == nil {
		return params, err
	}
	scoped := *params
	scoped.Filters = make([]Filter, 0, len(params.Filters)+1)
	scoped.Filters = append(scoped.Filters, params.Filters...)
	scoped.Filters = append(scoped.Filters, Filter{Column: field.Name(), Filter: repo.Eq(tenantID)})
	return &scoped, nil
}
