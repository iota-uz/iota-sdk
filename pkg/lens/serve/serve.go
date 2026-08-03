package serve

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/engine"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	defaultPageSize    = 50
	defaultWorkTimeout = 2 * time.Minute
	defaultConcurrency = 4
)

// RequestResolver supplies host-owned runtime inputs such as data sources and
// tenant scope. Serve fills empty transport fields from the HTTP request.
type RequestResolver func(*http.Request) lensruntime.Request

type DrawerResolver func(*http.Request, document.DrawerResolveRequest, lensruntime.Request) (string, error)

// Observer receives internal serve errors before a generic response is written.
type Observer interface {
	OnError(context.Context, string, error)
}

type MetricName string

const (
	MetricTimeToFirstUsefulKPI MetricName = "time_to_first_useful_kpi_ms"
	MetricTimeToFirstChild     MetricName = "time_to_first_child_frame_ms"
	MetricPrefetchHit          MetricName = "prefetch_hit"
	MetricCancelledSpeculation MetricName = "cancelled_speculative_work"
	MetricRedundantWork        MetricName = "redundant_work"
	MetricSchedulerSaturation  MetricName = "scheduler_saturation"
)

type Metric struct {
	Name   MetricName
	Value  float64
	Labels map[string]string
}

// MetricsObserver is an optional extension implemented by Observer values
// that consume execution-session telemetry.
type MetricsObserver interface {
	OnMetric(context.Context, Metric)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(context.Context, string, error)

func (f ObserverFunc) OnError(ctx context.Context, op string, err error) {
	f(ctx, op, err)
}

// Config describes one host-registered dashboard HTTP surface.
type Config struct {
	Spec           lens.DashboardSpec
	Engine         engine.Executor
	Snapshots      document.SnapshotStore
	BasePath       string
	InlineDepth    int
	PageSize       int
	WorkTimeout    time.Duration
	Request        RequestResolver
	Observer       Observer
	DrawerResolver DrawerResolver
	// Progressive returns a layout-only document and materialises each panel
	// independently through Handlers.Panel.
	Progressive bool
}

// Handlers serves one dashboard registration.
type Handlers struct {
	spec           lens.DashboardSpec
	plan           lens.ExecutionPlan
	engine         engine.Executor
	snapshots      document.SnapshotStore
	basePath       string
	inlineDepth    int
	pageSize       int
	workTimeout    time.Duration
	request        RequestResolver
	observer       Observer
	progressive    bool
	drawerResolver DrawerResolver
	sessionsMu     sync.Mutex
	sessions       map[string]*executionSession
}

type noopObserver struct{}

func (noopObserver) OnError(context.Context, string, error) {}

func (h *Handlers) observeMetric(ctx context.Context, metric Metric) {
	recordMetric(ctx, metric)
	if observer, ok := h.observer.(MetricsObserver); ok {
		observer.OnMetric(ctx, metric)
	}
}

// New validates cfg and constructs the dashboard handlers.
func New(cfg Config) (*Handlers, error) {
	const op serrors.Op = "lens/serve.New"
	if cfg.Engine == nil {
		return nil, serrors.E(op, fmt.Errorf("lens executor is required"))
	}
	if cfg.Snapshots == nil {
		return nil, serrors.E(op, fmt.Errorf("snapshot store is required"))
	}
	if cfg.InlineDepth < 0 {
		return nil, serrors.E(op, fmt.Errorf("inline depth cannot be negative"))
	}
	if cfg.PageSize < 0 {
		return nil, serrors.E(op, fmt.Errorf("page size cannot be negative"))
	}
	if err := lensruntime.Validate(cfg.Spec); err != nil {
		return nil, serrors.E(op, err)
	}
	plan, err := lens.CompileExecutionPlan(cfg.Spec)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	basePath, err := normalizeBasePath(cfg.BasePath)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	pageSize := cfg.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	workTimeout := cfg.WorkTimeout
	if workTimeout <= 0 {
		workTimeout = defaultWorkTimeout
	}
	observer := cfg.Observer
	if observer == nil {
		observer = noopObserver{}
	}
	return &Handlers{
		spec: cfg.Spec, plan: plan, engine: cfg.Engine, snapshots: cfg.Snapshots,
		basePath: basePath, inlineDepth: cfg.InlineDepth, pageSize: pageSize, workTimeout: workTimeout,
		request: cfg.Request, observer: observer, progressive: cfg.Progressive,
		drawerResolver: cfg.DrawerResolver,
		sessions:       make(map[string]*executionSession),
	}, nil
}

func normalizeBasePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("base path must start with a slash")
	}
	if strings.ContainsAny(value, "?#") {
		return "", fmt.Errorf("base path cannot contain a query or fragment")
	}
	return strings.TrimSuffix(path.Clean(value), "/"), nil
}

func (h *Handlers) endpoint(suffix string) string {
	return h.basePath + suffix
}

func (h *Handlers) runtimeRequest(r *http.Request) lensruntime.Request {
	var req lensruntime.Request
	if h.request != nil {
		req = h.request(r)
	}
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
	return req
}
