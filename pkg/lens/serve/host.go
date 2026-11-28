package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/engine"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	defaultHostTTL              = 30 * time.Minute
	defaultHostMaxEntries       = 128
	defaultHostCacheVersion     = "lens-host-v1"
	defaultHostFacetLimit       = 75
	recomputeCooldownClockGrace = 50 * time.Millisecond
)

type hostRuntimeRequestContextKey struct{}
type hostRecomputeContextKey struct{}

// RecomputeRequested reports whether the active request is an explicit user
// panel recompute dispatched by Host.Panel. Host data loaders and resolvers
// can check this to bypass upstream memoization on recompute.
func RecomputeRequested(ctx context.Context) bool {
	recompute, _ := ctx.Value(hostRecomputeContextKey{}).(bool)
	return recompute
}

// WithRecompute marks ctx as an explicit user recompute request.
func WithRecompute(ctx context.Context) context.Context {
	return withRecompute(ctx, true)
}

func withRecompute(ctx context.Context, recompute bool) context.Context {
	return context.WithValue(ctx, hostRecomputeContextKey{}, recompute)
}

// CacheOptions configures parameter-keyed document reuse, bounded memory,
// admission control, and panel recompute cooldown for Host.
type CacheOptions struct {
	// TTL bounds snapshot lifetime when Host constructs its own SnapshotStore.
	// Defaults to 30 minutes.
	TTL time.Duration
	// MaxEntries bounds cached keyed documents (default 128).
	MaxEntries int
	// MaxDerivedPerReport bounds derived drawer/sub-documents per parent (default 16).
	MaxDerivedPerReport int
	// MaxDocumentBytes bounds one frozen document payload (default document.DefaultMaxDocumentBytes).
	MaxDocumentBytes int64
	// MaxConcurrentBuilds bounds concurrent document builds (default 4).
	MaxConcurrentBuilds int
	// MaxPendingBuilds bounds queued document builds (default 16).
	MaxPendingBuilds int
	// MaxWaitersPerBuild bounds coalesced waiters per flight (default 128).
	MaxWaitersPerBuild int
	// Budget accounts retained payload across ReportCache, SnapshotStore, and exports.
	// When nil, Host allocates an isolated default Budget.
	Budget *resource.Budget
	// BuildLimiter may be shared across multiple Host instances for process-wide admission.
	BuildLimiter *ReportBuildLimiter
	// Version salts cache keys so hosts can invalidate across schema revisions.
	// Defaults to "lens-host-v1".
	Version string
	// RecomputeCooldown controls how Host.Panel handles global recomputes that
	// arrive shortly after a panel was materialized. Zero defaults to 5s;
	// negative disables the cooldown wait/check.
	RecomputeCooldown time.Duration
}

// RequestCacheKey supplies the lightweight cache identity for one HTTP request
// before ResolveCube or ResolveDashboard executes.
type RequestCacheKey struct {
	// Request supplies the runtime scope (DataScope, Namespace, Locale,
	// Timezone, DataSources) used to partition cached entries.
	Request lensruntime.Request
	// CacheKey overrides the canonical query-string params portion of ReportKey.
	// When empty, Host derives it from the normalized URL query.
	CacheKey string
	// Variant optionally partitions cached reports and cubes (for example by
	// permission variant).
	Variant string
	// CubeKey optionally identifies a reusable ResolvedCube across /document
	// drill levels and /facet-options requests (for example, date range only
	// without _f). When empty in cube mode, Host derives the cube cache key
	// from the normalized URL query without _facet and _facet_search.
	CubeKey string
}

// CacheKeyResolver computes the lightweight cache identity for an HTTP request
// so Host can check ReportCache, coalesce concurrent builds, and enforce
// BuildLimiter admission before invoking ResolveCube or ResolveDashboard.
type CacheKeyResolver func(ctx context.Context, r *http.Request) (RequestCacheKey, error)

// ResolvedCube supplies the cube specification and per-request runtime context
// for one HTTP request served in cube mode.
type ResolvedCube struct {
	Spec cube.CubeSpec
	// LoadSpec, when non-nil, defers expensive cube construction (such as
	// loading database facts into a Dataset-mode FrameSet) until ReportCache
	// misses and BuildLimiter admits the build, or until /facet-options misses
	// the host's cube cache.
	LoadSpec func(ctx context.Context) (cube.CubeSpec, error)
	Request  lensruntime.Request
	CacheKey string
	Variant  string
	// CubeCacheKey optionally identifies the reusable CubeSpec across /document
	// drill levels and /facet-options requests when LoadSpec is set. When empty,
	// Host derives it from the normalized URL query without _facet and _facet_search.
	CubeCacheKey    string
	DocumentOptions document.BuildOptions
	Prepare         func(*document.DashboardDocument) error
	// PagePath is the browser-facing page URL used as target for drill and facet
	// navigation links. Defaults to HostConfig.PagePath (or BasePath without "/lens").
	PagePath string
	// DataSource optionally supplies the datasource used for SQL/Named facet
	// options and drill filter label lookups. Defaults to Request.DataSources[Spec.DataSource].
	DataSource datasource.DataSource
}

// CubeResolver resolves one incoming HTTP request and normalized drill context
// into a declarative cube specification and runtime inputs.
type CubeResolver func(ctx context.Context, r *http.Request, drill cube.DrillContext) (ResolvedCube, error)

// ResolvedDashboard supplies a manual/custom DashboardSpec and per-request
// runtime context for one HTTP request served in dashboard mode.
type ResolvedDashboard struct {
	Spec            lens.DashboardSpec
	Request         lensruntime.Request
	CacheKey        string
	Variant         string
	DocumentOptions document.BuildOptions
	Prepare         func(*document.DashboardDocument) error
}

// DashboardResolver resolves one incoming HTTP request into a custom
// lens.DashboardSpec and runtime inputs.
type DashboardResolver func(ctx context.Context, r *http.Request) (ResolvedDashboard, error)

