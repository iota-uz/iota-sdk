package endpoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type errorBody struct {
	Error struct {
		Kind    string          `json:"kind,omitempty"`
		Code    string          `json:"code"`
		Message string          `json:"message,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
		Issues  []pool.Issue    `json:"issues,omitempty"`
	} `json:"error"`
	Invocation string `json:"invocation,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, message
	writeJSON(w, status, b)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "response encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

// ServeHTTP routes /_ext/endpoints/<namespace>/<path> to the active endpoint
// with that method and path.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, PathPrefix)
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "unknown endpoint")
		return
	}
	all, err := h.cfg.Source.ActiveEndpoints(r.Context())
	if err != nil {
		h.cfg.Logger.WithError(err).Error("extruntime endpoint: read active endpoints")
		writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "endpoints are not available")
		return
	}
	candidates := match(all, rest)
	if len(candidates) == 0 {
		writeError(w, http.StatusNotFound, CodeNotFound, "unknown endpoint")
		return
	}
	var ep *Endpoint
	allowed := make([]string, 0, len(candidates))
	for i := range candidates {
		allowed = append(allowed, candidates[i].Method)
		if strings.EqualFold(candidates[i].Method, r.Method) {
			ep = &candidates[i]
		}
	}
	if ep == nil {
		sort.Strings(allowed)
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllow, "method not allowed")
		return
	}

	subject, ok := h.subject(w, r, ep)
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.cfg.MaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge, "request body is too large")
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, "request body could not be read")
		return
	}
	input, err := json.Marshal(buildRequest(r, ep, body))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "request cannot be represented")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.Timeout)
	defer cancel()
	res, err := h.cfg.Runner.Run(ctx, host.RunRequest{
		Export: host.Export{Kind: pool.KindEndpoint, Resource: ep.ID, Binding: host.Binding{
			Contract: ep.ID, Package: ep.Package, Version: ep.Version, Export: ep.Export, Artifact: ep.Artifact, Secrets: ep.Secrets,
		}},
		Input:   input,
		Subject: subject,
		Trigger: host.Trigger{Kind: host.TriggerEndpoint, Ref: ep.ID},
	})
	if err != nil {
		h.fail(w, ep, res, err)
		return
	}
	h.respond(w, ep, res)
}

// subject decides who the endpoint runs as. It writes the response itself when
// there is nobody to run as.
//
// A non-public endpoint runs as the signed-in user, and a state-changing
// request must not come from another site (the session cookie is sent with
// it). A public endpoint never looks at the session: it runs as the trigger
// owner of the tenant for everybody. Otherwise a page on any site could post
// to it from a visitor's browser and have it run with that visitor's rights.
func (h *Handler) subject(w http.ResponseWriter, r *http.Request, ep *Endpoint) (spi.Subject, bool) {
	if !ep.Public {
		if err := h.cfg.Origins.Check(r); err != nil {
			h.cfg.Logger.WithError(err).WithField("endpoint", ep.ID).Warn("extruntime endpoint: cross-site request refused")
			writeError(w, http.StatusForbidden, CodeCrossSite, "cross-site request refused")
			return spi.Subject{}, false
		}
		if sub, ok := h.cfg.SessionSubject(r.Context()); ok {
			sub.Credential = "session"
			return sub, true
		}
		writeError(w, http.StatusUnauthorized, CodeUnauthenticated, "sign in required")
		return spi.Subject{}, false
	}
	if h.cfg.Owners == nil || h.cfg.Subjects == nil {
		writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "public endpoints are not configured: trigger owners are unavailable")
		return spi.Subject{}, false
	}
	tenant, ok := h.cfg.PublicTenant(r)
	if !ok || tenant == uuid.Nil {
		writeError(w, http.StatusServiceUnavailable, CodeNoTenant, "cannot determine the tenant of this request; a public endpoint runs as the trigger owner of a tenant")
		return spi.Subject{}, false
	}
	owner, found, err := h.cfg.Owners.Owner(r.Context(), ep.ID, tenant)
	if err != nil {
		h.cfg.Logger.WithError(err).WithField("endpoint", ep.ID).Error("extruntime endpoint: read trigger owner")
		writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "trigger owner could not be read")
		return spi.Subject{}, false
	}
	if !found {
		writeError(w, http.StatusServiceUnavailable, CodeOwnerMissing, "public endpoint "+ep.ID+" has no trigger owner assigned for this tenant")
		return spi.Subject{}, false
	}
	sub, err := h.cfg.Subjects.LoadSubject(r.Context(), tenant, owner, spi.SubjectOwner)
	if err != nil {
		if errors.Is(err, host.ErrSubjectBlocked) {
			writeError(w, http.StatusServiceUnavailable, CodeOwnerBlocked, "the trigger owner of "+ep.ID+" cannot act: "+err.Error())
		} else {
			h.cfg.Logger.WithError(err).WithField("endpoint", ep.ID).Error("extruntime endpoint: load trigger owner")
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "trigger owner could not be loaded")
		}
		return spi.Subject{}, false
	}
	sub.Kind = spi.SubjectOwner
	sub.Credential = "endpoint"
	return sub, true
}

