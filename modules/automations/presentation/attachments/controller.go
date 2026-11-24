// Package attachments serves the attachments of incoming email to the users of the tenant they belong to.
package attachments

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// ControllerID identifies the controller.
const ControllerID = "automations.attachments"

// Uploads is the part of the SDK's upload service the route reads.
type Uploads interface {
	GetByID(ctx context.Context, id uint) (upload.Upload, error)
	OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error)
}

// Access tells who is asking: the tenant of the session and whether the user may read automations.
type Access func(ctx context.Context) (tenant uuid.UUID, mayRead bool)

// Controller serves GET /automations/attachments/{id}.
type Controller struct {
	uploads Uploads
	access  Access
}

var _ application.Controller = (*Controller)(nil)

// NewController is the route over the upload service, for the user of the session.
func NewController(u Uploads) *Controller { return NewControllerWithAccess(u, sessionAccess) }

// NewControllerWithAccess is NewController with its own answer to who is asking.
func NewControllerWithAccess(u Uploads, access Access) *Controller {
	return &Controller{uploads: u, access: access}
}

func sessionAccess(ctx context.Context) (uuid.UUID, bool) {
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return uuid.Nil, false
	}
	return sub.TenantID, sub.Has(permissions.AutomationsRead.Name())
}

func (c *Controller) Descriptor() application.ControllerDescriptor {
	return application.Descriptor(ControllerID, 0)
}

func (c *Controller) Register(r *mux.Router) {
	s := r.PathPrefix("/automations/attachments").Methods(http.MethodGet).Subrouter()
	s.Use(middleware.Authorize(), middleware.ProvideUser())
	s.HandleFunc("/{id:[0-9]+}", c.Download)
}

// active content a browser would run when it opens the file: never sent with its own type.
var activeTypes = map[string]bool{
	"text/html": true, "application/xhtml+xml": true, "image/svg+xml": true, "text/xml": true, "application/xml": true,
	"text/javascript": true, "application/javascript": true,
}

// Download serves the attachment {id} of the route.
func (c *Controller) Download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// A file is for the users who may read automations of the tenant the file belongs to; a file of
	// another tenant, or one that is not an email attachment, looks like one that is not there.
	tenantID, mayRead := c.access(ctx)
	if !mayRead {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	entity, err := c.uploads.GetByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, persistence.ErrUploadNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if entity.TenantID() != tenantID || !inNamespace(entity.Path()) {
		http.NotFound(w, r)
		return
	}
	body, info, err := c.uploads.OpenReader(ctx, entity.Path())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() { _ = body.Close() }()

	contentType := ""
	if m := entity.Mimetype(); m != nil {
		contentType = m.String()
	}
	if contentType == "" {
		contentType = info.ContentType
	}
	base, _, _ := mime.ParseMediaType(contentType)
	if base == "" || activeTypes[strings.ToLower(base)] {
		contentType = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	if d := mime.FormatMediaType("attachment", map[string]string{"filename": entity.Name()}); d != "" {
		h.Set("Content-Disposition", d)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	_, _ = io.Copy(w, body)
}

// inNamespace reports whether a stored path is in the private namespace of email attachments.
func inNamespace(stored string) bool {
	p := path.Clean("/" + filepath.ToSlash(upload.LayoutPath(stored)))
	return strings.Contains(p+"/", "/"+uploadsconfig.PrivateDirectory+"/"+services.EmailAttachmentsNamespace+"/")
}
