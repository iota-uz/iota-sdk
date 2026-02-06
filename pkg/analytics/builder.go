package analytics

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

// ViewOption is a functional option for configuring a View.
type ViewOption func(*View)

// TenantView creates a tenant-isolated view.
// It generates: SELECT * FROM public.{tableName} WHERE tenant_id = current_setting('app.tenant_id', true)::uuid
func TenantView(tableName string, opts ...ViewOption) View {
	v := View{
		Name:   tableName,
		Schema: "analytics",
		SQL: fmt.Sprintf(
			"SELECT * FROM public.%s WHERE tenant_id = current_setting('app.tenant_id', true)::uuid",
			tableName,
		),
	}
	for _, opt := range opts {
		opt(&v)
	}
	return v
}

// TenantViewFrom creates a tenant-isolated view with a custom name and source schema.
func TenantViewFrom(tableName, sourceSchema string, opts ...ViewOption) View {
	v := View{
		Name:   tableName,
		Schema: "analytics",
		SQL: fmt.Sprintf(
			"SELECT * FROM %s.%s WHERE tenant_id = current_setting('app.tenant_id', true)::uuid",
			sourceSchema, tableName,
		),
	}
	for _, opt := range opts {
		opt(&v)
	}
	return v
}

// CustomView creates a view with arbitrary SQL.
func CustomView(name string, sql string, opts ...ViewOption) View {
	v := View{
		Name:   name,
		Schema: "analytics",
		SQL:    sql,
	}
	for _, opt := range opts {
		opt(&v)
	}
	return v
}

// As overrides the view name.
func As(name string) ViewOption {
	return func(v *View) {
		v.Name = name
	}
}

// InSchema sets the target schema for the view.
func InSchema(schema string) ViewOption {
	return func(v *View) {
		v.Schema = schema
	}
}

// Desc sets the view-level description (COMMENT ON VIEW).
func Desc(d string) ViewOption {
	return func(v *View) {
		v.Description = d
	}
}

// ColumnComment adds a column-level comment (COMMENT ON COLUMN).
func ColumnComment(col, comment string) ViewOption {
	return func(v *View) {
		if v.ColumnComments == nil {
			v.ColumnComments = make(map[string]string)
		}
		v.ColumnComments[col] = comment
	}
}

// RequireAny sets permissions with OR logic.
func RequireAny(perms ...permission.Permission) ViewOption {
	return func(v *View) {
		v.Required = perms
		v.Logic = LogicAny
	}
}

// RequireAll sets permissions with AND logic.
func RequireAll(perms ...permission.Permission) ViewOption {
	return func(v *View) {
		v.Required = perms
		v.Logic = LogicAll
	}
}
