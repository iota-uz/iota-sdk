package uploads_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/uploads"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/attachments"
	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	mailconn "github.com/iota-uz/iota-sdk/pkg/automation/connectors/email"
	tgconn "github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The migrator reads ./migrations relative to the working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type uploadsEnv struct {
	scope *itf.Scope
	cfg   *uploadsconfig.Config
	core  *coreservices.UploadService
	up    tgconn.Uploads
}

func newUploadsEnv(t *testing.T, name string) uploadsEnv {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      name,
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	return uploadsEnv{scope: scope, cfg: cfg, core: core, up: uploads.NewTelegramUploads(core, scope.Pool, cfg.PrivatePath())}
}

func (e uploadsEnv) scoped() context.Context {
	return composables.WithPool(composables.WithTenantID(context.Background(), e.scope.Tenant.ID), e.scope.Pool)
}

func (e uploadsEnv) stored(t *testing.T, id string) upload.Upload {
	t.Helper()
	n, err := strconv.ParseUint(id, 10, 64)
	require.NoError(t, err)
	u, err := e.core.GetByID(e.scoped(), uint(n))
	require.NoError(t, err)
	return u
}

// The adapter runs over the real upload service, repository and storage, from a context that carries
// neither a pool nor a tenant — what a webhook request and a call worker have.
func TestTelegramUploadsOverTheSDKStorage(t *testing.T) {
	e := newUploadsEnv(t, "automations_uploads")
	ctx := context.Background()

	saved, err := e.up.Save(ctx, e.scope.Tenant.ID, tgconn.UploadFile{Name: "photo.jpg", Mime: "image/jpeg", Data: []byte("\xff\xd8\xff\xe0 jpeg bytes")})
	require.NoError(t, err, "an inbound photo is stored with no scope in the context")
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, "/automations/attachments/"+saved.ID, saved.URL, "the link is the authorized attachments route")

	f, err := e.up.Open(ctx, e.scope.Tenant.ID, saved.ID)
	require.NoError(t, err, "an outbound upload is read with no scope in the context")
	assert.Equal(t, "photo.jpg", f.Name)
	assert.Equal(t, []byte("\xff\xd8\xff\xe0 jpeg bytes"), f.Data)

	_, err = e.up.Open(ctx, uuid.New(), saved.ID)
	require.Error(t, err, "another tenant's upload is not found")
	_, err = e.up.Open(ctx, e.scope.Tenant.ID, "not-an-id")
	require.Error(t, err)

	again, err := e.up.Save(ctx, e.scope.Tenant.ID, tgconn.UploadFile{Name: "photo.jpg", Mime: "image/jpeg", Data: []byte("\xff\xd8\xff\xe0 jpeg bytes")})
	require.NoError(t, err)
	assert.Equal(t, saved.ID, again.ID, "a file received twice is one upload")
}

// A file of a chat is private, like an email attachment (#1439): it is kept in the private namespace of Telegram
// files, no anonymous URL (the public file handler, the core's download route) serves it, and the attachments route
// serves it only to a signed-in user of its tenant who may read automations — as a sandboxed download, so an HTML
// or SVG document never runs as a page on the application's origin.
func TestInboundFilesArePrivateAndServedToTheirTenantOnly(t *testing.T) {
	e := newUploadsEnv(t, "automations_uploads_private")
	ctx := context.Background()
	tenant := e.scope.Tenant.ID
	html := []byte("<script>alert(1)</script>")
	saved, err := e.up.Save(ctx, tenant, tgconn.UploadFile{Name: "evil.html", Mime: "text/html", Data: html})
	require.NoError(t, err)
	stored := e.stored(t, saved.ID)
	wantDir := filepath.Join(e.cfg.PrivatePath(), services.TelegramAttachmentsNamespace, tenant.String())
	assert.Equal(t, wantDir, filepath.Dir(stored.Path()), "the file is in the tenant's directory of the private namespace")
	assert.Equal(t, "evil.html", stored.Name(), "the name is the chat's")

	// No anonymous route of the core serves it: neither the public file handler nor the signed-in download route.
	core := mux.NewRouter()
	controllers.NewUploadController(e.core, e.cfg).Register(core)
	root := strings.TrimSuffix(e.cfg.Path, "/") + "/"
	rec := httptest.NewRecorder()
	core.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path.Join("/", e.cfg.Path, strings.TrimPrefix(stored.Path(), root)), nil))
	assert.NotEqual(t, http.StatusOK, rec.Code, "the public file handler does not serve the private namespace")
	email, err := internet.NewEmail("viewer@example.com")
	require.NoError(t, err)
	viewer := user.New("View", "Er", email, user.UILanguageEN)
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/uploads/"+saved.ID, nil)
	core.ServeHTTP(rec, req.WithContext(composables.WithPool(composables.WithTenantID(composables.WithUser(req.Context(), viewer), tenant), e.scope.Pool)))
	assert.Equal(t, http.StatusNotFound, rec.Code, "the core's download route does not serve it either")

	// Without a session the attachments route serves nothing.
	routes := mux.NewRouter()
	attachments.NewController(e.core).Register(routes)
	rec = httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, saved.URL, nil))
	assert.Equal(t, http.StatusForbidden, rec.Code, "an anonymous request is refused")
	assert.NotContains(t, rec.Body.String(), "alert(1)")

	var who struct {
		tenant uuid.UUID
		read   bool
	}
	who.tenant, who.read = tenant, true
	h := attachments.NewControllerWithAccess(e.core, func(context.Context) (uuid.UUID, bool) { return who.tenant, who.read })
	serve := func(id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, services.AttachmentPath(id), nil), map[string]string{"id": id})
		r = r.WithContext(composables.WithPool(r.Context(), e.scope.Pool))
		h.Download(w, r)
		return w
	}
	w := serve(saved.ID)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, html, w.Body.Bytes())
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"), "active content is never served as itself")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))

	who.tenant = uuid.New()
	assert.Equal(t, http.StatusNotFound, serve(saved.ID).Code, "another tenant's file is not there")
	who.tenant, who.read = tenant, false
	assert.Equal(t, http.StatusForbidden, serve(saved.ID).Code, "reading automations is required")
}