// HostConfig configures a unified Lens HTTP host supporting both declarative
// cube dashboards (ResolveCube) and custom dashboards (ResolveDashboard).
type HostConfig struct {
	// BasePath is the mounted HTTP prefix for the Lens endpoints (for example
	// "/analytics/agent-program/lens").
	BasePath string
	// PagePath is the browser-facing dashboard page path used for cube drill
	// and facet links (for example "/analytics/agent-program"). When empty, Host
	// derives it by stripping a trailing "/lens" segment from BasePath.
	PagePath         string
	Executor         engine.Executor
	Snapshots        document.SnapshotStore
	Cache            CacheOptions
	ExecutionTimeout time.Duration
	Export           ExportOptions
	Observer         Observer
	InlineDepth      int
	PageSize         int
	Progressive      bool
	// Request optionally supplies per-request runtime scope (DataScope,
	// Namespace, DataSources, Locale, Timezone) for follow-up snapshot routes
	// (/query, /panel, /export, /release, /drawer). When nil, Host falls back
	// to ResolveCacheKey, ResolveCube, or ResolveDashboard to obtain runtime scope.
	Request RequestResolver
	// ResolveCacheKey optionally computes lightweight cache identity before
	// ResolveCube or ResolveDashboard executes. When configured, ReportCache
	// lookup, singleflight coalescing, and BuildLimiter admission run before
	// ResolveCube/ResolveDashboard, and resolved cubes are cached across
	// /document and /facet-options requests.
	ResolveCacheKey   CacheKeyResolver
	DrawerResolver    DrawerResolver
	Exploration       ExplorationExecutor
	ResolveLoader     ExplorationLoaderResolver
	ResolveCube       CubeResolver
	ResolveDashboard  DashboardResolver
	EmptyDrillValue   string
	FacetOptionsLimit int
}

type hostEntry struct {
	progressive bool
}

// Host is a unified, parameter-keyed HTTP host built on top of serve.Handlers,
// serve.ReportCache, and document.SnapshotStore.
type Host struct {
	basePath          string
	pagePath          string
	executor          engine.Executor
	snapshots         document.SnapshotStore
	cache             *ReportCache[*hostEntry]
	cubeCache         *hostCubeCache
	budget            *resource.Budget
	buildLimiter      *ReportBuildLimiter
	cacheVersion      string
	executionTimeout  time.Duration
	recomputeCooldown time.Duration
	export            ExportOptions
	observer          Observer
	inlineDepth       int
	pageSize          int
	progressive       bool
	request           RequestResolver
	resolveCacheKey   CacheKeyResolver
	drawerResolver    DrawerResolver
	exploration       ExplorationExecutor
	resolveLoader     ExplorationLoaderResolver
	resolveCube       CubeResolver
	resolveDashboard  DashboardResolver
	emptyDrillValue   string
	facetLimit        int
}

// NewHost validates cfg and constructs a unified Lens Host.
func NewHost(cfg HostConfig) (*Host, error) {
	const op serrors.Op = "lens/serve.NewHost"
	if cfg.Executor == nil {
		return nil, serrors.Wrap(op, fmt.Errorf("lens executor is required"))
	}
	if (cfg.ResolveCube == nil) == (cfg.ResolveDashboard == nil) {
		return nil, serrors.Wrap(op, fmt.Errorf("exactly one of ResolveCube or ResolveDashboard must be configured"))
	}
	if (cfg.Exploration == nil) != (cfg.ResolveLoader == nil) {
		return nil, serrors.Wrap(op, fmt.Errorf("exploration executor and loader resolver must be configured together"))
	}
	if cfg.InlineDepth < 0 {
		return nil, serrors.Wrap(op, fmt.Errorf("inline depth cannot be negative"))
	}
	if cfg.PageSize < 0 {
		return nil, serrors.Wrap(op, fmt.Errorf("page size cannot be negative"))
	}
	basePath, err := normalizeBasePath(cfg.BasePath)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	pagePath := strings.TrimSpace(cfg.PagePath)
	if pagePath == "" {
		pagePath = defaultPagePath(basePath)
	} else {
		pagePath, err = normalizeBasePath(pagePath)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		if pagePath == "" {
			pagePath = "/"
		}
	}

	budget := cfg.Cache.Budget
	if budget == nil {
		budget = resource.NewBudget(resource.DefaultLimit)
	}
	maxEntries := cfg.Cache.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultHostMaxEntries
	}
	maxDocumentBytes := cfg.Cache.MaxDocumentBytes
	if maxDocumentBytes <= 0 {
		maxDocumentBytes = document.DefaultMaxDocumentBytes
	}
	ttl := cfg.Cache.TTL
	if ttl <= 0 {
		ttl = defaultHostTTL
	}
	snapshots := cfg.Snapshots
	if snapshots == nil {
		snapshots = document.NewMemoryStoreWithOptions(document.MemoryStoreOptions{
			TTL:           ttl,
			MaxEntries:    maxEntries,
			Budget:        budget,
			MaxEntryBytes: maxDocumentBytes,
		})
	}
	execTimeout := cfg.ExecutionTimeout
	if execTimeout <= 0 {
		execTimeout = defaultWorkTimeout
	}
	concurrent := cfg.Cache.MaxConcurrentBuilds
	if concurrent <= 0 {
		concurrent = 4
	}
	pending := cfg.Cache.MaxPendingBuilds
	if pending == 0 && cfg.Cache.BuildLimiter == nil {
		pending = 16
	}
	limiter := cfg.Cache.BuildLimiter
	if limiter == nil {
		limiter = NewReportBuildLimiter(concurrent, pending)
	}
	cacheVersion := strings.TrimSpace(cfg.Cache.Version)
	if cacheVersion == "" {
		cacheVersion = defaultHostCacheVersion
	}
	cache, err := NewReportCache(ReportCacheOptions[*hostEntry]{
		Budget:              budget,
		BuildLimiter:        limiter,
		MaxEntries:          maxEntries,
		MaxDerivedPerReport: cfg.Cache.MaxDerivedPerReport,
		MaxDocumentBytes:    maxDocumentBytes,
		MaxConcurrentBuilds: concurrent,
		MaxPendingBuilds:    pending,
		MaxWaitersPerBuild:  cfg.Cache.MaxWaitersPerBuild,
		BuildTimeout:        execTimeout,
	})
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	observer := cfg.Observer
	if observer == nil {
		observer = noopObserver{}
	}
	cooldown := cfg.Cache.RecomputeCooldown
	if cooldown == 0 {
		cooldown = recomputeCooldown
	}
	emptyDrillValue := strings.TrimSpace(cfg.EmptyDrillValue)
	if emptyDrillValue == "" {
		emptyDrillValue = cube.UnknownDrillValue
	}
	facetLimit := cfg.FacetOptionsLimit
	if facetLimit <= 0 {
		facetLimit = defaultHostFacetLimit
	}
	return &Host{
		basePath:          basePath,
		pagePath:          pagePath,
		executor:          cfg.Executor,
		snapshots:         snapshots,
		cache:             cache,
		cubeCache:         newHostCubeCache(ttl, maxEntries, execTimeout, limiter),
		budget:            budget,
		buildLimiter:      limiter,
		cacheVersion:      cacheVersion,
		executionTimeout:  execTimeout,
		recomputeCooldown: cooldown,
		export:            cfg.Export,
		observer:          observer,
		inlineDepth:       cfg.InlineDepth,
		pageSize:          cfg.PageSize,
		progressive:       cfg.Progressive,
		request:           cfg.Request,
		resolveCacheKey:   cfg.ResolveCacheKey,
		drawerResolver:    cfg.DrawerResolver,
		exploration:       cfg.Exploration,
		resolveLoader:     cfg.ResolveLoader,
		resolveCube:       cfg.ResolveCube,
		resolveDashboard:  cfg.ResolveDashboard,
		emptyDrillValue:   emptyDrillValue,
		facetLimit:        facetLimit,
	}, nil
}

