package components

import (
	"embed"
)

//go:embed base/*.templ
//go:embed user/*.templ
var FS embed.FS
