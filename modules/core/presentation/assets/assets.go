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

var HashFS = hashfs.NewFS(&FS)