// Mount registers the standard Lens routes on router:
//   - GET  /document
//   - POST /query
//   - POST /panel
//   - GET  /export
//   - POST /release
//   - POST /drawer (when DrawerResolver is configured)
//   - GET  /facet-options (when ResolveCube is configured)
//   - GET  /toggle-filter (when ResolveCube is configured)
func (h *Host) Mount(router *mux.Router) {
	if router == nil {
		return
	}
	router.HandleFunc("/document", h.Document).Methods(http.MethodGet)
	router.HandleFunc("/query", h.Query).Methods(http.MethodPost)
	router.HandleFunc("/panel", h.Panel).Methods(http.MethodPost)
	router.HandleFunc("/export", h.Export).Methods(http.MethodGet)
	router.HandleFunc("/release", h.Release).Methods(http.MethodPost)
	if h.drawerResolver != nil {
		router.HandleFunc("/drawer", h.Drawer).Methods(http.MethodPost)
	}
	if h.resolveCube != nil {
		router.HandleFunc("/facet-options", h.FacetOptions).Methods(http.MethodGet)
		router.HandleFunc("/toggle-filter", h.ToggleFilter).Methods(http.MethodGet)
	}
}

// Handler returns a standalone http.Handler dispatching the host's endpoints.
// It accepts both relative paths ("/document") and BasePath-prefixed paths
// ("<BasePath>/document").
func (h *Host) Handler() http.Handler {
	router := mux.NewRouter()
	if h.basePath != "" {
		h.Mount(router.PathPrefix(h.basePath).Subrouter())
	}
	h.Mount(router)
	return router
}

// Stats returns the current ReportCache statistics for this host.
func (h *Host) Stats() ReportCacheStats {
	return h.cache.Stats()
}

// Close cancels in-flight builds and releases cached entries.
func (h *Host) Close() {
	h.cubeCache.Close()
	h.cache.Close()
}

// MaterializeDerived builds or reuses a derived document bound to parentID.
func (h *Host) MaterializeDerived(
	ctx context.Context,
	parentID string,
	key string,
	class DerivedSurfaceClass,
	build func(context.Context) (*document.DashboardDocument, error),
) (*document.DashboardDocument, error) {
	return h.cache.MaterializeDerived(ctx, parentID, key, class, build)
}

// Document materializes or reuses the parameter-keyed dashboard document for r.
func (h *Host) Document(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	if h.resolveCube != nil {
		h.serveCubeDocument(w, r)
		return
	}
	h.serveDashboardDocument(w, r)
}

func (h *Host) serveCubeDocument(w http.ResponseWriter, r *http.Request) {
	rawQuery := cloneValues(r.URL.Query())
	drillCtx := cube.ParseDrillContextNormalized(rawQuery, h.emptyDrillValue)

	if h.resolveCacheKey != nil {
		keyID, err := h.resolveCacheKey(r.Context(), r)
		if err != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Host.Document", "cache key resolution failed", err)
			return
		}
		baseReq := h.normalizeRuntimeRequest(r, keyID.Request)
		cacheParams := strings.TrimSpace(keyID.CacheKey)
		if cacheParams == "" {
			cacheParams = canonicalQueryKey(drillCtx.WithValues(rawQuery))
		}
		key := h.reportKey(baseReq, keyID.Variant, cacheParams)
		cubeKey := h.cubeCacheKey(baseReq, keyID.Variant, keyID.CubeKey, drillCtx, rawQuery)
		reqSnapshot := r.Clone(r.Context())

		h.materializeReportAndWrite(w, r, key, func(buildCtx context.Context) (ReportDefinition[*hostEntry], error) {
			resolved, err := h.resolveCubeCached(buildCtx, reqSnapshot, drillCtx, cubeKey, false)
			if err != nil {
				return ReportDefinition[*hostEntry]{}, err
			}
			runtimeReq := h.normalizeRuntimeRequest(reqSnapshot, mergeRuntimeRequest(baseReq, resolved.Request))
			return h.buildCubeReportDefinition(buildCtx, resolved, drillCtx, rawQuery, runtimeReq)
		})
		return
	}

	resolved, err := h.resolveCube(r.Context(), r, drillCtx)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Document", "cube resolution failed", err)
		return
	}
	runtimeReq := h.normalizeRuntimeRequest(r, resolved.Request)
	cacheParams := strings.TrimSpace(resolved.CacheKey)
	if cacheParams == "" {
		cacheParams = canonicalQueryKey(drillCtx.WithValues(rawQuery))
	}
	key := h.reportKey(runtimeReq, resolved.Variant, cacheParams)
	cubeKey := h.cubeCacheKey(runtimeReq, resolved.Variant, resolved.CubeCacheKey, drillCtx, rawQuery)

	h.materializeReportAndWrite(w, r, key, func(buildCtx context.Context) (ReportDefinition[*hostEntry], error) {
		active := resolved
		if active.LoadSpec != nil {
			loaded, err := h.loadCubeSpecCached(buildCtx, active, cubeKey, false)
			if err != nil {
				return ReportDefinition[*hostEntry]{}, err
			}
			active = loaded
		}
		return h.buildCubeReportDefinition(buildCtx, active, drillCtx, rawQuery, runtimeReq)
	})
}

func (h *Host) buildCubeReportDefinition(
	ctx context.Context,
	resolved ResolvedCube,
	drillCtx cube.DrillContext,
	rawQuery url.Values,
	runtimeReq lensruntime.Request,
) (ReportDefinition[*hostEntry], error) {
	pagePath := strings.TrimSpace(resolved.PagePath)
	if pagePath == "" {
		pagePath = h.pagePath
	}
	spec, err := cube.ResolveWithContext(ctx, resolved.Spec, drillCtx, pagePath)
	if err != nil {
		return ReportDefinition[*hostEntry]{}, err
	}
	// Resolve human-readable labels for active drill filters (LabelMap / Dataset / SQL).
	if spec.Drill != nil && drillCtx.HasFilters() {
		if filters, labelErr := h.resolveCubeFilterLabels(ctx, resolved, drillCtx, runtimeReq); labelErr == nil {
			spec.Drill.Filters = filters
		}
	}

	facetFilters := h.buildCubeFacetFilters(resolved.Spec, spec.Drill, drillCtx, pagePath, rawQuery)
	docOpts := h.mergeDocumentOptions(resolved.DocumentOptions)
	prepare := func(built *document.DashboardDocument) error {
		if resolved.DocumentOptions.Filters == nil && len(facetFilters) > 0 {
			built.Filters = append(built.Filters, facetFilters...)
		}
		if resolved.Prepare != nil {
			if err := resolved.Prepare(built); err != nil {
				return err
			}
		}
		return built.Validate()
	}
	return h.newReportDefinition(spec, runtimeReq, docOpts, prepare), nil
}

