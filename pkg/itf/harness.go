// Package itf provides this package.
package itf

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	opCreateDB           = serrors.Op("itf.createDB")
	opCreatePool         = serrors.Op("itf.createPool")
	opSetupApplication   = serrors.Op("itf.setupApplication")
	opResolveTenant      = serrors.Op("itf.resolveTenant")
	opRunMigrationPolicy = serrors.Op("itf.runMigrationPolicy")
	opOncePerHarnessSeed = serrors.Op("itf.seedOncePerHarness")
	opDropDB             = serrors.Op("itf.dropDB")
	opCloseControllers   = serrors.Op("itf.closeControllers")
	opSchemaReadiness    = serrors.Op("itf.schemaReadiness")
)

type ProvisioningMode string

const (
	ProvisioningSharedPerPackage ProvisioningMode = "shared_per_package"
	ProvisioningPerTestDatabase  ProvisioningMode = "per_test_database"
)

type CleanupMode string

const (
	CleanupDropOnExit CleanupMode = "drop_on_exit"
	CleanupKeep       CleanupMode = "keep"
)

type MigrationPolicy string

const (
	MigrationApplyOnce MigrationPolicy = "apply_once"
	MigrationSkip      MigrationPolicy = "skip"
)

type IsolationMode string

const (
	IsolationRollback  IsolationMode = "rollback"
	IsolationCommitted IsolationMode = "committed"
)

type SeedPolicy string

const (
	SeedNone           SeedPolicy = "none"
	SeedOncePerHarness SeedPolicy = "once_per_harness"
	SeedPerTest        SeedPolicy = "per_test"
)

type PoolConfig struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type DatabaseConfig struct {
	Provisioning ProvisioningMode
	Cleanup      CleanupMode
	Pool         PoolConfig
}

type MigrationConfig struct {
	Policy MigrationPolicy
}

type TxConfig struct {
	LockTimeout      time.Duration
	IdleInTxTimeout  time.Duration
	StatementTimeout time.Duration
}

type IsolationConfig struct {
	Mode IsolationMode
	Tx   TxConfig
}

type SeedConfig struct {
	Policy SeedPolicy
	Run    func(ctx context.Context, app application.Application) error
}

type ContextConfig struct {
	User     user.User
	TenantID *uuid.UUID
	Locales  []string
}

type HarnessConfig struct {
	Name       string
	Components []composition.Component
	Database   DatabaseConfig
	Migration  MigrationConfig
	Isolation  IsolationConfig
	Seed       SeedConfig
	Context    ContextConfig
}

type Scope struct {
	Ctx    context.Context
	Pool   *pgxpool.Pool
	Tx     pgx.Tx
	App    application.Application
	Tenant *composables.Tenant
	User   user.User
}

type Harness interface {
	Scope(tb testing.TB) *Scope
	Close() error
}

type harnessImpl struct {
	cfg       HarnessConfig
	state     *harnessState
	shared    bool
	closed    bool
	mu        sync.Mutex
	closeErr  error
	closeDone chan struct{}
}

type harnessState struct {
	cfg       HarnessConfig
	key       string
	dbName    string
	pool      *pgxpool.Pool
	app       application.Application
	tenant    *composables.Tenant
	baseCtx   context.Context
	closeOnce sync.Once
}

type harnessManager struct {
	mu      sync.Mutex
	entries map[string]*managedHarnessState
}

type managedHarnessState struct {
	state   *harnessState
	refs    int
	closing bool
	cond    *sync.Cond
}

type schemaReadinessQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var sharedHarnessManager = &harnessManager{
	entries: map[string]*managedHarnessState{},
}

func NewHarness(tb testing.TB, cfg HarnessConfig) Harness {
	tb.Helper()

	cfg = normalizeHarnessConfig(tb, cfg)
	key := buildHarnessKey(cfg)

	if cfg.Database.Provisioning == ProvisioningPerTestDatabase {
		state, err := createHarnessState(key, cfg, true)
		if err != nil {
			tb.Fatalf("failed to create per-test harness: %v", err)
		}
		h := &harnessImpl{cfg: cfg, state: state, shared: false}
		tb.Cleanup(func() { _ = h.Close() })
		return h
	}

	state, err := sharedHarnessManager.getOrCreate(key, cfg)
	if err != nil {
		tb.Fatalf("failed to create shared harness: %v", err)
	}

	h := &harnessImpl{cfg: cfg, state: state, shared: true}
	tb.Cleanup(func() { _ = h.Close() })
	return h
}

