package templates

import (
	"embed"
)

//go:embed pages/*
//go:embed icons/*
//go:embed layouts/*
var FS embed.FS
