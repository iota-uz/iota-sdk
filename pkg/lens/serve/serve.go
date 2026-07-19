package serve

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/engine"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"golang.org/x/sync/singleflight"
)

const defaultPageSize = 50

// RequestResolver supplies host-owned runtime inputs such as data sources and
// tenant scope. Serve fills empty transport fields from the HTTP request.
type RequestResolver func(*http.Request) lensruntime.Request

// Config describes one host-registered dashboard HTTP surface.
type Config struct {
	Spec        lens.DashboardSpec
	Engine      engine.Executor
	Snapshots   document.SnapshotStore
	BasePath    string
	InlineDepth int
	PageSize    int
	Request     RequestResolver
}

// Handlers serves one dashboard registration.
type Handlers struct {
	spec        lens.DashboardSpec
	engine      engine.Executor
	snapshots   document.SnapshotStore
	basePath    string
	inlineDepth int
	pageSize    int
	request     RequestResolver
	loads       singleflight.Group
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
	basePath, err := normalizeBasePath(cfg.BasePath)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	pageSize := cfg.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	return &Handlers{
		spec: cfg.Spec, engine: cfg.Engine, snapshots: cfg.Snapshots,
		basePath: basePath, inlineDepth: cfg.InlineDepth, pageSize: pageSize, request: cfg.Request,
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
