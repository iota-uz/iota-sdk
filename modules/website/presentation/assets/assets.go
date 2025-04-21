package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed js/*
var FS embed.FS

var HashFS = hashfs.NewFS(&FS)