func (h *harnessImpl) Scope(tb testing.TB) *Scope {
	tb.Helper()

	ctx := h.state.baseCtx
	if h.cfg.Context.User != nil {
		ctx = composables.WithUser(ctx, h.cfg.Context.User)
	}
	switch h.cfg.Isolation.Mode {
	case IsolationCommitted:
		if h.cfg.Seed.Policy == SeedPerTest && h.cfg.Seed.Run != nil {
			if err := composables.InTx(ctx, func(seedCtx context.Context) error {
				return h.cfg.Seed.Run(seedCtx, h.state.app)
			}); err != nil {
				tb.Fatalf("failed to run per-test seed in committed mode: %v", err)
			}
		}
		return &Scope{
			Ctx:    ctx,
			Pool:   h.state.pool,
			App:    h.state.app,
			Tenant: h.state.tenant,
			User:   h.cfg.Context.User,
		}
	case IsolationRollback:
		tx, err := h.state.pool.Begin(ctx)
		if err != nil {
			tb.Fatalf("failed to begin rollback scope transaction: %v", err)
		}

		scopeCtx := composables.WithTx(ctx, tx)
		if err := applyTxSettings(scopeCtx, tx, h.cfg.Isolation.Tx); err != nil {
			_ = tx.Rollback(scopeCtx)
			tb.Fatalf("failed to apply rollback scope tx settings: %v", err)
		}

		if h.cfg.Seed.Policy == SeedPerTest && h.cfg.Seed.Run != nil {
			if err := h.cfg.Seed.Run(scopeCtx, h.state.app); err != nil {
				_ = tx.Rollback(scopeCtx)
				tb.Fatalf("failed to run per-test seed: %v", err)
			}
		}

		tb.Cleanup(func() {
			if err := tx.Rollback(scopeCtx); err != nil && err != pgx.ErrTxClosed {
				tb.Logf("warning: failed to rollback scope tx: %v", err)
			}
		})

		return &Scope{
			Ctx:    scopeCtx,
			Pool:   h.state.pool,
			Tx:     tx,
			App:    h.state.app,
			Tenant: h.state.tenant,
			User:   h.cfg.Context.User,
		}
	default:
		tb.Fatalf("unsupported isolation mode: %s", h.cfg.Isolation.Mode)
		return nil
	}
}

func (h *harnessImpl) Close() error {
	h.mu.Lock()
	if h.closed {
		done := h.closeDone
		h.mu.Unlock()
		if done != nil {
			<-done
		}
		h.mu.Lock()
		err := h.closeErr
		h.mu.Unlock()
		return err
	}
	h.closed = true
	done := make(chan struct{})
	h.closeDone = done
	h.mu.Unlock()

	var err error
	if h.shared {
		err = sharedHarnessManager.close(h.state.key, h.cfg.Database.Cleanup)
	} else {
		err = h.state.close(h.cfg.Database.Cleanup)
	}

	h.mu.Lock()
	h.closeErr = err
	h.closeDone = nil
	close(done)
	h.mu.Unlock()
	return err
}

func (m *harnessManager) getOrCreate(key string, cfg HarnessConfig) (*harnessState, error) {
	m.mu.Lock()

	for {
		if entry, ok := m.entries[key]; ok {
			for entry.closing {
				entry.cond.Wait()
			}
			if entry.state == nil {
				delete(m.entries, key)
				continue
			}
			entry.refs++
			m.mu.Unlock()
			return entry.state, nil
		}

		entry := &managedHarnessState{
			refs:    1,
			closing: true,
			cond:    sync.NewCond(&m.mu),
		}
		m.entries[key] = entry
		m.mu.Unlock()

		state, err := createHarnessState(key, cfg, false)

		m.mu.Lock()
		if err != nil {
			delete(m.entries, key)
			entry.closing = false
			entry.cond.Broadcast()
			m.mu.Unlock()
			return nil, err
		}

		entry.state = state
		entry.closing = false
		entry.cond.Broadcast()
		m.mu.Unlock()
		return state, nil
	}
}