func (h *Host) serveDashboardDocument(w http.ResponseWriter, r *http.Request) {
	if h.resolveCacheKey != nil {
		rawQuery := cloneValues(r.URL.Query())
		keyID, err := h.resolveCacheKey(r.Context(), r)
		if err != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Host.Document", "cache key resolution failed", err)
			return
		}
		baseReq := h.normalizeRuntimeRequest(r, keyID.Request)
		cacheParams := strings.TrimSpace(keyID.CacheKey)
		if cacheParams == "" {
			cacheParams = canonicalQueryKey(rawQuery)
		}
		key := h.reportKey(baseReq, keyID.Variant, cacheParams)
		reqSnapshot := r.Clone(r.Context())

		h.materializeReportAndWrite(w, r, key, func(buildCtx context.Context) (ReportDefinition[*hostEntry], error) {
			resolved, err := h.resolveDashboard(buildCtx, reqSnapshot.Clone(buildCtx))
			if err != nil {
				return ReportDefinition[*hostEntry]{}, err
			}
			runtimeReq := h.normalizeRuntimeRequest(reqSnapshot, mergeRuntimeRequest(baseReq, resolved.Request))
			docOpts := h.mergeDocumentOptions(resolved.DocumentOptions)
			prepare := func(built *document.DashboardDocument) error {
				if resolved.Prepare != nil {
					if err := resolved.Prepare(built); err != nil {
						return err
					}
				}
				return built.Validate()
			}
			return h.newReportDefinition(resolved.Spec, runtimeReq, docOpts, prepare), nil
		})
		return
	}

	resolved, err := h.resolveDashboard(r.Context(), r)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Document", "dashboard resolution failed", err)
		return
	}
	runtimeReq := h.normalizeRuntimeRequest(r, resolved.Request)
	cacheParams := strings.TrimSpace(resolved.CacheKey)
	if cacheParams == "" {
		cacheParams = canonicalQueryKey(r.URL.Query())
	}
	key := h.reportKey(runtimeReq, resolved.Variant, cacheParams)
	docOpts := h.mergeDocumentOptions(resolved.DocumentOptions)

	prepare := func(built *document.DashboardDocument) error {
		if resolved.Prepare != nil {
			if err := resolved.Prepare(built); err != nil {
				return err
			}
		}
		return built.Validate()
	}

	h.materializeReportAndWrite(w, r, key, func(context.Context) (ReportDefinition[*hostEntry], error) {
		return h.newReportDefinition(resolved.Spec, runtimeReq, docOpts, prepare), nil
	})
}

func (h *Host) newReportDefinition(
	spec lens.DashboardSpec,
	runtimeReq lensruntime.Request,
	docOpts document.BuildOptions,
	prepare func(*document.DashboardDocument) error,
) ReportDefinition[*hostEntry] {
	return ReportDefinition[*hostEntry]{
		Config: Config{
			Spec:              spec,
			Engine:            h.executor,
			Snapshots:         h.snapshots,
			BasePath:          h.basePath,
			InlineDepth:       h.inlineDepth,
			PageSize:          h.pageSize,
			WorkTimeout:       h.executionTimeout,
			RecomputeCooldown: h.recomputeCooldown,
			Progressive:       h.progressive,
			Observer:          h.observer,
			DrawerResolver:    h.drawerResolver,
			Exploration:       h.exploration,
			ResolveLoader:     h.resolveLoader,
			Request: func(req *http.Request) lensruntime.Request {
				if injected, ok := req.Context().Value(hostRuntimeRequestContextKey{}).(lensruntime.Request); ok {
					return injected
				}
				if resolved, resolveErr := h.resolveRuntimeRequest(req); resolveErr == nil {
					return resolved
				}
				return runtimeReq
			},
		},
		Request: runtimeReq,
		Options: docOpts,
		Prepare: prepare,
		State:   &hostEntry{progressive: h.progressive},
	}
}

func (h *Host) materializeReportAndWrite(
	w http.ResponseWriter,
	r *http.Request,
	key ReportKey,
	build func(context.Context) (ReportDefinition[*hostEntry], error),
) {
	report, err := h.cache.Materialize(r.Context(), key, build)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Document", "lens execution failed", err)
		return
	}
	defer report.Release()
	writeJSON(w, http.StatusOK, report.Document)
}

// Query proxies a drill/exploration level query to the snapshot-bound Handlers.
func (h *Host) Query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	body, ok := readBoundedBody(w, r)
	if !ok {
		return
	}
	var req QueryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "invalid JSON body")
		return
	}
	entry, err := h.cache.LookupSnapshotRegistration(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	defer entry.Release()
	current, err := h.resolveRuntimeRequest(r)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Query", "request scope resolution failed", err)
		return
	}
	proxy := r.Clone(context.WithValue(r.Context(), hostRuntimeRequestContextKey{}, current))
	proxy.Body = io.NopCloser(bytes.NewReader(body))
	entry.Handlers.Query(w, proxy)
}

// Panel proxies a progressive panel batch request to the snapshot-bound
// Handlers. When any panel requests Recompute, Host waits out any remaining
// recompute cooldown gracefully instead of failing recently-completed panels.
func (h *Host) Panel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	w.Header().Set("Content-Encoding", "identity")
	body, ok := readBoundedBody(w, r)
	if !ok {
		return
	}
	var req PanelBatchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "invalid JSON body")
		return
	}
	entry, err := h.cache.LookupSnapshotRegistration(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	defer entry.Release()
	if !entry.State.progressive {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "panel is not deferred")
		return
	}

	hasRecompute := false
	for _, panelReq := range req.Panels {
		if panelReq.Recompute {
			hasRecompute = true
			break
		}
	}
	reqCtx := r.Context()
	resolveReq := r
	if hasRecompute {
		reqCtx = WithRecompute(reqCtx)
		resolveReq = r.Clone(reqCtx)
	}
	if hasRecompute && h.recomputeCooldown > 0 {
		if err := h.waitForRecomputeCooldown(reqCtx, req.SnapshotID, req.Panels); err != nil {
			return
		}
	}
	current, err := h.resolveRuntimeRequest(resolveReq)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Panel", "request scope resolution failed", err)
		return
	}
	// Keep the request context unmarked here; Handlers.queuePanelValue sets the
	// per-job recompute marker from each PanelRequest.Recompute so non-recompute
	// siblings and background prefetch never inherit it.
	proxy := r.Clone(context.WithValue(r.Context(), hostRuntimeRequestContextKey{}, current))
	proxy.Body = io.NopCloser(bytes.NewReader(body))
	entry.Handlers.Panel(w, proxy)
}

