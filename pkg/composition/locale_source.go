package composition

import "embed"

// LocaleSource is an optional interface for Components that ship locale TOML/JSON
// files. Tooling that only needs to read locales (such as the check_tr_keys
// validator) can extract them without booting the application or running
// Build — which is important for fast, dependency-free CLI checks.
//
// Implementations should return a stable slice of the package-level embed.FS
// values that hold `presentation/locales/*.{toml,json}` files.
type LocaleSource interface {
	LocaleFS() []*embed.FS
}