func (m *harnessManager) close(key string, cleanup CleanupMode) error {
	m.mu.Lock()
	entry, ok := m.entries[key]
	if !ok {
		m.mu.Unlock()
		return nil
	}

	for entry.closing {
		entry.cond.Wait()
	}

	entry.refs--
	if entry.refs > 0 {
		m.mu.Unlock()
		return nil
	}

	entry.closing = true
	m.mu.Unlock()

	err := entry.state.close(cleanup)

	m.mu.Lock()
	delete(m.entries, key)
	entry.state = nil
	entry.closing = false
	entry.cond.Broadcast()
	m.mu.Unlock()

	return err
}

func (s *harnessState) close(cleanup CleanupMode) error {
	var closeErr error
	s.closeOnce.Do(func() {
		if s.app != nil {
			closeErr = mergeCloseErrors(closeErr, closeApplication(s.app))
		}
		if s.pool != nil {
			s.pool.Close()
		}
		if cleanup == CleanupDropOnExit {
			if err := DropDBE(s.dbName); err != nil {
				closeErr = mergeCloseErrors(closeErr, serrors.E(opDropDB, err, "drop database on close"))
			}
		}
	})
	return closeErr
}

func mergeCloseErrors(existing, next error) error {
	if next == nil {
		return existing
	}
	if existing == nil {
		return next
	}
	return errors.Join(existing, next)
}

func harnessDBOpts(name string) string {
	c := configuration.Use()
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		strings.ToLower(sanitizeDBName(name)),
		c.Database.Password,
	)
}

func createHarnessState(key string, cfg HarnessConfig, isPerTest bool) (*harnessState, error) {
	dbName := buildDBName(cfg.Name, key, isPerTest)
	if err := CreateDBE(dbName); err != nil {
		return nil, serrors.E(opCreateDB, err, "create database")
	}

	pool, err := newPoolWithConfig(harnessDBOpts(dbName), cfg.Database.Pool)
	if err != nil {
		if cleanupErr := DropDBE(dbName); cleanupErr != nil {
			return nil, serrors.E(opCreatePool, cleanupErr, "cleanup database after pool creation failure")
		}
		return nil, serrors.E(opCreatePool, err, "create pool")
	}

	app, err := SetupApplication(pool, cfg.Components)
	if err != nil {
		pool.Close()
		_ = DropDBE(dbName)
		return nil, serrors.E(opSetupApplication, err, "setup application")
	}

	if err := runMigrationPolicy(context.Background(), pool, app, cfg.Migration); err != nil {
		combinedErr := serrors.E(opRunMigrationPolicy, err, "migration policy")
		closeErr := closeApplication(app)
		pool.Close()
		dropErr := DropDBE(dbName)
		if closeErr != nil {
			combinedErr = mergeCloseErrors(
				combinedErr,
				serrors.E(opRunMigrationPolicy, closeErr, "failed to close controllers after migration policy failure"),
			)
		}
		if dropErr != nil {
			combinedErr = mergeCloseErrors(
				combinedErr,
				serrors.E(opDropDB, dropErr, "drop database after migration policy failure"),
			)
		}
		return nil, combinedErr
	}

	tenant, err := resolveTenant(context.Background(), pool, cfg.Context.TenantID)
	if err != nil {
		closeErr := closeApplication(app)
		pool.Close()
		_ = DropDBE(dbName)
		if closeErr != nil {
			return nil, serrors.E(opResolveTenant, closeErr, "failed to close controllers before tenant resolve failure")
		}
		return nil, serrors.E(opResolveTenant, err, "resolve tenant")
	}

	baseCtx := buildBaseContext(pool, app, tenant, cfg.Context)

	if cfg.Seed.Policy == SeedOncePerHarness && cfg.Seed.Run != nil {
		if err := composables.InTx(baseCtx, func(seedCtx context.Context) error {
			return cfg.Seed.Run(seedCtx, app)
		}); err != nil {
			closeErr := closeApplication(app)
			pool.Close()
			_ = DropDBE(dbName)
			if closeErr != nil {
				return nil, serrors.E(opOncePerHarnessSeed, closeErr, "failed to close controllers before seed failure")
			}
			return nil, serrors.E(opOncePerHarnessSeed, err, "once-per-harness seed")
		}
	}

	return &harnessState{
		cfg:     cfg,
		key:     key,
		dbName:  dbName,
		pool:    pool,
		app:     app,
		tenant:  tenant,
		baseCtx: baseCtx,
	}, nil
}