func (h *Host) waitForRecomputeCooldown(ctx context.Context, snapshotID string, panels []PanelRequest) error {
	snapshot, err := h.snapshots.Get(ctx, snapshotID)
	if err != nil {
		// Let Handlers.Panel emit its canonical snapshot-gone response.
		return nil
	}
	now := time.Now()
	var wait time.Duration
	for _, panelReq := range panels {
		if !panelReq.Recompute {
			continue
		}
		calc, ok := snapshot.Panels[strings.TrimSpace(panelReq.PanelID)]
		if !ok || calc.CalculatedAt.IsZero() {
			continue
		}
		remaining := h.recomputeCooldown - now.Sub(calc.CalculatedAt)
		if remaining > wait {
			wait = remaining
		}
	}
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait + recomputeCooldownClockGrace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Release detaches a browser session from a snapshot's speculative queue.
func (h *Host) Release(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	body, ok := readBoundedBody(w, r)
	if !ok {
		return
	}
	var req document.ReleaseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "invalid JSON body")
		return
	}
	entry, err := h.cache.LookupSnapshotRegistration(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	defer entry.Release()
	current, err := h.resolveRuntimeRequest(r)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Release", "request scope resolution failed", err)
		return
	}
	proxy := r.Clone(context.WithValue(r.Context(), hostRuntimeRequestContextKey{}, current))
	proxy.Body = io.NopCloser(bytes.NewReader(body))
	entry.Handlers.Release(w, proxy)
}

// Drawer resolves a metric drawer URL against frozen snapshot state.
func (h *Host) Drawer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be POST")
		return
	}
	if h.drawerResolver == nil {
		writeError(w, http.StatusNotFound, document.QueryErrorBadRequest, "drawer resolver is unavailable")
		return
	}
	body, ok := readBoundedBody(w, r)
	if !ok {
		return
	}
	var req document.DrawerResolveRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "invalid JSON body")
		return
	}
	entry, err := h.cache.LookupSnapshotRegistration(r.Context(), req.SnapshotID)
	if err != nil {
		h.writeSnapshotError(r.Context(), w, err)
		return
	}
	defer entry.Release()
	current, err := h.resolveRuntimeRequest(r)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.Drawer", "request scope resolution failed", err)
		return
	}
	proxy := r.Clone(context.WithValue(r.Context(), hostRuntimeRequestContextKey{}, current))
	proxy.Body = io.NopCloser(bytes.NewReader(body))
	entry.Handlers.Drawer(w, proxy)
}

// FacetOptions serves searchable multi-select facet options in cube mode.
func (h *Host) FacetOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	if h.resolveCube == nil {
		writeError(w, http.StatusNotFound, document.QueryErrorBadRequest, "facet options require cube mode")
		return
	}
	dimension := strings.TrimSpace(r.URL.Query().Get(cube.QueryFacet))
	if dimension == "" {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "facet dimension is required")
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get(cube.QueryFacetSearch))
	if len([]rune(search)) > 200 {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "facet search cannot exceed 200 characters")
		return
	}
	rawQuery := cloneValues(r.URL.Query())
	drillCtx := cube.ParseDrillContextNormalized(rawQuery, h.emptyDrillValue)
	var (
		resolved   ResolvedCube
		runtimeReq lensruntime.Request
		err        error
	)
	if h.resolveCacheKey != nil {
		keyID, keyErr := h.resolveCacheKey(r.Context(), r)
		if keyErr != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Host.FacetOptions", "cache key resolution failed", keyErr)
			return
		}
		baseReq := h.normalizeRuntimeRequest(r, keyID.Request)
		cubeKey := h.cubeCacheKey(baseReq, keyID.Variant, keyID.CubeKey, drillCtx, rawQuery)
		resolved, err = h.resolveCubeCached(r.Context(), r, drillCtx, cubeKey, true)
		if err != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Host.FacetOptions", "cube resolution failed", err)
			return
		}
		runtimeReq = h.normalizeRuntimeRequest(r, mergeRuntimeRequest(baseReq, resolved.Request))
	} else {
		resolved, err = h.resolveCube(r.Context(), r, drillCtx)
		if err != nil {
			h.writeInternalError(r.Context(), w, "lens/serve.Host.FacetOptions", "cube resolution failed", err)
			return
		}
		runtimeReq = h.normalizeRuntimeRequest(r, resolved.Request)
		if resolved.LoadSpec != nil {
			cubeKey := h.cubeCacheKey(runtimeReq, resolved.Variant, resolved.CubeCacheKey, drillCtx, rawQuery)
			resolved, err = h.loadCubeSpecCached(r.Context(), resolved, cubeKey, true)
			if err != nil {
				h.writeInternalError(r.Context(), w, "lens/serve.Host.FacetOptions", "cube resolution failed", err)
				return
			}
		}
	}
	dim, ok := resolved.Spec.Dimension(dimension)
	if !ok {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "unknown facet dimension")
		return
	}
	if len(resolved.Spec.FacetDimensions()) > 0 && !dim.Facet {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "dimension is not configured as a facet")
		return
	}
	options, err := h.resolveCubeFacetOptions(r.Context(), resolved, drillCtx, dim, search, runtimeReq)
	if err != nil {
		h.writeInternalError(r.Context(), w, "lens/serve.Host.FacetOptions", "facet options failed", err)
		return
	}
	pagePath := strings.TrimSpace(resolved.PagePath)
	if pagePath == "" {
		pagePath = h.pagePath
	}
	normalizedQuery := drillCtx.WithValues(r.URL.Query())
	applyContext := drillCtx.WithoutDimension(dimension)
	applyContext.Leaf = false
	response := document.FacetOptionsResponse{
		Options:  make([]document.FacetOption, 0, len(options)),
		ApplyURL: joinURLQuery(pagePath, applyContext.WithValues(normalizedQuery)),
	}
	for _, option := range options {
		toggled := drillCtx.ToggleFilter(dimension, option.Value)
		toggled.Leaf = false
		response.Options = append(response.Options, document.FacetOption{
			Label:     option.Label,
			Value:     option.Value,
			Count:     option.Count,
			Selected:  option.Selected,
			ToggleURL: joinURLQuery(pagePath, toggled.WithValues(normalizedQuery)),
		})
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, response)
}

// ToggleFilter toggles one cube dimension filter value while preserving period
// and all unrelated query parameters, then redirects back to the target page.
func (h *Host) ToggleFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, document.QueryErrorBadRequest, "method must be GET")
		return
	}
	if h.resolveCube == nil {
		writeError(w, http.StatusNotFound, document.QueryErrorBadRequest, "toggle filter requires cube mode")
		return
	}
	targetURL, err := h.toggleFilterURL(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, targetURL, http.StatusFound)
}

