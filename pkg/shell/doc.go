// Package shell is the application-side contract of the sandboxed shell
// capability: request/result types, input and artifact interfaces, and an HTTP
// executor for the separately deployed worker (see the worker module under
// pkg/shell/worker).
//
// The package imports only the standard library, so an application can wire
// the capability without pulling in the worker runtime or its Go toolchain.
//
// A Runner validates a Request, resolves files[].file_ref through the
// application's FileResolver (which owns user and tenant checks), runs the
// script on an Executor and publishes everything left in /output through the
// application's ArtifactPublisher. Only published file_refs reach the caller.
//
// Errors before execution are *Error values with the codes from the contract
// (invalid_input, forbidden, limit_exceeded, busy, dependency_unavailable). A
// non-zero shell exit status is a regular Result, never an error.
package shell