func normalizeHarnessConfig(tb testing.TB, cfg HarnessConfig) HarnessConfig {
	tb.Helper()
	if cfg.Database.Provisioning == "" {
		cfg.Database.Provisioning = ProvisioningSharedPerPackage
	}
	if cfg.Name == "" {
		if cfg.Database.Provisioning == ProvisioningSharedPerPackage {
			cfg.Name = inferSharedHarnessName()
			if cfg.Name == "" {
				cfg.Name = "itf_harness_shared"
			}
		} else {
			cfg.Name = "itf_harness_" + tb.Name()
		}
	}
	if cfg.Database.Cleanup == "" {
		cfg.Database.Cleanup = CleanupDropOnExit
	}
	if cfg.Migration.Policy == "" {
		cfg.Migration.Policy = MigrationApplyOnce
	}
	if cfg.Isolation.Mode == "" {
		cfg.Isolation.Mode = IsolationRollback
	}
	if cfg.Seed.Policy == "" {
		cfg.Seed.Policy = SeedNone
	}
	if len(cfg.Context.Locales) == 0 {
		cfg.Context.Locales = []string{"en"}
	}
	return cfg
}

func inferSharedHarnessName() string {
	const defaultName = "itf_harness_shared"

	for depth := 2; ; depth++ {
		_, file, _, ok := runtime.Caller(depth)
		if !ok || file == "" {
			return defaultName
		}
		dir := filepath.Dir(file)
		if !strings.Contains(dir, "/pkg/itf") {
			// Hash the caller path so different packages that share a folder name do not collide.
			hash := sha1.Sum([]byte(dir))
			return fmt.Sprintf("itf_harness_shared_%s", hex.EncodeToString(hash[:])[:8])
		}
	}
}

func buildHarnessKey(cfg HarnessConfig) string {
	componentTypes := make([]string, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		componentTypes = append(componentTypes, reflect.TypeOf(component).String())
	}

	return fmt.Sprintf(
		"name=%s|mods=%v|components=%v|prov=%s|migrate=%s|iso=%s|cleanup=%s|seed=%s|pool=%d/%d/%s/%s|tx=%s/%s/%s|tenant=%s|locales=%v",
		cfg.Name,
		nil,
		componentTypes,
		cfg.Database.Provisioning,
		cfg.Migration.Policy,
		cfg.Isolation.Mode,
		cfg.Database.Cleanup,
		cfg.Seed.Policy,
		cfg.Database.Pool.MaxConns,
		cfg.Database.Pool.MinConns,
		cfg.Database.Pool.MaxConnLifetime,
		cfg.Database.Pool.MaxConnIdleTime,
		cfg.Isolation.Tx.LockTimeout,
		cfg.Isolation.Tx.IdleInTxTimeout,
		cfg.Isolation.Tx.StatementTimeout,
		tenantID(cfg.Context.TenantID),
		cfg.Context.Locales,
	)
}

func tenantID(t *uuid.UUID) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func buildDBName(base, key string, perTest bool) string {
	hash := sha1.Sum([]byte(key))
	suffix := hex.EncodeToString(hash[:])[:8]
	if perTest {
		return fmt.Sprintf("%s_%s_%s", base, suffix, uuid.New().String()[:8])
	}
	return fmt.Sprintf("%s_%s", base, suffix)
}

func runMigrationPolicy(ctx context.Context, pool schemaReadinessQuerier, app application.Application, cfg MigrationConfig) error {
	switch cfg.Policy {
	case MigrationApplyOnce:
		return app.Migrations().Run()
	case MigrationSkip:
		if pool == nil {
			return serrors.E(opSchemaReadiness, serrors.Invalid, "schema readiness requires a database pool")
		}
		return ensureSchemaReady(ctx, pool)
	default:
		return serrors.E(
			opRunMigrationPolicy,
			serrors.Invalid,
			fmt.Sprintf("unsupported migration policy: %s", cfg.Policy),
		)
	}
}