func (h *Host) toggleFilterURL(raw url.Values) (string, error) {
	values := cloneValues(raw)
	targetPath := strings.TrimSpace(values.Get("lens_filter_target"))
	if targetPath == "" {
		targetPath = h.pagePath
	}
	dimension := strings.TrimSpace(values.Get("lens_filter_dimension"))
	_, hasValueParam := values["lens_filter_value"]
	value := strings.TrimSpace(values.Get("lens_filter_value"))
	if value == "" && hasValueParam {
		value = h.emptyDrillValue
	}
	values.Del("lens_filter_target")
	values.Del("lens_filter_dimension")
	values.Del("lens_filter_value")
	values.Del(lensruntime.TablePaginationPanelQuery)
	values.Del(lensruntime.TablePaginationPageQuery)
	values.Del(lensruntime.TablePaginationLimitQuery)

	if dimension == "" || value == "" {
		return "", fmt.Errorf("lens_filter_dimension and lens_filter_value are required")
	}
	target, err := url.Parse(targetPath)
	if err != nil || target.IsAbs() || target.Scheme != "" || target.Host != "" || target.User != nil ||
		target.RawQuery != "" || target.Fragment != "" ||
		hasControlRune(targetPath) || hasControlRune(target.Path) ||
		!strings.HasPrefix(target.Path, "/") || strings.HasPrefix(target.Path, "//") ||
		strings.ContainsRune(targetPath, '\\') || strings.ContainsRune(target.Path, '\\') {
		return "", fmt.Errorf("invalid filter toggle target")
	}
	cleaned := path.Clean(target.Path)
	if !strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "//") ||
		strings.ContainsRune(cleaned, '\\') || cleaned == h.endpoint("/toggle-filter") {
		return "", fmt.Errorf("invalid filter toggle target")
	}
	drillCtx := cube.ParseDrillContextNormalized(values, h.emptyDrillValue).ToggleFilter(dimension, value)
	drillCtx.Leaf = false
	escapedPath := (&url.URL{Path: cleaned}).EscapedPath()
	encodedQuery := drillCtx.WithValues(values).Encode()
	if encodedQuery == "" {
		return escapedPath, nil
	}
	return escapedPath + "?" + encodedQuery, nil
}

func (h *Host) resolveCubeFacetOptions(
	ctx context.Context,
	resolved ResolvedCube,
	drillCtx cube.DrillContext,
	dim cube.DimensionSpec,
	search string,
	runtimeReq lensruntime.Request,
) ([]lens.DrillFacetOptionMeta, error) {
	switch resolved.Spec.DataMode {
	case cube.DataModeDataset:
		return cube.ResolveFacetOptions(ctx, resolved.Spec, drillCtx, dim.Name, search, h.facetLimit, nil)
	case cube.DataModeSQL:
		ds := resolved.DataSource
		if ds == nil && runtimeReq.DataSources != nil {
			ds = runtimeReq.DataSources[resolved.Spec.DataSource]
		}
		if ds == nil {
			return nil, fmt.Errorf("datasource %q not configured", resolved.Spec.DataSource)
		}
		return cube.ResolveFacetOptions(ctx, resolved.Spec, drillCtx, dim.Name, search, h.facetLimit,
			func(ctx context.Context, text string, params map[string]lens.ParamValue, maxRows int) ([]lens.DrillFacetOptionMeta, error) {
				boundParams, paramErr := resolveFacetParams(resolved.Spec.Variables, params, runtimeReq)
				if paramErr != nil {
					return nil, paramErr
				}
				frames, err := ds.Run(ctx, datasource.QueryRequest{
					Source:   resolved.Spec.DataSource,
					Text:     text,
					Params:   boundParams,
					Timezone: runtimeReq.Timezone,
					Locale:   runtimeReq.Locale,
					Kind:     datasource.QueryKindRaw,
					MaxRows:  maxRows,
				})
				if err != nil || frames == nil || frames.Primary() == nil {
					return nil, err
				}
				rows := frames.Primary().Rows()
				out := make([]lens.DrillFacetOptionMeta, 0, len(rows))
				for _, row := range rows {
					val := strings.TrimSpace(fmt.Sprint(row["value"]))
					if val == "" || val == "<nil>" {
						continue
					}
					lbl := strings.TrimSpace(fmt.Sprint(row["label"]))
					if lbl == "" || lbl == "<nil>" {
						lbl = dim.DisplayLabel(val)
					}
					out = append(out, lens.DrillFacetOptionMeta{
						Dimension: dim.Name,
						Value:     val,
						Label:     lbl,
						Count:     coerceInt(row["count"]),
					})
				}
				return out, nil
			},
		)
	case cube.DataModeNamed:
		ds := resolved.DataSource
		if ds == nil && runtimeReq.DataSources != nil {
			ds = runtimeReq.DataSources[resolved.Spec.DataSource]
		}
		if ds == nil {
			return nil, fmt.Errorf("datasource %q not configured", resolved.Spec.DataSource)
		}
		return cube.ResolveFacetOptions(ctx, resolved.Spec, drillCtx, dim.Name, search, h.facetLimit,
			func(ctx context.Context, queryName string, params map[string]lens.ParamValue, _ int) ([]lens.DrillFacetOptionMeta, error) {
				boundParams, facetTimeRange, paramErr := resolveFacetQueryInputs(resolved.Spec.Variables, params, runtimeReq)
				if paramErr != nil {
					return nil, paramErr
				}
				frames, err := ds.Run(ctx, datasource.QueryRequest{
					Source:    resolved.Spec.DataSource,
					Text:      queryName,
					Params:    boundParams,
					TimeRange: facetTimeRange,
					Timezone:  runtimeReq.Timezone,
					Locale:    runtimeReq.Locale,
					Kind:      datasource.QueryKindNamed,
				})
				if err != nil {
					return nil, err
				}
				return cube.ResolveFacetOptionsFromFrames(resolved.Spec, frames, drillCtx, dim, search, h.facetLimit), nil
			},
		)
	default:
		return nil, fmt.Errorf("unsupported cube mode %q", resolved.Spec.DataMode)
	}
}

func (h *Host) resolveCubeFilterLabels(
	ctx context.Context,
	resolved ResolvedCube,
	drillCtx cube.DrillContext,
	runtimeReq lensruntime.Request,
) ([]lens.DrillFilterMeta, error) {
	var lookup cube.SQLLabelLookup
	if resolved.Spec.DataMode == cube.DataModeSQL {
		ds := resolved.DataSource
		if ds == nil && runtimeReq.DataSources != nil {
			ds = runtimeReq.DataSources[resolved.Spec.DataSource]
		}
		if ds != nil {
			lookup = func(ctx context.Context, text string, params map[string]any) (string, bool, error) {
				frames, err := ds.Run(ctx, datasource.QueryRequest{
					Source:   resolved.Spec.DataSource,
					Text:     text,
					Params:   params,
					Timezone: runtimeReq.Timezone,
					Locale:   runtimeReq.Locale,
					Kind:     datasource.QueryKindRaw,
					MaxRows:  1,
				})
				if err != nil || frames == nil || frames.Primary() == nil || frames.Primary().RowCount == 0 {
					return "", false, err
				}
				rows := frames.Primary().Rows()
				if len(rows) == 0 {
					return "", false, nil
				}
				label := strings.TrimSpace(fmt.Sprint(rows[0]["label"]))
				if label == "" || label == "<nil>" {
					return "", false, nil
				}
				return label, true, nil
			}
		}
	}
	return cube.ResolveDrillFilters(ctx, resolved.Spec, drillCtx, lookup)
}

