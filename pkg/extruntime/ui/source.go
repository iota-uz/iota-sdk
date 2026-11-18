package ui

import (
	"context"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Binding slot kinds stored in extruntime.bindings.
const (
	SlotScreen = "screen"
	SlotShell  = "shell"
)

// ActivePackage is an activated package version with its client surface.
type ActivePackage struct {
	Package string
	Version string
	// ArtifactHash addresses the immutable artifact that holds the version's
	// client chunks; it is the first segment of every asset URL.
	ArtifactHash string
	Manifest     manifest.Manifest
	// Styles are the stylesheets of the version's client build, as paths
	// inside the artifact ("client/tailwind.css"), in link order. A source
	// that leaves them empty gets them read from the artifact's client
	// manifest (Runtime.Index).
	Styles []string
}

// styleURLs are the immutable URLs of the version's stylesheets.
func (p ActivePackage) styleURLs() []string {
	var urls []string
	for _, s := range p.Styles {
		if servableAsset(p.ArtifactHash, s) {
			urls = append(urls, AssetsPrefix+p.ArtifactHash+"/"+strings.TrimPrefix(s, "/"))
		}
	}
	return urls
}

// Binding is an open row of extruntime.bindings for a screen or the shell.
// For a screen, TargetID is the public route ID being replaced and Export
// names the replacing page of Package@Version; for the shell, Export names the
// package's shell renderer.
type Binding struct {
	ID         string
	SlotKind   string
	TargetID   string
	Package    string
	Version    string
	Export     string
	ActiveFrom time.Time
}

// Snapshot is the activated client surface at one moment.
type Snapshot struct {
	// Revision changes whenever the snapshot content changes. Sources should
	// return a value derived from the activations and bindings (for example
	// the greatest activation / binding ID), so the index is rebuilt only
	// when something changed.
	Revision string
	Packages []ActivePackage
	Bindings []Binding
}

// Source reports what is active. It is called on the request path and must
// answer from memory or a cheap read; it must not call Node.
type Source interface {
	Snapshot(ctx context.Context) (*Snapshot, error)
}

// ManifestSource lists the activated package versions. The management slice
// implements it over its activations and versions storage.
type ManifestSource interface {
	ActivePackages(ctx context.Context) (revision string, packages []ActivePackage, err error)
}

// NativeRoutes maps the public route ID of a native page to its path pattern
// (":name" marks a parameter). A screen binding may target a native route ID.
type NativeRoutes map[string]string
