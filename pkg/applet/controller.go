package applet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

var htmlNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var mimeTypes = map[string]string{
	".js":    "application/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

type AppletController struct {
	applet  Applet
	builder *ContextBuilder
	logger  *logrus.Logger

	assetsBasePath string
	resolvedAssets *ResolvedAssets
	assetsErr      error

	devAssets *DevAssetConfig
}

func NewAppletController(
	applet Applet,
	bundle *i18n.Bundle,
	sessionConfig SessionConfig,
	logger *logrus.Logger,
	metrics MetricsRecorder,
	opts ...BuilderOption,
) *AppletController {
	config := applet.Config()
	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics, opts...)

	c := &AppletController{
		applet:  applet,
		builder: builder,
		logger:  logger,
	}
	c.initAssets()
	return c
}

func (c *AppletController) Register(router *mux.Router) {
	c.RegisterRoutes(router)
}

func (c *AppletController) Key() string {
	return "applet_" + c.applet.Name()
}

func (c *AppletController) RegisterRoutes(router *mux.Router) {
	config := c.applet.Config()

	if config.Assets.BasePath == "" {
		config.Assets.BasePath = "/assets"
	}
	if !strings.HasPrefix(config.Assets.BasePath, "/") {
		config.Assets.BasePath = "/" + config.Assets.BasePath
	}

	fullAssetsPath := path.Join(c.applet.BasePath(), config.Assets.BasePath)
	if c.devAssets != nil || config.Assets.FS != nil {
		c.registerAssetRoutes(router, fullAssetsPath)
	}

	appletRouter := router.PathPrefix(c.applet.BasePath()).Subrouter()

	if config.Middleware != nil {
		for _, middleware := range config.Middleware {
			appletRouter.Use(middleware)
		}
	}

	if config.RPC != nil {
		rpcPath := strings.TrimSpace(config.RPC.Path)
		if rpcPath == "" {
			rpcPath = "/rpc"
		}
		if !strings.HasPrefix(rpcPath, "/") {
			rpcPath = "/" + rpcPath
		}
		appletRouter.HandleFunc(rpcPath, c.handleRPC).Methods(http.MethodPost)
	}

	for _, p := range config.RoutePatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		appletRouter.HandleFunc(p, c.RenderApp).Methods(http.MethodGet, http.MethodHead)
	}

	appletRouter.HandleFunc("", c.RenderApp).Methods(http.MethodGet, http.MethodHead)
	appletRouter.HandleFunc("/", c.RenderApp).Methods(http.MethodGet, http.MethodHead)
	appletRouter.PathPrefix("/").HandlerFunc(c.RenderApp).Methods(http.MethodGet, http.MethodHead)
}

func (c *AppletController) initAssets() {
	const op serrors.Op = "applet.Controller.initAssets"

	config := c.applet.Config()

	assetsPath := strings.TrimSpace(config.Assets.BasePath)
	if assetsPath == "" {
		assetsPath = "/assets"
	}
	if !strings.HasPrefix(assetsPath, "/") {
		assetsPath = "/" + assetsPath
	}
	c.assetsBasePath = path.Join("/", strings.TrimPrefix(c.applet.BasePath(), "/"), strings.TrimPrefix(assetsPath, "/"))

	if config.Assets.Dev != nil && config.Assets.Dev.Enabled {
		dev := *config.Assets.Dev
		if dev.ClientModule == "" {
			dev.ClientModule = "/@vite/client"
		}
		if dev.StripPrefix == nil {
			v := true
			dev.StripPrefix = &v
		}
		c.devAssets = &dev
		return
	}

	if config.Assets.FS == nil {
		c.assetsErr = serrors.E(op, serrors.Invalid, "assets FS is required when dev proxy is disabled")
		return
	}
	if strings.TrimSpace(config.Assets.ManifestPath) == "" || strings.TrimSpace(config.Assets.Entrypoint) == "" {
		c.assetsErr = serrors.E(op, serrors.Invalid, "assets ManifestPath and Entrypoint are required when dev proxy is disabled")
		return
	}

	manifest, err := loadManifest(config.Assets.FS, config.Assets.ManifestPath)
	if err != nil {
		c.assetsErr = serrors.E(op, err)
		return
	}
	resolved, err := resolveAssetsFromManifest(manifest, config.Assets.Entrypoint, c.assetsBasePath)
	if err != nil {
		c.assetsErr = serrors.E(op, err)
		return
	}
	c.resolvedAssets = resolved
}

