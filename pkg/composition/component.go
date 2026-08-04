package composition

import "embed"

type Descriptor struct {
	Name         string
	Capabilities []Capability
	Requires     []string
}

// Component is the unit of composition. Every component must declare its
// metadata via Descriptor, the locale files it ships (or nil for none) via
// LocaleFS, and may register additional providers / controllers / hooks
// in Build.
//
// LocaleFS is read by the composition engine at Compile time without
// instantiating Build, which lets tooling (CLI checks, doc generators)
// extract locales without booting the full application.
type Component interface {
	Descriptor() Descriptor
	LocaleFS() []*embed.FS
	Build(*Builder) error
}
