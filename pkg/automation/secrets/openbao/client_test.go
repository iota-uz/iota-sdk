package openbao_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets/openbao"
)

// fakeBao is enough of OpenBao for the client's token handling: an AppRole login that issues tokens of a
// TTL, renew-self, and a KV v2 read that checks the token.
type fakeBao struct {
	mu      sync.Mutex
	now     time.Time
	ttl     time.Duration
	tokens  map[string]time.Time // token → expiry
	issued  int
	renewed int
	reads   int
	// status answers every KV request with it when set (an outage, a sealed server).
	status int
	body   string
}

func newFakeBao(t *testing.T) (*fakeBao, *httptest.Server) {
	t.Helper()
	f := &fakeBao{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ttl: 30 * time.Second, tokens: map[string]time.Time{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeBao) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeBao) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func (f *fakeBao) revokeAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = map[string]time.Time{}
}

func (f *fakeBao) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(code int, v any) {
		w.WriteHeader(code)
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		_, _ = w.Write(raw)
	}
	valid := func() bool {
		exp, ok := f.tokens[r.Header.Get("X-Vault-Token")]
		return ok && f.now.Before(exp)
	}
	switch {
	case r.URL.Path == "/v1/auth/approle/login":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["role_id"] != "role" || body["secret_id"] != "the-secret-id" {
			write(http.StatusBadRequest, map[string]any{"errors": []string{"invalid secret id " + body["secret_id"]}})
			return
		}
		f.issued++
		tok := fmt.Sprintf("tok-%d", f.issued)
		f.tokens[tok] = f.now.Add(f.ttl)
		write(http.StatusOK, map[string]any{"auth": map[string]any{"client_token": tok, "lease_duration": int(f.ttl.Seconds()), "renewable": true}})
	case r.URL.Path == "/v1/auth/token/renew-self":
		if !valid() {
			write(http.StatusForbidden, map[string]any{"errors": []string{"permission denied"}})
			return
		}
		f.renewed++
		tok := r.Header.Get("X-Vault-Token")
		f.tokens[tok] = f.now.Add(f.ttl)
		write(http.StatusOK, map[string]any{"auth": map[string]any{"client_token": tok, "lease_duration": int(f.ttl.Seconds()), "renewable": true}})
	case strings.HasPrefix(r.URL.Path, "/v1/secret/data/"):
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(f.body))
			return
		}
		if !valid() {
			write(http.StatusForbidden, map[string]any{"errors": []string{"permission denied"}})
			return
		}
		f.reads++
		version := 3
		if v := r.URL.Query().Get("version"); v == "7" {
			version = 5 // a server answering another version than asked
		}
		write(http.StatusOK, map[string]any{"data": map[string]any{
			"data":     map[string]any{"value": "the-value", "count": 3},
			"metadata": map[string]any{"version": version},
		}})
	default:
		write(http.StatusNotFound, map[string]any{"errors": []string{}})
	}
}