func (h *Host) buildCubeFacetFilters(
	spec cube.CubeSpec,
	drillMeta *lens.DrillMeta,
	drillCtx cube.DrillContext,
	pagePath string,
	rawQuery url.Values,
) []document.Filter {
	facetDims := spec.FacetDimensions()
	if len(facetDims) == 0 {
		return nil
	}
	displayByKey := make(map[string]string)
	if drillMeta != nil {
		for _, f := range drillMeta.Filters {
			displayByKey[f.Dimension+"\x00"+f.Value] = f.Display
		}
	}
	normalized := drillCtx.WithValues(rawQuery)
	out := make([]document.Filter, 0, len(facetDims))
	for _, dim := range facetDims {
		optsQuery := cloneValues(normalized)
		optsQuery.Set(cube.QueryFacet, dim.Name)
		optsQuery.Del(cube.QueryFacetSearch)
		facet := &document.FacetFilter{
			Dimension:       dim.Name,
			OptionsEndpoint: joinURLQuery(h.endpoint("/facet-options"), optsQuery),
			SearchParam:     cube.QueryFacetSearch,
		}
		for _, filter := range drillCtx.Filters {
			if filter.Dimension != dim.Name {
				continue
			}
			vals := filter.Values
			if len(vals) == 0 && strings.TrimSpace(filter.Value) != "" {
				vals = []string{filter.Value}
			}
			for _, value := range vals {
				label := displayByKey[dim.Name+"\x00"+value]
				if strings.TrimSpace(label) == "" {
					label = dim.DisplayLabel(value)
				}
				removed := drillCtx.ToggleFilter(dim.Name, value)
				removed.Leaf = false
				facet.Selections = append(facet.Selections, document.FacetSelection{
					Label:     label,
					RemoveURL: joinURLQuery(pagePath, removed.WithValues(normalized)),
				})
			}
		}
		if len(facet.Selections) > 0 {
			cleared := drillCtx.WithoutDimension(dim.Name)
			cleared.Leaf = false
			facet.ClearURL = joinURLQuery(pagePath, cleared.WithValues(normalized))
		}
		out = append(out, document.Filter{
			ID:    "facet_" + dim.Name,
			Kind:  document.FilterKindFacet,
			Label: dim.Label,
			Facet: facet,
		})
	}
	return out
}

func (h *Host) mergeDocumentOptions(opts document.BuildOptions) document.BuildOptions {
	if opts.Endpoints == (document.Endpoints{}) {
		opts.Endpoints = document.Endpoints{
			Query:   h.endpoint("/query"),
			Export:  h.endpoint("/export"),
			Release: h.endpoint("/release"),
			Panel: func() string {
				if h.progressive {
					return h.endpoint("/panel")
				}
				return ""
			}(),
			Drawer: func() string {
				if h.drawerResolver != nil {
					return h.endpoint("/drawer")
				}
				return ""
			}(),
		}
	}
	return opts
}

func (h *Host) reportKey(req lensruntime.Request, variant, params string) ReportKey {
	namespace := strings.TrimSpace(req.Namespace)
	if namespace == "" {
		namespace = h.basePath
		if namespace == "" {
			namespace = "/"
		}
	}
	keyVariant := "progressive:" + strconv.FormatBool(h.progressive)
	if strings.TrimSpace(variant) != "" {
		keyVariant += "|" + strings.TrimSpace(variant)
	}
	return ReportKey{
		Namespace: namespace,
		DataScope: req.DataScope,
		Version:   h.cacheVersion,
		Locale:    req.Locale,
		Timezone:  req.Timezone,
		Variant:   keyVariant,
		Params:    params,
	}
}

func (h *Host) resolveRuntimeRequest(r *http.Request) (lensruntime.Request, error) {
	if h.request != nil {
		return h.normalizeRuntimeRequest(r, h.request(r)), nil
	}
	if h.resolveCacheKey != nil {
		keyID, err := h.resolveCacheKey(r.Context(), r)
		if err != nil {
			return lensruntime.Request{}, err
		}
		return h.normalizeRuntimeRequest(r, keyID.Request), nil
	}
	if h.resolveCube != nil {
		drillCtx := cube.ParseDrillContextNormalized(r.URL.Query(), h.emptyDrillValue)
		resolved, err := h.resolveCube(r.Context(), r, drillCtx)
		if err != nil {
			return lensruntime.Request{}, err
		}
		return h.normalizeRuntimeRequest(r, resolved.Request), nil
	}
	if h.resolveDashboard != nil {
		resolved, err := h.resolveDashboard(r.Context(), r)
		if err != nil {
			return lensruntime.Request{}, err
		}
		return h.normalizeRuntimeRequest(r, resolved.Request), nil
	}
	return h.normalizeRuntimeRequest(r, lensruntime.Request{}), nil
}

func (h *Host) cubeCacheKey(
	req lensruntime.Request,
	variant string,
	explicitCubeKey string,
	drillCtx cube.DrillContext,
	rawQuery url.Values,
) ReportKey {
	params := strings.TrimSpace(explicitCubeKey)
	if params == "" {
		cleaned := cloneValues(drillCtx.WithValues(rawQuery))
		cleaned.Del(cube.QueryFacet)
		cleaned.Del(cube.QueryFacetSearch)
		params = canonicalQueryKey(cleaned)
	}
	return h.reportKey(req, variant, params)
}

func (h *Host) resolveCubeCached(
	ctx context.Context,
	r *http.Request,
	drillCtx cube.DrillContext,
	cubeKey ReportKey,
	admit bool,
) (ResolvedCube, error) {
	baseReq := r.Clone(ctx)
	return h.cubeCache.getOrLoad(ctx, cubeKey, admit, func(loadCtx context.Context) (ResolvedCube, error) {
		resolved, err := h.resolveCube(loadCtx, baseReq.Clone(loadCtx), drillCtx)
		if err != nil {
			return ResolvedCube{}, err
		}
		if resolved.LoadSpec != nil {
			spec, err := resolved.LoadSpec(loadCtx)
			if err != nil {
				return ResolvedCube{}, err
			}
			resolved.Spec = spec
			resolved.LoadSpec = nil
		}
		return resolved, nil
	})
}

