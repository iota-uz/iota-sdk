package components

import (
	"embed"
)

//go:embed base/*
//go:embed user/*
var FS embed.FS
