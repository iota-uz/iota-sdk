// Package templates provides this package.
package templates

import "embed"

//go:embed pages/**
var FS embed.FS