func (h *Host) loadCubeSpecCached(
	ctx context.Context,
	resolved ResolvedCube,
	cubeKey ReportKey,
	admit bool,
) (ResolvedCube, error) {
	if resolved.LoadSpec == nil {
		return resolved, nil
	}
	loadFn := resolved.LoadSpec
	cached, err := h.cubeCache.getOrLoad(ctx, cubeKey, admit, func(loadCtx context.Context) (ResolvedCube, error) {
		spec, err := loadFn(loadCtx)
		if err != nil {
			return ResolvedCube{}, err
		}
		out := resolved
		out.Spec = spec
		out.LoadSpec = nil
		return out, nil
	})
	if err != nil {
		return ResolvedCube{}, err
	}
	out := resolved
	out.Spec = cached.Spec
	out.LoadSpec = nil
	return out, nil
}

func mergeRuntimeRequest(base, override lensruntime.Request) lensruntime.Request {
	out := base
	if override.DataScope != "" {
		out.DataScope = override.DataScope
	}
	if override.Namespace != "" {
		out.Namespace = override.Namespace
	}
	if override.Locale != "" {
		out.Locale = override.Locale
	}
	if override.Timezone != "" {
		out.Timezone = override.Timezone
	}
	if override.Path != "" {
		out.Path = override.Path
	}
	if override.Request != nil {
		out.Request = override.Request
	}
	if override.DataSources != nil {
		out.DataSources = override.DataSources
	}
	if override.Overrides != nil {
		out.Overrides = override.Overrides
	}
	return out
}

func (h *Host) normalizeRuntimeRequest(r *http.Request, req lensruntime.Request) lensruntime.Request {
	if req.Locale == "" {
		if locale, ok := intl.UseLocale(r.Context()); ok {
			req.Locale = locale.String()
		}
	}
	if req.Path == "" {
		req.Path = r.URL.Path
	}
	if req.Request == nil {
		req.Request = r.URL.Query()
	}
	if strings.TrimSpace(req.Namespace) == "" && h.basePath != "" {
		req.Namespace = h.basePath
	}
	return req
}

func (h *Host) endpoint(suffix string) string {
	return h.basePath + suffix
}

func (h *Host) writeSnapshotError(ctx context.Context, w http.ResponseWriter, err error) {
	if ctx.Err() != nil {
		return
	}
	if errors.Is(err, document.ErrSnapshotGone) {
		writeError(w, http.StatusGone, document.QueryErrorSnapshotGone, "snapshot is unknown or expired")
		return
	}
	h.writeInternalError(ctx, w, "lens/serve.Host", "snapshot lookup failed", err)
}

func (h *Host) writeInternalError(ctx context.Context, w http.ResponseWriter, op, message string, err error) {
	if ctx.Err() != nil {
		return
	}
	if status, ok := resource.StatusCode(err); ok {
		h.observer.OnError(ctx, op, serrors.Wrap(serrors.Op(op), err))
		writeError(w, status, document.QueryErrorResourceExhausted, "report resource budget exceeded")
		return
	}
	wrapped := serrors.Wrap(serrors.Op(op), err)
	h.observer.OnError(ctx, op, wrapped)
	writeJSON(w, http.StatusInternalServerError, errorResponse{
		Error:   document.QueryErrorInternal,
		Message: message,
		Reason:  document.ClassifyFailure(err),
	})
}

func readBoundedBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxQueryBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "failed to read request body")
		return nil, false
	}
	if len(body) > maxQueryBodyBytes {
		writeError(w, http.StatusBadRequest, document.QueryErrorBadRequest, "request body too large")
		return nil, false
	}
	return body, true
}

func defaultPagePath(basePath string) string {
	if basePath == "" {
		return "/"
	}
	if strings.HasSuffix(basePath, "/lens") {
		trimmed := strings.TrimSuffix(basePath, "/lens")
		if trimmed == "" {
			return "/"
		}
		return trimmed
	}
	return basePath
}

func joinURLQuery(base string, query url.Values) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "/"
	}
	cleaned := path.Clean(base)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if len(query) == 0 {
		return cleaned
	}
	return cleaned + "?" + query.Encode()
}

func hasControlRune(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func canonicalQueryKey(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		for j, item := range values[k] {
			if j > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(item))
		}
	}
	return b.String()
}

func resolveFacetParams(variables []lens.VariableSpec, params map[string]lens.ParamValue, req lensruntime.Request) (map[string]any, error) {
	bound, _, err := resolveFacetQueryInputs(variables, params, req)
	return bound, err
}

func resolveFacetQueryInputs(variables []lens.VariableSpec, params map[string]lens.ParamValue, req lensruntime.Request) (map[string]any, datasource.TimeRange, error) {
	var resolvedVars map[string]any
	if len(variables) > 0 {
		var err error
		resolvedVars, err = lensruntime.ResolveVariables(variables, req)
		if err != nil {
			return nil, datasource.TimeRange{}, err
		}
	}
	timeRange := resolveFacetTimeRange(variables, resolvedVars)
	if len(params) == 0 {
		return map[string]any{}, timeRange, nil
	}
	out := make(map[string]any, len(params))
	for key, param := range params {
		if param.Literal != nil {
			out[key] = param.Literal
			continue
		}
		if param.Variable != "" {
			if resolvedVars != nil {
				if val, ok := resolvedVars[param.Variable]; ok {
					out[key] = resolveBoundVariableParam(val, param.Part)
					continue
				}
			}
			if req.Overrides != nil {
				if val, ok := req.Overrides[param.Variable]; ok {
					out[key] = resolveBoundVariableParam(val, param.Part)
					continue
				}
			}
			if req.Request != nil {
				if val := strings.TrimSpace(req.Request.Get(param.Variable)); val != "" {
					out[key] = val
					continue
				}
				if canonical := lens.CanonicalRequestKey(param.Variable); canonical != param.Variable {
					if val := strings.TrimSpace(req.Request.Get(canonical)); val != "" {
						out[key] = val
						continue
					}
				}
			}
		}
		out[key] = nil
	}
	return out, timeRange, nil
}

func resolveFacetTimeRange(specs []lens.VariableSpec, variables map[string]any) datasource.TimeRange {
	for _, spec := range specs {
		value, ok := variables[spec.Name]
		if !ok {
			continue
		}
		if timeRange := lens.ResolveTimeRange(value); timeRange.Mode != "" {
			return timeRange
		}
	}
	return datasource.TimeRange{}
}

func resolveBoundVariableParam(val any, part lens.RangePart) any {
	if part != "" {
		if partVal, ok := lens.ResolveRangePart(val, part); ok {
			return partVal
		}
	}
	if comparison, ok := val.(lens.CompareValue); ok {
		return comparison.Range
	}
	return val
}

func coerceInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}
