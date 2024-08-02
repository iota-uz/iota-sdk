//go:build production
// +build production

package assets

import "embed"

//go:embed css/*.css
//go:embed images/*
var fsys embed.FS
