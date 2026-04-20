package applet

import (
	"fmt"
	"os"
	"slices"
	"strings"

	appletsconfig "github.com/iota-uz/applets/config"
	appletenginehandlers "github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
)

type BackendFactory[T any] struct {
	ID           string
	ValidateFunc func(BuildContext) error
	BuildFunc    func(*Container) (T, error)
}

func (f BackendFactory[T]) BackendID() string {
	return strings.TrimSpace(f.ID)
}

func (f BackendFactory[T]) Validate(ctx BuildContext) error {
	if f.ValidateFunc == nil {
		return nil
	}
	return f.ValidateFunc(ctx)
}

func (f BackendFactory[T]) Build(container *Container) (T, error) {
	if f.BuildFunc == nil {
		var zero T
		return zero, fmt.Errorf("composition applet: backend %q requires a build function", f.BackendID())
	}
	return f.BuildFunc(container)
}

type BackendRegistry struct {
	KV      *TypedBackendRegistry[appletenginehandlers.KVStore]
	DB      *TypedBackendRegistry[appletenginehandlers.DBStore]
	Jobs    *TypedBackendRegistry[appletenginehandlers.JobsStore]
	Files   *TypedBackendRegistry[appletenginehandlers.FilesStore]
	Secrets *TypedBackendRegistry[appletenginehandlers.SecretsStore]
}

type TypedBackendRegistry[T any] struct {
	kind      string
	factories map[string]BackendFactory[T]
}

func NewBackendRegistry() *BackendRegistry {
	return &BackendRegistry{
		KV:      NewTypedBackendRegistry[appletenginehandlers.KVStore]("kv"),
		DB:      NewTypedBackendRegistry[appletenginehandlers.DBStore]("db"),
		Jobs:    NewTypedBackendRegistry[appletenginehandlers.JobsStore]("jobs"),
		Files:   NewTypedBackendRegistry[appletenginehandlers.FilesStore]("files"),
		Secrets: NewTypedBackendRegistry[appletenginehandlers.SecretsStore]("secrets"),
	}
}

func DefaultBackendRegistry() *BackendRegistry {
	registry := NewBackendRegistry()
	mustRegisterBuiltins(registry)
	return registry
}

func NewTypedBackendRegistry[T any](kind string) *TypedBackendRegistry[T] {
	return &TypedBackendRegistry[T]{
		kind:      strings.TrimSpace(kind),
		factories: make(map[string]BackendFactory[T]),
	}
}

func (r *TypedBackendRegistry[T]) Register(name string, factory BackendFactory[T]) error {
	if r == nil {
		return fmt.Errorf("composition applet: backend registry is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = factory.BackendID()
	}
	if name == "" {
		return fmt.Errorf("composition applet: %s backend name is required", r.kind)
	}
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("composition applet: %s backend %q already registered", r.kind, name)
	}
	if factory.BuildFunc == nil {
		return fmt.Errorf("composition applet: %s backend %q requires a build function", r.kind, name)
	}
	factory.ID = name
	r.factories[name] = factory
	return nil
}

func (r *TypedBackendRegistry[T]) RegisterFactory(factory BackendFactory[T]) error {
	return r.Register(factory.BackendID(), factory)
}

func (r *TypedBackendRegistry[T]) Lookup(name string) (BackendFactory[T], bool) {
	if r == nil {
		var zero BackendFactory[T]
		return zero, false
	}
	factory, ok := r.factories[strings.TrimSpace(name)]
	return factory, ok
}

func (r *TypedBackendRegistry[T]) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (r *TypedBackendRegistry[T]) Validate(name string, ctx BuildContext) error {
	factory, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("composition applet: %s backend %q not registered", r.kind, strings.TrimSpace(name))
	}
	return factory.Validate(ctx)
}

func (r *TypedBackendRegistry[T]) Build(name string, container *Container) (T, error) {
	factory, ok := r.Lookup(name)
	if !ok {
		var zero T
		return zero, fmt.Errorf("composition applet: %s backend %q not registered", r.kind, strings.TrimSpace(name))
	}
	return factory.Build(container)
}