func fakeClient(t *testing.T, f *fakeBao, srv *httptest.Server, secretID string) *openbao.Client {
	t.Helper()
	c, err := openbao.New(openbao.Config{
		Address: srv.URL, Now: f.clock,
		Auth: openbao.AppRoleAuth("", openbao.Literal("role"), openbao.Literal(secretID)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func tenantRef() secrets.Ref {
	return secrets.DefaultLocator("test/").Locate(uuid.New(), uuid.New())
}

func TestTokenIsRenewedBeforeItExpiresAndReplacedAfter(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	r := fakeClient(t, f, srv, "the-secret-id").Reader()
	ctx, ref := context.Background(), tenantRef()

	v, err := r.Read(ctx, ref)
	if err != nil || v.Version != 3 || v.Data["value"] != "the-value" || v.Data["count"] != "3" {
		t.Fatalf("Read: %+v %v", v, err)
	}
	// Within two thirds of the TTL the token is used as it is.
	f.advance(10 * time.Second)
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if f.issued != 1 || f.renewed != 0 {
		t.Fatalf("issued %d renewed %d", f.issued, f.renewed)
	}
	// Past two thirds, before expiry: renewed, not a new login.
	f.advance(15 * time.Second)
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if f.issued != 1 || f.renewed != 1 {
		t.Fatalf("after renewal: issued %d renewed %d", f.issued, f.renewed)
	}
	// Idle past expiry: a new login.
	f.advance(time.Minute)
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if f.issued != 2 {
		t.Fatalf("after expiry: issued %d", f.issued)
	}
}

func TestRevokedTokenLogsInAgainOnce(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	r := fakeClient(t, f, srv, "the-secret-id").Reader()
	ctx, ref := context.Background(), tenantRef()
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatal(err)
	}
	f.revokeAll()
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatalf("Read after revocation: %v", err)
	}
	if f.issued != 2 || f.reads != 2 {
		t.Fatalf("issued %d reads %d", f.issued, f.reads)
	}
}

func TestPinnedReadOfAnotherVersionFails(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	ref := tenantRef()
	ref.Version = 7
	if _, err := fakeClient(t, f, srv, "the-secret-id").Reader().Read(context.Background(), ref); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Read of a version the server swapped: %v", err)
	}
}

func TestOutageIsUnavailableAndQuotesNothing(t *testing.T) {
	t.Parallel()
	ctx, ref := context.Background(), tenantRef()

	t.Run("sealed server", func(t *testing.T) {
		t.Parallel()
		f, srv := newFakeBao(t)
		r := fakeClient(t, f, srv, "the-secret-id").Reader()
		f.status, f.body = http.StatusServiceUnavailable, `{"errors":["Vault is sealed"],"data":{"value":"the-value"}}`
		_, err := r.Read(ctx, ref)
		if !errors.Is(err, secrets.ErrUnavailable) {
			t.Fatalf("Read: %v", err)
		}
		if strings.Contains(err.Error(), "the-value") || strings.Contains(err.Error(), "sealed") {
			t.Fatalf("the error quotes the response: %v", err)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		t.Parallel()
		f, srv := newFakeBao(t)
		c := fakeClient(t, f, srv, "the-secret-id")
		srv.Close()
		_, err := c.Reader().Read(ctx, ref)
		if !errors.Is(err, secrets.ErrUnavailable) {
			t.Fatalf("Read: %v", err)
		}
		_, err = c.Writer().Put(ctx, ref, map[string]string{"value": "new-value"})
		if !errors.Is(err, secrets.ErrUnavailable) || strings.Contains(err.Error(), "new-value") {
			t.Fatalf("Put: %v", err)
		}
	})
	t.Run("refused login", func(t *testing.T) {
		t.Parallel()
		f, srv := newFakeBao(t)
		_, err := fakeClient(t, f, srv, "a-wrong-secret-id").Reader().Read(ctx, ref)
		if !errors.Is(err, secrets.ErrUnavailable) {
			t.Fatalf("Read: %v", err)
		}
		if strings.Contains(err.Error(), "a-wrong-secret-id") {
			t.Fatalf("the error quotes the credential: %v", err)
		}
	})
	t.Run("credential file missing", func(t *testing.T) {
		t.Parallel()
		f, srv := newFakeBao(t)
		c, err := openbao.New(openbao.Config{Address: srv.URL, Now: f.clock,
			Auth: openbao.AppRoleAuth("", openbao.Literal("role"), openbao.File(filepath.Join(t.TempDir(), "missing")))})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Reader().Read(ctx, ref); !errors.Is(err, secrets.ErrUnavailable) {
			t.Fatalf("Read: %v", err)
		}
	})
}

func TestCredentialFileIsReadOnEveryLogin(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	file := filepath.Join(t.TempDir(), "secret-id")
	if err := os.WriteFile(file, []byte("an-old-secret-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := openbao.New(openbao.Config{Address: srv.URL, Now: f.clock,
		Auth: openbao.AppRoleAuth("", openbao.Literal("role"), openbao.File(file))})
	if err != nil {
		t.Fatal(err)
	}
	ctx, ref := context.Background(), tenantRef()
	if _, err := c.Reader().Read(ctx, ref); err == nil {
		t.Fatal("logged in with the old secret_id")
	}
	// A deploy rotates the secret_id in the file: the running process logs in with it.
	if err := os.WriteFile(file, []byte("the-secret-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Reader().Read(ctx, ref); err != nil {
		t.Fatalf("Read after the file rotated: %v", err)
	}
}

func TestRefsOutsideTheirTenantAreForbidden(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	s := fakeClient(t, f, srv, "the-secret-id").Store()
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()
	// Another tenant's path: not found to a read, forbidden to a write.
	foreign := secrets.Ref{TenantID: tenant, Path: "tenants/" + other.String() + "/x"}
	if _, err := s.Read(ctx, foreign); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("Read foreign: %v", err)
	}
	if _, err := s.Metadata(ctx, foreign); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("Metadata foreign: %v", err)
	}
	if _, err := s.Put(ctx, foreign, map[string]string{"value": "v"}); !errors.Is(err, secrets.ErrForbidden) {
		t.Errorf("Put foreign: %v", err)
	}
	if _, err := s.List(ctx, secrets.Ref{TenantID: tenant, Path: "tenants/" + other.String() + "/"}); !errors.Is(err, secrets.ErrForbidden) {
		t.Errorf("List foreign: %v", err)
	}
	for _, ref := range []secrets.Ref{
		{TenantID: uuid.Nil, Path: "tenants/" + other.String() + "/x"},
		{TenantID: tenant, Path: ""},
		{TenantID: tenant, Path: "tenants/" + tenant.String() + "/../" + other.String() + "/x"},
		{TenantID: tenant, Path: "/tenants/" + tenant.String() + "/x"},
		{TenantID: tenant, Path: "tenants/" + tenant.String() + "//x"},
		{TenantID: tenant, Path: "tenants/" + tenant.String() + "/x?version=1"},
	} {
		if _, err := s.Read(ctx, ref); !errors.Is(err, secrets.ErrForbidden) {
			t.Errorf("Read %+v: %v", ref, err)
		}
		if _, err := s.Put(ctx, ref, map[string]string{"value": "v"}); !errors.Is(err, secrets.ErrForbidden) {
			t.Errorf("Put %+v: %v", ref, err)
		}
		if err := s.Delete(ctx, ref); !errors.Is(err, secrets.ErrForbidden) {
			t.Errorf("Delete %+v: %v", ref, err)
		}
	}
	if f.reads != 0 || f.issued != 0 {
		t.Fatalf("a forbidden ref reached the server: reads %d logins %d", f.reads, f.issued)
	}
}

func TestPutAfterItsDeadlineSendsNothing(t *testing.T) {
	t.Parallel()
	f, srv := newFakeBao(t)
	w := fakeClient(t, f, srv, "the-secret-id").Writer()
	ref := tenantRef()
	ref.NotAfter = f.clock().Add(-time.Second)
	if _, err := w.Put(context.Background(), ref, map[string]string{"value": "late"}); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Put: %v", err)
	}
	if f.issued != 0 {
		t.Fatalf("a fenced write logged in: %d", f.issued)
	}
}

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()
	auth := openbao.TokenAuth(openbao.Literal("t"))
	for _, cfg := range []openbao.Config{
		{Auth: auth},
		{Address: "openbao:8200", Auth: auth},
		{Address: "ftp://openbao", Auth: auth},
		{Address: "http://openbao:8200"},
		{Address: "http://openbao:8200", Auth: auth, Mount: "a/../b"},
	} {
		if _, err := openbao.New(cfg); err == nil {
			t.Errorf("New(%+v) accepted", cfg)
		}
	}
}
