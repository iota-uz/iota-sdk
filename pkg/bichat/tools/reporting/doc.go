// Package reporting provides BiChat discovery and execution of registered
// semantic report plans, complete typed query exports, and bounded previews.
// Inject an authorized QueryExecutor whose source row budget is larger than
// ReportMaxRows so truncation can be detected. Source definitions and product
// policies belong to the consumer, not this package.
package reporting
