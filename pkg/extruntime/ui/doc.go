// Package ui is the UI slice of the extension runtime: it serves activated
// package pages, projects package navigation into the SDK navigation model,
// replaces screens and the whole shell through bindings, and exposes native
// list columns contributed by overlays.
//
// Everything reads one immutable Snapshot of activated client manifests and
// open screen/shell bindings obtained from a Source. The snapshot is cached
// per revision, so activation and revert take effect on the next request
// without a Go rebuild or restart, and Node is never called to render a page
// or the navigation.
//
// Boundaries:
//
//   - Source (implemented by the management slice for manifests, PGSource for
//     bindings) says what is active.
//   - ArtifactStore serves the immutable client chunks of a version.
//   - Invoker (implemented by *host.Host) executes commands and queries for
//     the browser as the session user.
//
// The package builds on the shell pieces of the SDK instead of duplicating
// them: pages render inside layouts.Authenticated, navigation reaches both the
// Templ sidebar and pkg/shellcontext through the request navigation items,
// and a shell binding is activated in the browser through the client host's
// ShellHost.
package ui
