// Package config provides a source-agnostic, DI-native configuration system.
//
// # Invariants
//
//   - A [Source] is immutable after [Build] returns. There is no Reload, Watch,
//     OnChange, or Subscribe API — and none will be added. To pick up new
//     configuration values, restart the process.
//   - No package-level globals. Callers compose providers, call [Build] once at
//     bootstrap, and pass the resulting [Source] to [NewRegistry].
//   - Hot-reload is explicitly excluded from the design.
//
// # Typical usage
//
//	import (
//	    "github.com/iota-uz/iota-sdk/pkg/config"
//	    "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
//	    "github.com/iota-uz/iota-sdk/pkg/config/providers/yamlfile"
//	    "github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
//	)
//
//	src, err := config.Build(
//	    env.New(".env", ".env.local"),
//	    yamlfile.New("config.yaml"),
//	)
//	r := config.NewRegistry(src)
//	dbCfg, err := config.Register[dbconfig.Config](r)
package config