func (c *AppletController) registerAssetRoutes(router *mux.Router, fullAssetsPath string) {
	config := c.applet.Config()

	if c.devAssets != nil {
		c.registerDevProxy(router, fullAssetsPath, c.devAssets)
		return
	}

	fileServer := http.FileServer(http.FS(config.Assets.FS))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mimeType, ok := mimeTypes[filepath.Ext(r.URL.Path)]; ok {
			w.Header().Set("Content-Type", mimeType)
		}
		fileServer.ServeHTTP(w, r)
	})

	router.PathPrefix(fullAssetsPath).Handler(
		http.StripPrefix(fullAssetsPath, handler),
	)
}

func (c *AppletController) registerDevProxy(router *mux.Router, fullAssetsPath string, dev *DevAssetConfig) {
	const op serrors.Op = "applet.Controller.registerDevProxy"

	targetStr := strings.TrimSpace(dev.TargetURL)
	if targetStr == "" {
		c.assetsErr = serrors.E(op, serrors.Invalid, "assets dev proxy TargetURL is required")
		return
	}

	targetURL, err := url.Parse(targetStr)
	if err != nil {
		c.assetsErr = serrors.E(op, serrors.Invalid, "invalid assets dev proxy TargetURL", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host

		p := req.URL.Path
		if dev.StripPrefix == nil || *dev.StripPrefix {
			p = strings.TrimPrefix(p, fullAssetsPath)
			if p == "" {
				p = "/"
			}
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
		}

		req.URL.Path = singleJoiningSlash(targetURL.Path, p)
		req.URL.RawPath = req.URL.Path
	}

	router.PathPrefix(fullAssetsPath).Handler(proxy)
}

func (c *AppletController) RenderApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	initialContext, err := c.builder.Build(ctx, r, c.applet.BasePath())
	if err != nil {
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	contextJSON, err := json.Marshal(initialContext)
	if err != nil {
		http.Error(w, "Failed to serialize context", http.StatusInternalServerError)
		return
	}

	c.render(ctx, w, r, contextJSON)
}

func (c *AppletController) render(ctx context.Context, w http.ResponseWriter, r *http.Request, contextJSON []byte) {
	config := c.applet.Config()

	if c.assetsErr != nil {
		if c.logger != nil {
			c.logger.WithError(c.assetsErr).Error("applet assets misconfigured")
		}
		http.Error(w, "Applet assets misconfigured", http.StatusInternalServerError)
		return
	}

	contextScript, err := buildSafeContextScript(config.WindowGlobal, contextJSON)
	if err != nil {
		http.Error(w, "Failed to inject context", http.StatusInternalServerError)
		return
	}

	mountHTML := buildMountElement(config.Mount)

	cssLinks, jsScripts, err := c.buildAssetTags()
	if err != nil {
		if c.logger != nil {
			c.logger.WithError(err).Error("failed to build asset tags")
		}
		http.Error(w, "Failed to resolve applet assets", http.StatusInternalServerError)
		return
	}

	switch config.Shell.Mode {
	case ShellModeEmbedded:
		if config.Shell.Layout == nil {
			http.Error(w, "Applet shell layout is required", http.StatusInternalServerError)
			return
		}

		title := strings.TrimSpace(config.Shell.Title)
		if title == "" {
			title = c.applet.Name()
		}

		existingHead, ok := ctx.Value(constants.HeadKey).(templ.Component)
		if !ok || existingHead == nil {
			existingHead = templ.NopComponent
			ctx = context.WithValue(ctx, constants.HeadKey, existingHead)
		}

		if cssLinks != "" {
			mergedHead := templ.ComponentFunc(func(headCtx context.Context, wr io.Writer) error {
				if err := existingHead.Render(headCtx, wr); err != nil {
					return err
				}
				return templ.Raw(cssLinks).Render(headCtx, wr)
			})
			ctx = context.WithValue(ctx, constants.HeadKey, mergedHead)
		}

		shell := templ.ComponentFunc(func(shellCtx context.Context, wr io.Writer) error {
			if _, err := io.WriteString(wr, mountHTML); err != nil {
				return err
			}
			if _, err := io.WriteString(wr, contextScript); err != nil {
				return err
			}
			if _, err := io.WriteString(wr, jsScripts); err != nil {
				return err
			}
			return nil
		})

		ctx = templ.WithChildren(ctx, shell)
		layout := config.Shell.Layout(title)
		templ.Handler(layout, templ.WithStreaming()).ServeHTTP(w, r.WithContext(ctx))
		return

	case ShellModeStandalone:
		title := strings.TrimSpace(config.Shell.Title)
		if title == "" {
			title = c.applet.Name()
		}

		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  %s
</head>
<body>
  %s
  %s
  %s
</body>
</html>`,
			template.HTMLEscapeString(title),
			cssLinks,
			mountHTML,
			contextScript,
			jsScripts,
		)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
		return

	default:
		http.Error(w, "Invalid applet shell mode", http.StatusInternalServerError)
		return
	}
}

func (c *AppletController) buildAssetTags() (string, string, error) {
	const op serrors.Op = "applet.Controller.buildAssetTags"

	if c.devAssets != nil {
		clientModule := strings.TrimSpace(c.devAssets.ClientModule)
		if clientModule == "" {
			clientModule = "/@vite/client"
		}
		entryModule := strings.TrimSpace(c.devAssets.EntryModule)
		if entryModule == "" {
			return "", "", serrors.E(op, serrors.Invalid, "assets dev proxy EntryModule is required")
		}

		clientSrc := joinURLPath(c.assetsBasePath, clientModule)
		entrySrc := joinURLPath(c.assetsBasePath, entryModule)
		js := fmt.Sprintf(
			`<script type="module" src="%s"></script><script type="module" src="%s"></script>`,
			template.HTMLEscapeString(clientSrc),
			template.HTMLEscapeString(entrySrc),
		)
		return "", js, nil
	}

	if c.resolvedAssets == nil {
		return "", "", serrors.E(op, serrors.Internal, "assets not resolved")
	}
	return buildCSSLinks(c.resolvedAssets.CSSFiles), buildJSScripts(c.resolvedAssets.JSFiles), nil
}

func buildMountElement(config MountConfig) string {
	tag := strings.TrimSpace(config.Tag)
	id := strings.TrimSpace(config.ID)
	attrs := config.Attributes

	if tag == "" {
		tag = "div"
	}
	if !htmlNameRe.MatchString(tag) {
		tag = "div"
		id = "root"
		attrs = nil
	}
	if id == "" && tag == "div" {
		id = "root"
	}

	var b strings.Builder
	b.WriteString("<")
	b.WriteString(template.HTMLEscapeString(tag))

	if id != "" {
		b.WriteString(` id="`)
		b.WriteString(template.HTMLEscapeString(id))
		b.WriteString(`"`)
	}

	type attr struct {
		k string
		v string
	}
	ordered := make([]attr, 0, len(attrs))
	for k, v := range attrs {
		k = strings.TrimSpace(k)
		if k == "" || !htmlNameRe.MatchString(k) {
			continue
		}
		ordered = append(ordered, attr{k: k, v: v})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].k < ordered[j].k })
	for _, a := range ordered {
		b.WriteString(" ")
		b.WriteString(template.HTMLEscapeString(a.k))
		b.WriteString(`="`)
		b.WriteString(template.HTMLEscapeString(a.v))
		b.WriteString(`"`)
	}

	b.WriteString("></")
	b.WriteString(template.HTMLEscapeString(tag))
	b.WriteString(">")
	return b.String()
}