// request is the invoke input of an endpoint: the `EndpointRequest` the
// handler sees (rawBody is base64 on the wire).
type request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    json.RawMessage   `json:"body,omitempty"`
	RawBody string            `json:"rawBody"`
}

// withheldHeaders never reach the package: credentials of the caller.
var withheldHeaders = map[string]bool{"cookie": true, "authorization": true, "proxy-authorization": true}

func buildRequest(r *http.Request, ep *Endpoint, body []byte) request {
	req := request{
		Method:  r.Method,
		Path:    ep.Path,
		Headers: map[string]string{},
		Query:   map[string]string{},
		RawBody: base64.StdEncoding.EncodeToString(body),
	}
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		if withheldHeaders[lower] {
			continue
		}
		req.Headers[lower] = strings.Join(values, ", ")
	}
	for name, values := range r.URL.Query() {
		req.Query[name] = values[0]
	}
	req.Body = parseBody(r.Header.Get("Content-Type"), body)
	return req
}

// parseBody parses JSON and url-encoded form bodies. Anything else (and a body
// that does not parse) has no parsed form; the handler still has rawBody.
func parseBody(contentType string, body []byte) json.RawMessage {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil
	}
	switch {
	case media == "application/json" || strings.HasSuffix(media, "+json"):
		if !json.Valid(body) {
			return nil
		}
		return json.RawMessage(bytes.TrimSpace(body))
	case media == "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return nil
		}
		form := make(map[string]any, len(values))
		for name, vs := range values {
			if len(vs) == 1 {
				form[name] = vs[0]
			} else {
				form[name] = vs
			}
		}
		b, err := json.Marshal(form)
		if err != nil {
			return nil
		}
		return b
	}
	return nil
}

// fail maps a failed invocation to an HTTP error without leaking internals.
func (h *Handler) fail(w http.ResponseWriter, ep *Endpoint, res host.Result, err error) {
	var b errorBody
	b.Invocation = res.InvocationID
	var we *pool.WireError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		b.Error.Code, b.Error.Message = CodeTimeout, "the endpoint did not answer in time"
		writeJSON(w, http.StatusGatewayTimeout, b)
	case errors.Is(err, pool.ErrQueueFull), errors.Is(err, pool.ErrPoolClosed):
		b.Error.Code, b.Error.Message = CodeUnavailable, "the runtime is busy"
		writeJSON(w, http.StatusServiceUnavailable, b)
	case errors.As(err, &we) && (we.Kind == pool.ErrKindDenied || we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation):
		b.Error.Kind, b.Error.Code, b.Error.Message, b.Error.Params, b.Error.Issues = string(we.Kind), we.Code, we.Message, we.Params, we.Issues
		status := http.StatusUnprocessableEntity
		if we.Kind == pool.ErrKindDenied {
			status = http.StatusForbidden
		}
		writeJSON(w, status, b)
	default:
		h.cfg.Logger.WithError(err).WithField("endpoint", ep.ID).WithField("invocation", res.InvocationID).Error("extruntime endpoint: invocation failed")
		b.Error.Kind, b.Error.Code, b.Error.Message = string(pool.ErrKindInternal), CodeInternal, "internal error"
		writeJSON(w, http.StatusInternalServerError, b)
	}
}

