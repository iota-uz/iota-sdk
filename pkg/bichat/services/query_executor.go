package services

// QueryExecutorService and related types have been moved to pkg/bichat/sql package.
// This file is kept for backward compatibility but is deprecated.
//
// Deprecated: Use github.com/iota-uz/iota-sdk/pkg/bichat/sql.QueryExecutor instead.
// The unified sql package provides:
//   - sql.QueryExecutor for query execution
//   - sql.SchemaLister for listing schemas
//   - sql.SchemaDescriber for describing table schemas
//   - sql.QueryResult as the canonical result type
//
// Migration guide:
//   - Replace QueryExecutorService with sql.QueryExecutor
//   - Replace TableInfo/TableSchema with sql.TableInfo/sql.TableSchema
//   - Replace QueryResult with sql.QueryResult
//   - Update ExecuteQuery signature: timeoutMs int -> timeout time.Duration
