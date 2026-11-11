package manage

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	corePerms "github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/artifact"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Permission names (permission.Name()) required by the service.
var (
	PermRead    = corePerms.ExtensionsRead.Name()
	PermAuthor  = corePerms.ExtensionsAuthor.Name()
	PermReview  = corePerms.ExtensionsReview.Name()
	PermPublish = corePerms.ExtensionsPublish.Name()
	PermOperate = corePerms.ExtensionsOperate.Name()
)

// Config wires the service.
type Config struct {
	DB *pgxpool.Pool
	// Builder is the Node build worker adapter.
	Builder build.Builder
	// Kits renders `@iota-kit/<namespace>` modules from catalog sources.
	Kits build.KitGenerator
	// Artifacts stores source and build archives.
	Artifacts *artifact.Store
	// Catalog is the active catalog; it is invalidated after activation.
	Catalog *catalog.Catalog
	// Natives are the native entity descriptors the schema compiler resolves
	// foreign references against.
	Natives *data.NativeRegistry
	Git     GitConfig
	Logger  *slog.Logger
}

// Service is the package lifecycle backend. All methods take the acting
// subject (the session user, or an MCP credential's user) whose permissions
// decide; every action is audited.
type Service struct {
	db       *pgxpool.Pool
	builder  build.Builder
	kits     build.KitGenerator
	arts     *artifact.Store
	catalog  *catalog.Catalog
	compiler *data.Compiler
	git      GitConfig
	log      *slog.Logger

	workflows Workflows

	kitMu    sync.Mutex
	kitKey   string
	kitCache build.Kit
	// kitContracts are the contract hashes the cached kit was generated from
	kitContracts map[string]string

	verMu    sync.RWMutex
	verCache map[string]versionMeta
}

// versionMeta is the immutable part of a version used on hot paths.
type versionMeta struct {
	artifactHash string
	secrets      []string
	manifest     manifest.Manifest
}

// New builds the service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.DB == nil, cfg.Builder == nil, cfg.Artifacts == nil, cfg.Catalog == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime manage: DB, Builder, Artifacts and Catalog are required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Natives == nil {
		cfg.Natives = data.NewNativeRegistry()
	}
	return &Service{
		db: cfg.DB, builder: cfg.Builder, kits: cfg.Kits, arts: cfg.Artifacts, catalog: cfg.Catalog,
		compiler: data.NewCompiler(cfg.Natives, cfg.Catalog), git: cfg.Git.withDefaults(), log: cfg.Logger,
		verCache: map[string]versionMeta{},
	}, nil
}

// Catalog returns the catalog the service keeps current.
func (s *Service) Catalog() *catalog.Catalog { return s.catalog }

// Artifacts returns the artifact store.
func (s *Service) Artifacts() *artifact.Store { return s.arts }

// querier is the query side shared by pgx.Tx and pools.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// auditEntry is what an action reports about itself. After runs once the
// action committed.
type auditEntry struct {
	Action       string
	Namespace    string
	Version      string
	Target       string
	SourceHash   string
	ArtifactHash string
	Detail       map[string]any
	After        func()
}

func (s *Service) writeAudit(ctx context.Context, q querier, actor spi.Subject, e auditEntry, outcome string, cause error) error {
	detail := map[string]any{}
	for k, v := range e.Detail {
		detail[k] = v
	}
	if cause != nil {
		detail["error"] = cause.Error()
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		raw = []byte("{}")
	}
	_, err = q.Exec(ctx, `INSERT INTO extruntime.audit_log
		(actor_user_id, credential, action, namespace, version, target, source_hash, artifact_hash, outcome, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)`,
		actor.UserID, actor.Credential, e.Action, e.Namespace, e.Version, e.Target, e.SourceHash, e.ArtifactHash, outcome, string(raw))
	return err
}

func deniedError(perm string) error {
	return serrors.New(serrors.PermissionDenied, "permission "+perm+" is required")
}

// deny records a denied action.
func (s *Service) deny(ctx context.Context, actor spi.Subject, perm string, e auditEntry) error {
	if err := s.writeAudit(context.WithoutCancel(ctx), s.db, actor, e, OutcomeDenied, errors.New("missing "+perm)); err != nil {
		s.log.Error("extruntime manage: audit write failed", "action", e.Action, "error", err)
	}
	return deniedError(perm)
}

// mutate runs fn in one transaction together with its audit row. A failure
// rolls back both and is audited on its own.
func (s *Service) mutate(ctx context.Context, actor spi.Subject, perm string, e *auditEntry, fn func(tx pgx.Tx) error) error {
	const op = serrors.Op("extruntime.manage.mutate")
	if !actor.Has(perm) {
		return s.deny(ctx, actor, perm, *e)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return serrors.FromDB(op, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		s.failed(ctx, actor, *e, err)
		return err
	}
	if err := s.writeAudit(ctx, tx, actor, *e, OutcomeOK, nil); err != nil {
		return serrors.FromDB(op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return serrors.FromDB(op, err)
	}
	if e.After != nil {
		e.After()
	}
	return nil
}

// observe runs fn without a transaction (builds, exports) and audits the
// outcome afterwards.
func (s *Service) observe(ctx context.Context, actor spi.Subject, perm string, e *auditEntry, fn func() error) error {
	if !actor.Has(perm) {
		return s.deny(ctx, actor, perm, *e)
	}
	if err := fn(); err != nil {
		s.failed(ctx, actor, *e, err)
		return err
	}
	if err := s.writeAudit(context.WithoutCancel(ctx), s.db, actor, *e, OutcomeOK, nil); err != nil {
		s.log.Error("extruntime manage: audit write failed", "action", e.Action, "error", err)
	}
	return nil
}

func (s *Service) failed(ctx context.Context, actor spi.Subject, e auditEntry, cause error) {
	if err := s.writeAudit(context.WithoutCancel(ctx), s.db, actor, e, OutcomeFailed, cause); err != nil {
		s.log.Error("extruntime manage: audit write failed", "action", e.Action, "error", err)
	}
}

// require checks a permission for a read that is not audited.
func require(actor spi.Subject, perm string) error {
	if !actor.Has(perm) {
		return deniedError(perm)
	}
	return nil
}
