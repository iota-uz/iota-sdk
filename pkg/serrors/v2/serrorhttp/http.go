// Package serrorhttp projects safe errors into route-owned HTTP responses.
package serrorhttp

import (
	"context"
	"encoding/json"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorlog"
	"net/http"
)

func Status(err error) int {
	switch serrors.CodeOf(err) {
	case serrors.Invalid:
		return http.StatusBadRequest
	case serrors.NotFound:
		return http.StatusNotFound
	case serrors.AlreadyExists, serrors.Conflict, serrors.FailedPrecondition:
		return http.StatusConflict
	case serrors.PermissionDenied:
		return http.StatusForbidden
	case serrors.Unauthenticated:
		return http.StatusUnauthorized
	case serrors.RateLimited:
		return http.StatusTooManyRequests
	case serrors.Unavailable:
		return http.StatusServiceUnavailable
	case serrors.Timeout:
		return http.StatusGatewayTimeout
	case serrors.Canceled:
		return http.StatusRequestTimeout
	case serrors.Unimplemented:
		return http.StatusNotImplemented
	case serrors.Internal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

type Problem struct {
	Type   string                `json:"type"`
	Title  string                `json:"title"`
	Status int                   `json:"status"`
	Detail string                `json:"detail"`
	Code   string                `json:"code"`
	Reason serrors.Reason        `json:"reason,omitempty"`
	Fields []serrors.PublicField `json:"fields,omitempty"`
}

// Profile explicitly owns the existing route's wire contract and headers.
type Profile struct {
	Status      func(error) int
	Headers     http.Header
	ContentType string
	Encode      func(http.ResponseWriter, serrors.Projection, int) error
}

func Write(w http.ResponseWriter, err error, l *i18n.Localizer, profile Profile) error {
	status := Status(err)
	if profile.Status != nil {
		status = profile.Status(err)
	}
	for key, values := range profile.Headers {
		w.Header()[key] = append([]string(nil), values...)
	}
	contentType := profile.ContentType
	if contentType == "" {
		contentType = "application/problem+json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	p := serrors.Public(err, l)
	if profile.Encode != nil {
		return profile.Encode(w, p, status)
	}
	return json.NewEncoder(w).Encode(Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: p.Message, Code: p.Code, Reason: p.Reason, Fields: p.Fields})
}

type Form struct {
	Render func(http.ResponseWriter, *http.Request, serrors.Projection) error
	Target string
	Swap   string
}

// WriteForm requires a route-owned renderer retaining input and field names.
func WriteForm(w http.ResponseWriter, r *http.Request, err error, l *i18n.Localizer, form Form) error {
	if form.Render == nil {
		return serrors.NewInternal("missing form renderer")
	}
	status := Status(err)
	if htmx.IsHxRequest(r) && serrors.HasCode(err, serrors.Invalid) {
		status = http.StatusOK
	}
	if form.Target != "" {
		htmx.Retarget(w, form.Target)
	}
	if form.Swap != "" {
		htmx.Reswap(w, form.Swap)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return form.Render(w, r, serrors.Public(err, l))
}

// WriteText retains plain-text responses and classifies semantic errors.
func WriteText(w http.ResponseWriter, err error, status int, l *i18n.Localizer) {
	if serrors.CodeOf(err) != serrors.Internal {
		status = Status(err)
	}
	http.Error(w, serrors.Public(err, l).Message, status)
}

// WriteTextContext logs and writes a safe plain-text error at the HTTP boundary.
func WriteTextContext(ctx context.Context, w http.ResponseWriter, err error, status int, l *i18n.Localizer) {
	serrorlog.Log(ctx, err, "HTTP request failed")
	WriteText(w, err, status, l)
}
