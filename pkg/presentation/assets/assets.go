package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed css/*.css
//go:embed images/*
//go:embed fonts/*
//go:embed js/*
var fsys embed.FS

var FS = hashfs.NewFS(fsys)
