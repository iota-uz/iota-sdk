package uploads_test

import (
	"bytes"
	"context"
	"mime"
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
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
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

// The adapter runs over the real upload service, repository and storage, from a context that carries
// neither a pool nor a tenant — what a webhook request and a call worker have.
func TestTelegramUploadsOverTheSDKStorage(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_uploads",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	up := uploads.NewTelegramUploads(core, scope.Pool)
	ctx := context.Background()

	saved, err := up.Save(ctx, scope.Tenant.ID, tgconn.UploadFile{Name: "photo.jpg", Mime: "image/jpeg", Data: []byte("\xff\xd8\xff\xe0 jpeg bytes")})
	require.NoError(t, err, "an inbound photo is stored with no scope in the context")
	assert.NotEmpty(t, saved.ID)
	assert.NotEmpty(t, saved.URL)

	f, err := up.Open(ctx, scope.Tenant.ID, saved.ID)
	require.NoError(t, err, "an outbound upload is read with no scope in the context")
	assert.Equal(t, "photo.jpg", f.Name)
	assert.Equal(t, []byte("\xff\xd8\xff\xe0 jpeg bytes"), f.Data)

	_, err = up.Open(ctx, uuid.New(), saved.ID)
	require.Error(t, err, "another tenant's upload is not found")
	_, err = up.Open(ctx, scope.Tenant.ID, "not-an-id")
	require.Error(t, err)
}

// A document of a chat is stored where the public file handler can only serve it as plain text, and its link is the
// signed-in download route: an HTML or SVG document is never served as a page on the application's origin.
func TestInboundDocumentsAreNeverServedAsPages(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_uploads_html",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	up := uploads.NewTelegramUploads(core, scope.Pool)
	ctx := context.Background()
	for _, name := range []string{"evil.html", "evil.svg", "evil.HTM", "no-extension"} {
		saved, err := up.Save(ctx, scope.Tenant.ID, tgconn.UploadFile{Name: name, Mime: "text/html", Data: []byte("<script>alert(1)</script> " + name)})
		require.NoError(t, err)
		assert.Equal(t, "/uploads/"+saved.ID, saved.URL, "%s: the link is the signed-in route, not the public file", name)
		id, err := strconv.ParseUint(saved.ID, 10, 64)
		require.NoError(t, err)
		stored, err := core.GetByID(composables.WithPool(composables.WithTenantID(ctx, scope.Tenant.ID), scope.Pool), uint(id))
		require.NoError(t, err)
		assert.Equal(t, ".txt", filepath.Ext(stored.Path()), "%s is stored as %s, which the public handler serves as plain text", name, stored.Path())
		assert.Equal(t, "text/plain; charset=utf-8", mime.TypeByExtension(filepath.Ext(stored.Path())), "the type the file server gives it")
		f, err := up.Open(ctx, scope.Tenant.ID, saved.ID)
		require.NoError(t, err)
		assert.Equal(t, name, f.Name, "the name Telegram gets back is the original")
	}
}

// Content that matches a file stored for another purpose neither publishes it nor renames it: a private upload stays
// private (the public route keeps answering 404 for it), a public one keeps its record and path, and a document
// that matches nothing is served by the public route as plain text. Checked through the real routes.
func TestInboundDocumentsKeepWhatTheContentMatches(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "automations_uploads_dedup",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	cfg := &uploadsconfig.Config{Path: t.TempDir()}
	storage, err := persistence.NewFSStorage(cfg)
	require.NoError(t, err)
	core := coreservices.NewConfiguredUploadService(persistence.NewUploadRepository(), storage, eventbus.NewEventPublisher(logrus.New()), cfg)
	up := uploads.NewTelegramUploads(core, scope.Pool)
	router := mux.NewRouter()
	controllers.NewUploadController(core, cfg).Register(router)
	scoped := composables.WithPool(composables.WithTenantID(context.Background(), scope.Tenant.ID), scope.Pool)
	root := strings.TrimSuffix(cfg.Path, "/") + "/"
	anonymous := func(storedPath string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path.Join("/", cfg.Path, strings.TrimPrefix(storedPath, root)), nil))
		return rec
	}
	html := func(name string) []byte { return []byte("<script>alert(1)</script> " + name) }

	// A private upload (outside the public root) with the content of a chat's document.
	privateDir := t.TempDir()
	private, err := core.CreatePrivate(scoped, &upload.CreateDTO{File: bytes.NewReader(html("private")), Name: "private.html", Size: len(html("private"))}, privateDir)
	require.NoError(t, err)
	publicRoute := func() int { // the public route for the file's name in the public root
		return anonymous(filepath.Join(cfg.Path, filepath.Base(private.Path()))).Code
	}
	require.Equal(t, http.StatusNotFound, publicRoute(), "the private upload is not public to begin with")
	saved, err := up.Save(context.Background(), scope.Tenant.ID, tgconn.UploadFile{Name: "evil.html", Mime: "text/html", Data: html("private")})
	require.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(private.ID()), 10), saved.ID, "the same content is the same upload")
	after, err := core.GetByID(scoped, private.ID())
	require.NoError(t, err)
	assert.Equal(t, private.Path(), after.Path(), "its path is as it was: not moved into the public root")
	assert.Equal(t, "private.html", after.Name(), "and its name")
	assert.Equal(t, http.StatusNotFound, publicRoute(), "the public route still does not serve it")

	// A public upload with the content of another document: its record is left as it is.
	public, err := core.Create(scoped, &upload.CreateDTO{File: bytes.NewReader(html("public")), Name: "public.html", Size: len(html("public"))})
	require.NoError(t, err)
	saved, err = up.Save(context.Background(), scope.Tenant.ID, tgconn.UploadFile{Name: "other.svg", Mime: "image/svg+xml", Data: html("public")})
	require.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(public.ID()), 10), saved.ID)
	after, err = core.GetByID(scoped, public.ID())
	require.NoError(t, err)
	assert.Equal(t, public.Path(), after.Path())
	assert.Equal(t, "public.html", after.Name())

	// A document that matches nothing is stored where the public route gives plain text.
	fresh, err := up.Save(context.Background(), scope.Tenant.ID, tgconn.UploadFile{Name: "new.html", Mime: "text/html", Data: html("new")})
	require.NoError(t, err)
	id, err := strconv.ParseUint(fresh.ID, 10, 64)
	require.NoError(t, err)
	stored, err := core.GetByID(scoped, uint(id))
	require.NoError(t, err)
	rec := anonymous(stored.Path())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"), "served as %q", rec.Header().Get("Content-Type"))
}
