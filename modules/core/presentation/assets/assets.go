// Package assets provides this package.
package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed css/*.css
//go:embed images/*
//go:embed fonts/*
//go:embed js/*
var FS embed.FS

// HashFS serves versioned core presentation assets, including the Spotlight runtime bundle.
var HashFS = hashfs.NewFS(&FS)
