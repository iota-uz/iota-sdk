// Package extensions embeds the extension SDK guides, so the application can
// serve them from its own Help Center next to the live API reference.
package extensions

import "embed"

// FS holds the guides: authoring.md (writing packages) and consumer.md
// (running the runtime in an application).
//
//go:embed authoring.md consumer.md
var FS embed.FS
