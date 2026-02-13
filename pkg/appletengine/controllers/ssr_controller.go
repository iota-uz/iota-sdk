package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	"github.com/sirupsen/logrus"
)

type SSRController struct {
	appletID  string
	basePath  string
	hosts     []string
	runtime   *appletengineruntime.Manager
	host      applets.HostServices
	logger    *logrus.Logger
	entryPath string
}

func NewSSRController(applet applets.Applet, runtime *appletengineruntime.Manager, host applets.HostServices, logger *logrus.Logger, entryPath string) *SSRController {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	config := applet.Config()
	return &SSRController{
		appletID:  applet.Name(),
		basePath:  applet.BasePath(),
		hosts:     append([]string(nil), config.Hosts...),
		runtime:   runtime,
		host:      host,
		logger:    logger,
		entryPath: entryPath,
	}
}

func (c *SSRController) Key() string {
	return "applet_ssr_" + c.appletID
}

func (c *SSRController) Register(router *mux.Router) {
	pathRouter := router.PathPrefix(c.basePath).Subrouter()
	pathRouter.PathPrefix("/").HandlerFunc(c.proxyToRuntime)
	pathRouter.HandleFunc("", c.proxyToRuntime)
	pathRouter.HandleFunc("/", c.proxyToRuntime)

	for _, host := range c.hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		hostRouter := router.Host(host).Subrouter()
		hostRouter.PathPrefix("/").HandlerFunc(c.proxyToRuntime)
		hostRouter.HandleFunc("", c.proxyToRuntime)
		hostRouter.HandleFunc("/", c.proxyToRuntime)
	}
}

func (c *SSRController) proxyToRuntime(w http.ResponseWriter, r *http.Request) {
	if c.runtime == nil {
		http.Error(w, "applet runtime unavailable", http.StatusServiceUnavailable)
		return
	}
	process, err := c.runtime.EnsureStarted(r.Context(), c.appletID, c.entryPath)
	if err != nil {
		http.Error(w, "failed to start applet runtime", http.StatusServiceUnavailable)
		return
	}
	if process == nil {
		http.Error(w, "applet runtime unavailable", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	targetURL := fmt.Sprintf("http://unix%s", r.URL.RequestURI())
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to build proxy request", http.StatusInternalServerError)
		return
	}
	proxyReq.Header = cloneHeaders(r.Header)
	c.injectContextHeaders(proxyReq)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := &net.Dialer{}
				return dialer.DialContext(ctx, "unix", process.AppletSocket)
			},
		},
	}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "failed to proxy request to applet runtime", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (c *SSRController) injectContextHeaders(req *http.Request) {
	if c.host == nil {
		return
	}
	ctx := req.Context()
	tenantID, err := c.host.ExtractTenantID(ctx)
	if err == nil {
		req.Header.Set("X-Iota-Tenant-Id", tenantID.String())
	}
	user, err := c.host.ExtractUser(ctx)
	if err == nil && user != nil {
		req.Header.Set("X-Iota-User-Id", fmt.Sprintf("%d", user.ID()))
		req.Header.Set("X-Iota-Permissions", strings.Join(user.PermissionNames(), ","))
	}
	if reqID := strings.TrimSpace(req.Header.Get("X-Iota-Request-Id")); reqID == "" {
		req.Header.Set("X-Iota-Request-Id", "ssr-proxy")
	}
}

func cloneHeaders(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, values := range in {
		copied := make([]string, len(values))
		copy(copied, values)
		out[k] = copied
	}
	return out
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
