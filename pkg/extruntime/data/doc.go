// Package data is the typed data layer of the extension runtime: DDL for the
// entity and overlay resources of a package manifest, a query compiler with
// access rules, and the spi.DataStore implementation.
//
// Physical model. Each package namespace owns a PostgreSQL schema
// "x_<namespace>", each entity one table with tenant_id, id, typed columns,
// created_at, updated_at and row_version; an overlay is a separate table keyed
// by (tenant_id, base_key). A field gets an immutable physical column the
// first time it is applied; field keys, including renamedFrom aliases, map to
// that column per package version (extruntime.entity_schema_versions). A
// caller's data calls resolve keys through the version it runs as
// (spi.Scope.Caller), so a workflow pinned to an older version keeps working.
//
// Evolution. Plan accepts added optional or defaulted fields, indexes, field
// renames (metadata only) and widening changes. Anything that could lose or
// invalidate data is refused with a *SchemaError that lists every reason.
// Tables and columns are never dropped.
package data
