package core

import "embed"

//go:embed locales/*.json
var LocalesFS embed.FS

//go:embed migrations/*.sql
var MigrationsFs embed.FS