func buildCSSLinks(cssFiles []string) string {
	if len(cssFiles) == 0 {
		return ""
	}

	var links strings.Builder
	for _, cssFile := range cssFiles {
		links.WriteString(fmt.Sprintf(`<link rel="stylesheet" href="%s">`, template.HTMLEscapeString(cssFile)))
	}
	return links.String()
}

func buildJSScripts(jsFiles []string) string {
	if len(jsFiles) == 0 {
		return ""
	}

	var scripts strings.Builder
	for _, jsFile := range jsFiles {
		scripts.WriteString(fmt.Sprintf(`<script type="module" src="%s"></script>`, template.HTMLEscapeString(jsFile)))
	}
	return scripts.String()
}

func buildSafeContextScript(windowGlobal string, contextJSON []byte) (string, error) {
	const op serrors.Op = "applet.buildSafeContextScript"

	keyJSON, err := json.Marshal(windowGlobal)
	if err != nil {
		return "", serrors.E(op, serrors.Internal, "failed to marshal window global key", err)
	}
	safeKey := escapeJSONForScriptTag(keyJSON)
	safeValue := escapeJSONForScriptTag(contextJSON)
	return fmt.Sprintf(`<script>window[%s] = %s;</script>`, safeKey, safeValue), nil
}

func escapeJSONForScriptTag(jsonBytes []byte) string {
	jsonStr := string(jsonBytes)
	jsonStr = strings.ReplaceAll(jsonStr, "</script>", "<\\/script>")
	jsonStr = strings.ReplaceAll(jsonStr, "</SCRIPT>", "<\\/SCRIPT>")
	jsonStr = strings.ReplaceAll(jsonStr, "</Script>", "<\\/Script>")
	jsonStr = strings.ReplaceAll(jsonStr, "</sCrIpT>", "<\\/sCrIpT>")
	return jsonStr
}

type rpcRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID     string    `json:"id"`
	Result any       `json:"result,omitempty"`
	Error  *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (c *AppletController) handleRPC(w http.ResponseWriter, r *http.Request) {
	config := c.applet.Config()
	rpcCfg := config.RPC
	if rpcCfg == nil {
		http.NotFound(w, r)
		return
	}

	requireSameOrigin := true
	if rpcCfg.RequireSameOrigin != nil {
		requireSameOrigin = *rpcCfg.RequireSameOrigin
	}

	trustForwardedHost := false
	if rpcCfg.TrustForwardedHost != nil {
		trustForwardedHost = *rpcCfg.TrustForwardedHost
	}

	if requireSameOrigin {
		if err := enforceSameOrigin(r, trustForwardedHost); err != nil {
			writeRPC(w, http.StatusForbidden, rpcResponse{
				ID: "",
				Error: &rpcError{
					Code:    "forbidden",
					Message: "cross-origin request blocked",
					Details: map[string]string{"reason": err.Error()},
				},
			})
			return
		}
	}

	maxBytes := rpcCfg.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer func() { _ = r.Body.Close() }()

	var req rpcRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeRPC(w, http.StatusRequestEntityTooLarge, rpcResponse{
				ID: "",
				Error: &rpcError{
					Code:    "payload_too_large",
					Message: "request too large",
				},
			})
			return
		}
		writeRPC(w, http.StatusBadRequest, rpcResponse{
			ID: "",
			Error: &rpcError{
				Code:    "invalid_request",
				Message: "invalid json request",
			},
		})
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeRPC(w, http.StatusBadRequest, rpcResponse{
			ID: "",
			Error: &rpcError{
				Code:    "invalid_request",
				Message: "invalid json request",
			},
		})
		return
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		writeRPC(w, http.StatusBadRequest, rpcResponse{
			ID: req.ID,
			Error: &rpcError{
				Code:    "invalid_request",
				Message: "method is required",
			},
		})
		return
	}

	rpcMethod, ok := rpcCfg.Methods[method]
	if !ok {
		writeRPC(w, http.StatusOK, rpcResponse{
			ID: req.ID,
			Error: &rpcError{
				Code:    "method_not_found",
				Message: "method not found",
			},
		})
		return
	}

	if len(rpcMethod.RequirePermissions) > 0 {
		if err := requirePermissionStrings(r.Context(), rpcMethod.RequirePermissions); err != nil {
			writeRPC(w, http.StatusOK, rpcResponse{
				ID: req.ID,
				Error: &rpcError{
					Code:    "forbidden",
					Message: "permission denied",
				},
			})
			return
		}
	}

	result, err := rpcMethod.Handler(r.Context(), req.Params)
	if err != nil {
		code := mapSErrorCode(err)
		writeRPC(w, http.StatusOK, rpcResponse{
			ID: req.ID,
			Error: &rpcError{
				Code:    code,
				Message: "request failed",
			},
		})
		return
	}

	writeRPC(w, http.StatusOK, rpcResponse{
		ID:     req.ID,
		Result: result,
	})
}