// Response headers a handler may set. Everything else is dropped: the set is
// an allowlist, so a header the platform did not think of (Set-Cookie, CORS,
// CSP, reverse-proxy controls such as X-Accel-Redirect) never reaches the
// application's own origin. A handler may also set its own X- headers, except
// the ones that steer a proxy or that the platform sets itself.
var allowedResponseHeaders = map[string]bool{
	"content-type": true, "cache-control": true, "etag": true, "last-modified": true, "expires": true,
	"content-disposition": true, "content-language": true, "vary": true, "location": true,
}

// proxyControlPrefixes are X- headers that tell a front proxy what to do.
var proxyControlPrefixes = []string{"x-accel-", "x-sendfile", "x-lighttpd-", "x-forwarded-", "x-real-", "x-original-", "x-rewrite-", "x-http-method"}

func responseHeaderAllowed(name string) bool {
	lower := strings.ToLower(name)
	if allowedResponseHeaders[lower] {
		return true
	}
	if !strings.HasPrefix(lower, "x-") || lower == "x-content-type-options" || lower == "x-frame-options" || lower == "x-xss-protection" {
		return false
	}
	for _, prefix := range proxyControlPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

// cleanHeaderValue reports whether a header value is plain: no control
// characters, so it cannot split the response.
func cleanHeaderValue(v string) bool {
	for _, r := range v {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return false
		}
	}
	return true
}

// safeLocation reports whether a redirect target stays on this origin: an
// absolute path ("/x"), not a protocol-relative one ("//host") or one a
// browser reads as such ("/\host").
func safeLocation(v string) bool {
	return strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") && !strings.HasPrefix(v, "/\\") && !strings.ContainsAny(v, "\\")
}

// activeContent reports a media type a browser runs scripts from.
func activeContent(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		media = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	}
	switch media {
	case "text/html", "application/xhtml+xml", "image/svg+xml", "text/xml", "application/xml", "text/xsl", "text/xslt", "application/xslt+xml":
		return true
	}
	return strings.HasSuffix(media, "+xml")
}

type response struct {
	Status  *int              `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

// respond writes the handler's {status, headers?, body?} as the response.
func (h *Handler) respond(w http.ResponseWriter, ep *Endpoint, res host.Result) {
	var out response
	if err := json.Unmarshal(res.Output, &out); err != nil || out.Status == nil || *out.Status < 200 || *out.Status > 599 {
		h.cfg.Logger.WithField("endpoint", ep.ID).WithField("invocation", res.InvocationID).Error("extruntime endpoint: handler returned an invalid response")
		var b errorBody
		b.Invocation = res.InvocationID
		b.Error.Kind, b.Error.Code, b.Error.Message = string(pool.ErrKindInternal), CodeBadResponse, "the endpoint returned an invalid response; expected {status: 200..599, headers?, body?}"
		writeJSON(w, http.StatusBadGateway, b)
		return
	}
	status := *out.Status
	for name, value := range out.Headers {
		if !responseHeaderAllowed(name) || !cleanHeaderValue(name) || !cleanHeaderValue(value) {
			continue
		}
		if strings.EqualFold(name, "location") && ((status < 300 || status > 399) && status != http.StatusCreated || !safeLocation(value)) {
			continue
		}
		w.Header().Set(name, value)
	}
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	hasBody := len(out.Body) > 0 && string(out.Body) != "null" && status != http.StatusNoContent && status != http.StatusNotModified
	if hasBody && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	if activeContent(w.Header().Get("Content-Type")) {
		// A document the handler serves runs in a sandbox of its own: no
		// scripts, no forms, and an opaque origin that cannot reach the
		// application's cookies or storage.
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	w.WriteHeader(status)
	if hasBody {
		_, _ = w.Write(out.Body)
	}
}