// Content the tenant already has is that upload when the attachments route serves it (a public upload, an email
// attachment); content of another feature's private file is neither stored under it nor sent.
func TestInboundFilesAndOtherUploadsOfTheSameContent(t *testing.T) {
	e := newUploadsEnv(t, "automations_uploads_dedup")
	ctx := context.Background()
	tenant := e.scope.Tenant.ID

	public, err := e.core.Create(e.scoped(), &upload.CreateDTO{File: bytes.NewReader([]byte("public bytes")), Name: "public.pdf", Size: 12})
	require.NoError(t, err)
	saved, err := e.up.Save(ctx, tenant, tgconn.UploadFile{Name: "doc.pdf", Data: []byte("public bytes")})
	require.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(public.ID()), 10), saved.ID, "the tenant's public upload of the content is the file")
	assert.Equal(t, public.Path(), e.stored(t, saved.ID).Path(), "and it stays where it is")

	mail := services.NewEmailUploads(e.core, e.scope.Pool, e.cfg.PrivatePath())
	att, err := mail.Save(ctx, tenant, email2("mail bytes"))
	require.NoError(t, err)
	saved, err = e.up.Save(ctx, tenant, tgconn.UploadFile{Name: "same.pdf", Data: []byte("mail bytes")})
	require.NoError(t, err)
	assert.Equal(t, att.ID, saved.ID, "an email attachment of the content is the file")
	opened, err := e.up.Open(ctx, tenant, att.ID)
	require.NoError(t, err, "a graph may send a file an email brought")
	assert.Equal(t, []byte("mail bytes"), opened.Data)

	elsewhere := filepath.Join(e.cfg.PrivatePath(), "other-feature")
	require.NoError(t, os.MkdirAll(elsewhere, 0o700))
	secret, err := e.core.CreatePrivate(e.scoped(), &upload.CreateDTO{File: bytes.NewReader([]byte("secret")), Name: "s.txt", Size: 6, Slug: "otherfeaturesecret"}, elsewhere)
	require.NoError(t, err)
	_, err = e.up.Save(ctx, tenant, tgconn.UploadFile{Name: "s.txt", Data: []byte("secret")})
	require.Error(t, err, "the content of another feature's private file is not stored under it (files[].error)")
	_, err = e.up.Open(ctx, tenant, strconv.FormatUint(uint64(secret.ID()), 10))
	require.Error(t, err, "and a graph cannot send another feature's private file")
}

// A file stored before the files were private kept its name under a suffix: Open gives the chat's name back.
func TestALegacyChatFileIsOpenedUnderItsName(t *testing.T) {
	e := newUploadsEnv(t, "automations_uploads_legacy")
	legacy, err := e.core.Create(e.scoped(), &upload.CreateDTO{File: bytes.NewReader([]byte("old file")), Name: "report.pdf.tg.txt", Size: 8})
	require.NoError(t, err)
	f, err := e.up.Open(context.Background(), e.scope.Tenant.ID, strconv.FormatUint(uint64(legacy.ID()), 10))
	require.NoError(t, err)
	assert.Equal(t, "report.pdf", f.Name)
}

func email2(body string) mailconn.UploadFile {
	return mailconn.UploadFile{Name: "mail.pdf", Data: []byte(body)}
}
