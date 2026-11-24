package services_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/presentation/attachments"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// The adapter runs over the real upload service, repository and storage, from a context that carries
// neither a pool nor a tenant — what a poll has. The attachment lands in the private namespace: no
// anonymous URL names it, and the route that serves it checks the tenant, the permission and sends it as a
// download.
func TestEmailUploadsArePrivateAndServedToTheirTenantOnly(t *testing.T) {
	e := newEnv(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	up := services.NewEmailUploads(core, e.db, cfg.PrivatePath())

	saved, err := up.Save(context.Background(), e.tenant, email.UploadFile{Name: "cat.png", Mime: "image/png", Data: []byte("\x89PNG\r\n\x1a\n image bytes")})
	require.NoError(t, err, "an attachment is stored with no scope in the context")
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, "/automations/attachments/"+saved.ID, saved.URL, "the URL is the authorized route, never a static file")

	// A page of HTML in the mail must not run in the application's origin.
	html, err := up.Save(context.Background(), e.tenant, email.UploadFile{Name: "x.html", Mime: "text/html", Data: []byte("<script>alert(1)</script>")})
	require.NoError(t, err)

	var who struct {
		tenant uuid.UUID
		read   bool
	}
	who.tenant, who.read = e.tenant, true
	h := attachments.NewControllerWithAccess(core, func(context.Context) (uuid.UUID, bool) { return who.tenant, who.read })
	serve := func(id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/automations/attachments/"+id, nil), map[string]string{"id": id})
		r = r.WithContext(composables.WithPool(r.Context(), e.db))
		h.Download(w, r)
		return w
	}

	w := serve(saved.ID)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "\x89PNG\r\n\x1a\n image bytes", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))

	w = serve(html.ID)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"), "active content is never served as itself")

	who.tenant = e.newTenant()
	assert.Equal(t, http.StatusNotFound, serve(saved.ID).Code, "another tenant's attachment is not there")
	who.tenant, who.read = e.tenant, false
	assert.Equal(t, http.StatusForbidden, serve(saved.ID).Code, "reading automations is required")
	who.read = true

	// A public upload of the same tenant is not an email attachment: this route does not serve it.
	pub, err := core.Create(composables.WithPool(composables.WithTenantID(context.Background(), e.tenant), e.db), &upload.CreateDTO{File: bytesReader("public"), Name: "p.txt", Size: 6})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, serve(strconv.FormatUint(uint64(pub.ID()), 10)).Code)
}