func writeRPC(w http.ResponseWriter, status int, resp rpcResponse) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func mapSErrorCode(err error) string {
	var se *serrors.Error
	if !errors.As(err, &se) {
		return "error"
	}
	switch se.Kind {
	case serrors.KindValidation:
		return "validation"
	case serrors.Invalid:
		return "invalid"
	case serrors.NotFound:
		return "not_found"
	case serrors.PermissionDenied:
		return "forbidden"
	case serrors.Internal:
		return "internal"
	case serrors.Other:
		return "error"
	default:
		return "error"
	}
}

func requirePermissionStrings(ctx context.Context, required []string) error {
	const op serrors.Op = "applet.requirePermissionStrings"

	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.E(op, serrors.PermissionDenied, "no user", err)
	}
	if u == nil {
		return serrors.E(op, serrors.PermissionDenied, "no user")
	}
	have := make(map[string]struct{}, len(u.Permissions()))
	for _, p := range u.Permissions() {
		have[p.Name()] = struct{}{}
	}
	for _, need := range required {
		need = strings.TrimSpace(need)
		if need == "" {
			continue
		}
		if _, ok := have[need]; !ok {
			return serrors.E(op, serrors.PermissionDenied, "missing permission: "+need)
		}
	}
	return nil
}

func enforceSameOrigin(r *http.Request, trustForwardedHost bool) error {
	const op serrors.Op = "applet.enforceSameOrigin"

	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil {
		return serrors.E(op, serrors.Invalid, "invalid origin", err)
	}

	originHost, originPort := normalizeHostPort(strings.TrimSpace(u.Host))
	reqHost, reqPort := normalizeHostPort(requestHost(r, trustForwardedHost))

	if originHost == "" || reqHost == "" {
		return serrors.E(op, serrors.Invalid, "invalid host")
	}

	originPort = defaultPortIfEmpty(originPort, strings.ToLower(strings.TrimSpace(u.Scheme)))
	reqPort = defaultPortIfEmpty(reqPort, requestProto(r, trustForwardedHost))

	if originHost != reqHost || originPort != reqPort {
		return serrors.E(op, serrors.Invalid, "origin mismatch")
	}
	return nil
}

func requestHost(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		xfh := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
		if xfh != "" {
			parts := strings.Split(xfh, ",")
			if len(parts) > 0 {
				h := strings.TrimSpace(parts[0])
				if h != "" {
					return h
				}
			}
		}
	}
	return strings.TrimSpace(r.Host)
}

func requestProto(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		xfp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
		if xfp != "" {
			parts := strings.Split(xfp, ",")
			if len(parts) > 0 {
				p := strings.ToLower(strings.TrimSpace(parts[0]))
				if p != "" {
					return p
				}
			}
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func normalizeHostPort(hostport string) (string, string) {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return "", ""
	}
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		return strings.ToLower(h), p
	}
	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		return strings.ToLower(strings.Trim(hostport, "[]")), ""
	}
	return strings.ToLower(hostport), ""
}

func defaultPortIfEmpty(port string, proto string) string {
	if port != "" {
		return port
	}
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "https":
		return "443"
	default:
		return "80"
	}
}

func joinURLPath(base string, p string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return base + p
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if a == "" {
			return "/" + b
		}
		return a + "/" + b
	}
	return a + b
}
