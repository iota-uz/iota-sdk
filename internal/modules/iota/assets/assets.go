package assets

import (
	"embed"

	"github.com/benbjohnson/hashfs"
)

//go:embed images/*
var fsys embed.FS

var FS = hashfs.NewFS(fsys)