func mustRegisterBuiltins(registry *BackendRegistry) {
	must(registry.KV.Register(appletsconfig.KVBackendMemory, BackendFactory[appletenginehandlers.KVStore]{
		ID: appletsconfig.KVBackendMemory,
		BuildFunc: func(*Container) (appletenginehandlers.KVStore, error) {
			return appletenginehandlers.NewMemoryKVStore(), nil
		},
	}))
	must(registry.KV.Register(appletsconfig.KVBackendRedis, BackendFactory[appletenginehandlers.KVStore]{
		ID: appletsconfig.KVBackendRedis,
		BuildFunc: func(container *Container) (appletenginehandlers.KVStore, error) {
			return appletenginehandlers.NewRedisKVStore(strings.TrimSpace(container.Context().EngineConfig.Redis.URL))
		},
	}))

	must(registry.DB.Register(appletsconfig.DBBackendMemory, BackendFactory[appletenginehandlers.DBStore]{
		ID: appletsconfig.DBBackendMemory,
		BuildFunc: func(*Container) (appletenginehandlers.DBStore, error) {
			return appletenginehandlers.NewMemoryDBStore(), nil
		},
	}))
	must(registry.DB.Register(appletsconfig.DBBackendPostgres, BackendFactory[appletenginehandlers.DBStore]{
		ID: appletsconfig.DBBackendPostgres,
		BuildFunc: func(container *Container) (appletenginehandlers.DBStore, error) {
			ctx := container.Context()
			if err := validateAppletSchemaArtifact(ctx.AppletName, ctx.ProjectRoot, ctx.Pool); err != nil {
				return nil, err
			}
			return appletenginehandlers.NewPostgresDBStore(ctx.Pool)
		},
	}))

	must(registry.Jobs.Register(appletsconfig.JobsBackendMemory, BackendFactory[appletenginehandlers.JobsStore]{
		ID: appletsconfig.JobsBackendMemory,
		BuildFunc: func(*Container) (appletenginehandlers.JobsStore, error) {
			return appletenginehandlers.NewMemoryJobsStore(), nil
		},
	}))
	must(registry.Jobs.Register(appletsconfig.JobsBackendPostgres, BackendFactory[appletenginehandlers.JobsStore]{
		ID: appletsconfig.JobsBackendPostgres,
		BuildFunc: func(container *Container) (appletenginehandlers.JobsStore, error) {
			return appletenginehandlers.NewPostgresJobsStore(container.Context().Pool)
		},
	}))

	must(registry.Files.Register(appletsconfig.FilesBackendLocal, BackendFactory[appletenginehandlers.FilesStore]{
		ID: appletsconfig.FilesBackendLocal,
		BuildFunc: func(container *Container) (appletenginehandlers.FilesStore, error) {
			return appletenginehandlers.NewLocalFilesStore(strings.TrimSpace(container.Context().EngineConfig.Files.Dir)), nil
		},
	}))
	must(registry.Files.Register(appletsconfig.FilesBackendPostgres, BackendFactory[appletenginehandlers.FilesStore]{
		ID: appletsconfig.FilesBackendPostgres,
		BuildFunc: func(container *Container) (appletenginehandlers.FilesStore, error) {
			ctx := container.Context()
			return appletenginehandlers.NewPostgresFilesStore(ctx.Pool, strings.TrimSpace(ctx.EngineConfig.Files.Dir))
		},
	}))
	must(registry.Files.Register(appletsconfig.FilesBackendS3, BackendFactory[appletenginehandlers.FilesStore]{
		ID: appletsconfig.FilesBackendS3,
		BuildFunc: func(container *Container) (appletenginehandlers.FilesStore, error) {
			ctx := container.Context()
			cfg := ctx.EngineConfig.S3
			accessKeyID, secretAccessKey := resolveS3Credentials(cfg)
			return appletenginehandlers.NewS3FilesStore(ctx.Pool, appletenginehandlers.S3FilesConfig{
				Bucket:          strings.TrimSpace(cfg.Bucket),
				Region:          strings.TrimSpace(cfg.Region),
				Endpoint:        strings.TrimSpace(cfg.Endpoint),
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				ForcePathStyle:  cfg.ForcePathStyle,
			})
		},
	}))

	must(registry.Secrets.Register(appletsconfig.SecretsBackendEnv, BackendFactory[appletenginehandlers.SecretsStore]{
		ID: appletsconfig.SecretsBackendEnv,
		BuildFunc: func(*Container) (appletenginehandlers.SecretsStore, error) {
			return appletenginehandlers.NewEnvSecretsStore(), nil
		},
	}))
	must(registry.Secrets.Register(appletsconfig.SecretsBackendPostgres, BackendFactory[appletenginehandlers.SecretsStore]{
		ID: appletsconfig.SecretsBackendPostgres,
		BuildFunc: func(container *Container) (appletenginehandlers.SecretsStore, error) {
			ctx := container.Context()
			masterKeyPayload, err := os.ReadFile(strings.TrimSpace(ctx.EngineConfig.Secrets.MasterKeyFile))
			if err != nil {
				return nil, err
			}
			return appletenginehandlers.NewPostgresSecretsStore(ctx.Pool, strings.TrimSpace(string(masterKeyPayload)))
		},
	}))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// resolveS3Credentials resolves S3 access credentials from the engine config.
//
// The applets config carries env-var names (AccessKeyEnv / SecretKeyEnv) rather
// than the secret values themselves, because TOML config files are not a safe
// place for secrets. This helper performs the single env lookup at backend
// build time — ensuring the os.Getenv call is isolated to one place and
// happens exactly once per backend construction, not per request.
//
// When the applets config is extended to carry direct values in a future
// version, this helper is the sole place that needs updating.
func resolveS3Credentials(cfg appletsconfig.AppletEngineS3Config) (string, string) {
	accessKeyID := strings.TrimSpace(os.Getenv(strings.TrimSpace(cfg.AccessKeyEnv)))
	secretAccessKey := strings.TrimSpace(os.Getenv(strings.TrimSpace(cfg.SecretKeyEnv)))
	return accessKeyID, secretAccessKey
}