func ensureSchemaReady(ctx context.Context, pool schemaReadinessQuerier) error {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'gorp_migrations'
		)
	`
	var ok bool
	if err := pool.QueryRow(ctx, query).Scan(&ok); err != nil {
		return serrors.E(opSchemaReadiness, err, "schema readiness probe failed")
	}
	if !ok {
		return serrors.E(opSchemaReadiness, serrors.KindValidation, "schema not ready: public.gorp_migrations is missing")
	}
	return nil
}

func resolveTenant(ctx context.Context, pool *pgxpool.Pool, tenantID *uuid.UUID) (*composables.Tenant, error) {
	if tenantID == nil {
		return CreateTestTenant(ctx, pool)
	}
	t := &composables.Tenant{
		ID:     *tenantID,
		Name:   "Test Tenant " + tenantID.String()[:8],
		Domain: tenantID.String()[:8] + ".test.com",
	}
	_, err := pool.Exec(
		ctx,
		"INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, domain = EXCLUDED.domain, updated_at = NOW()",
		t.ID,
		t.Name,
		t.Domain,
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func buildBaseContext(pool *pgxpool.Pool, app application.Application, tenant *composables.Tenant, cfg ContextConfig) context.Context {
	ctx := context.Background()
	ctx = composables.WithPool(ctx, pool)
	ctx = composables.WithTenantID(ctx, tenant.ID)
	ctx = composables.WithParams(ctx, DefaultParams())
	ctx = composables.WithSession(ctx, MockSession())
	ctx = context.WithValue(ctx, constants.AppKey, app)
	if container, ok := composition.ForApp(app); ok {
		ctx = context.WithValue(ctx, constants.ContainerKey, container)
	}

	locale := "en"
	if len(cfg.Locales) > 0 && cfg.Locales[0] != "" {
		locale = cfg.Locales[0]
	}
	localizer := i18n.NewLocalizer(app.Bundle(), locale)
	ctx = intl.WithLocalizer(ctx, localizer)
	return ctx
}

func applyTxSettings(ctx context.Context, tx pgx.Tx, cfg TxConfig) error {
	lockTimeout := cfg.LockTimeout
	if lockTimeout == 0 {
		lockTimeout = 2 * time.Second
	}
	idleTimeout := cfg.IdleInTxTimeout
	if idleTimeout == 0 {
		idleTimeout = 60 * time.Second
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%d'", lockTimeout.Milliseconds())); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL idle_in_transaction_session_timeout = '%d'", idleTimeout.Milliseconds())); err != nil {
		return err
	}
	if cfg.StatementTimeout > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%d'", cfg.StatementTimeout.Milliseconds())); err != nil {
			return err
		}
	}

	return nil
}

func newPoolWithConfig(dbOpts string, cfg PoolConfig) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsed, err := pgxpool.ParseConfig(dbOpts)
	if err != nil {
		return nil, err
	}

	parsed.MaxConns = 4
	parsed.MinConns = 0
	parsed.MaxConnLifetime = 5 * time.Minute
	parsed.MaxConnIdleTime = 30 * time.Second

	if cfg.MaxConns > 0 {
		parsed.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		parsed.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		parsed.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		parsed.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	return pgxpool.NewWithConfig(ctx, parsed)
}

func closeControllers(controllers []application.Controller) error {
	var closeErr error
	for _, controller := range controllers {
		closer, ok := controller.(controllerCloser)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			closeErr = mergeCloseErrors(
				closeErr,
				serrors.E(opCloseControllers, err, fmt.Sprintf("failed to close controller %T", controller)),
			)
		}
	}
	return closeErr
}

func closeApplication(app application.Application) error {
	if app == nil {
		return nil
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var closeErr error
	if container, ok := composition.ForApp(app); ok {
		closeErr = composition.Stop(stopCtx, container)
	}
	return mergeCloseErrors(closeErr, closeControllers(app.Controllers()))
}
