package composition

import (
	"embed"
)

// ContributeMigrations attaches one or more SQL migration embed.FS bundles to
// the component. The bundles can be retrieved from the container via
// (*Container).MigrationFiles(). A binary like cmd/migrate compiles with
// CapabilityMigrate to materialize migration contributions without booting
// the HTTP/worker stacks.
func ContributeMigrations(builder *Builder, fs ...*embed.FS) {
	if builder == nil || len(fs) == 0 {
		return
	}
	captured := append([]*embed.FS(nil), fs...)
	contributeMigrations(builder, func(*Container) ([]*embed.FS, error) {
		return captured, nil
	})
}

func contributeMigrations(builder *Builder, factory func(*Container) ([]*embed.FS, error)) {
	if builder == nil || factory == nil {
		return
	}
	builder.migrationFactories = append(builder.migrationFactories, namedFactory[[]*embed.FS]{
		component: builder.descriptor.Name,
		label:     "migrations",
		factory:   factory,
	})
}
