package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed js/lib/*.js
//go:embed licenses/*
var FS embed.FS

var HashFS = hashfs.NewFS(&FS)
