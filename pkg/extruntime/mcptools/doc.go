// Package mcptools publishes the extension management service
// (pkg/extruntime/manage) and the runtime's discoverability surface as MCP
// tools in the shared pkg/mcp registry.
//
// Every tool is a thin adapter: it decodes its input, derives the acting
// subject from the MCP principal and calls exactly one manage.Service method
// (or the host for ext_invoke). Rights, validation and audit therefore are the
// service's, the same as for the session UI and the /_ext/manage HTTP API.
//
// # Identity
//
// The principal is the real user behind a personal credential (an OAuth grant
// or API key of that user); the authorizer (rbacauth) re-reads the account on
// every call and puts it into the context. The subject's permissions are the
// account's current effective RBAC permissions; the grant's scopes only narrow
// what a credential may attempt and never add rights. Audit rows carry the
// user as actor and "mcp:<client>" as credential.
//
// # Policies
//
// Read tools require Extensions.Read in their Policy. Tools whose action the
// service audits (every write, check/build/test, export, kit download)
// declare only a scope: the service decides the permission, so a refused
// attempt leaves the same "denied" audit row as over HTTP. The permission each
// tool needs is published in Spec.Meta under MetaPermissions.
package mcptools
