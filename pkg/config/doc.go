// Package config provides a source-agnostic, DI-native configuration system.
//
// # Invariants
//
//   - A Source is immutable after Build returns. No Reload, no SIGHUP support.
//     The only reconfiguration path is process restart.
//   - No package-level globals. Callers compose providers, call Build once at
//     bootstrap, and pass the resulting Source to NewRegistry.
//   - Hot-reload is explicitly excluded from the design.
//
// # Typical usage
//
//	src, err := config.Build(
//	    env.New(".env", ".env.local"),
//	    yamlfile.New("config.yaml"),
//	)
//	r := config.NewRegistry(src)
//	dbCfg, err := config.Register[dbconfig.Config](r, "db")
package config
