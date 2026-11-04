// Package assets embeds Help Center browser assets.
package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed css/*.css
//go:embed js/lib/*.js
//go:embed licenses/*
var FS embed.FS

var HashFS = hashfs.NewFS(&FS)
