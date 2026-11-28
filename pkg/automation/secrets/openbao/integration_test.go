package openbao_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets/openbao"
)

// The tests against a real OpenBao (dev mode, KV v2 at secret/) run when OPENBAO_TEST_ADDR and
// OPENBAO_TEST_TOKEN (a root token) are set, as CI's unit job does:
//
//	docker run -d -p 8200:8200 -e BAO_DEV_ROOT_TOKEN_ID=root quay.io/openbao/openbao:2.4.3
//	OPENBAO_TEST_ADDR=http://127.0.0.1:8200 OPENBAO_TEST_TOKEN=root go test ./pkg/automation/secrets/openbao/

type bao struct {
	t     *testing.T
	addr  string
	root  string
	label string // a fresh name per test: its policies, roles and paths
}

func realOpenBao(t *testing.T) *bao {
	t.Helper()
	addr, root := os.Getenv("OPENBAO_TEST_ADDR"), os.Getenv("OPENBAO_TEST_TOKEN")
	if addr == "" || root == "" {
		t.Skip("OPENBAO_TEST_ADDR and OPENBAO_TEST_TOKEN are not set")
	}
	b := &bao{t: t, addr: strings.TrimRight(addr, "/"), root: root, label: "it" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]}
	// AppRole is enabled once per server; a second enable answers 400.
	b.call(http.MethodPost, "sys/auth/approle", map[string]any{"type": "approle"}, http.StatusNoContent, http.StatusBadRequest)
	return b
}

func (b *bao) call(method, path string, body any, ok ...int) map[string]any {
	b.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			b.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, b.addr+"/v1/"+path, rd)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("X-Vault-Token", b.root)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		b.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	for _, code := range ok {
		if res.StatusCode == code {
			return out
		}
	}
	b.t.Fatalf("%s %s: status %d %v", method, path, res.StatusCode, out)
	return nil
}

// prefix is this test's environment prefix: every path the locator forms lies under it.
func (b *bao) prefix() string { return b.label + "/" }

// role creates an AppRole role with a policy of rules and returns its auth.
func (b *bao) role(name, rules string, ttl time.Duration) openbao.Auth {
	b.t.Helper()
	policy := b.label + "-" + name
	b.call(http.MethodPut, "sys/policies/acl/"+policy, map[string]any{"policy": rules}, http.StatusNoContent, http.StatusOK)
	role := b.label + "-" + name
	cfg := map[string]any{"token_policies": []string{policy}, "secret_id_ttl": "1h"}
	if ttl > 0 {
		cfg["token_ttl"] = fmt.Sprintf("%ds", int(ttl.Seconds()))
		cfg["token_max_ttl"] = fmt.Sprintf("%ds", int(ttl.Seconds()))
	}
	b.call(http.MethodPost, "auth/approle/role/"+role, cfg, http.StatusNoContent, http.StatusOK)
	idResp := b.call(http.MethodGet, "auth/approle/role/"+role+"/role-id", nil, http.StatusOK)
	secResp := b.call(http.MethodPost, "auth/approle/role/"+role+"/secret-id", map[string]any{}, http.StatusOK)
	roleID := idResp["data"].(map[string]any)["role_id"].(string)
	secretID := secResp["data"].(map[string]any)["secret_id"].(string)
	return openbao.AppRoleAuth("", openbao.Literal(roleID), openbao.Literal(secretID))
}

func (b *bao) client(auth openbao.Auth) *openbao.Client {
	b.t.Helper()
	c, err := openbao.New(openbao.Config{Address: b.addr, Auth: auth})
	if err != nil {
		b.t.Fatal(err)
	}
	return c
}

// writerRules may write values and manage their metadata, never read them.
func (b *bao) writerRules() string {
	p := b.prefix()
	return fmt.Sprintf(`
path "secret/data/%[1]stenants/*" { capabilities = ["create", "update"] }
path "secret/metadata/%[1]stenants/*" { capabilities = ["create", "update", "read", "list", "delete"] }
path "secret/destroy/%[1]stenants/*" { capabilities = ["update"] }
path "secret/config" { capabilities = ["read"] }
`, p)
}

// readerRules may read the values of the tenants given (all with none).
func (b *bao) readerRules(tenants ...uuid.UUID) string {
	if len(tenants) == 0 {
		return fmt.Sprintf(`path "secret/data/%stenants/*" { capabilities = ["read"] }`, b.prefix())
	}
	var sb strings.Builder
	for _, t := range tenants {
		fmt.Fprintf(&sb, "path \"secret/data/%stenants/%s/*\" { capabilities = [\"read\"] }\n", b.prefix(), t)
	}
	return sb.String()
}

func TestOpenBaoRotationIsReadByARunningProcess(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	w := b.client(b.role("writer", b.writerRules(), 0)).Writer()
	r := b.client(b.role("reader", b.readerRules(), 0)).Reader()
	loc := secrets.DefaultLocator(b.prefix())
	tenant := uuid.New()
	ref := loc.Locate(tenant, uuid.New())

	md, err := w.Put(ctx, ref, map[string]string{"login": "u1", "password": "p1"})
	if err != nil || md.Version != 1 {
		t.Fatalf("Put v1: %+v %v", md, err)
	}
	v, err := r.Read(ctx, ref)
	if err != nil || v.Version != 1 || v.Data["login"] != "u1" || v.Data["password"] != "p1" {
		t.Fatalf("Read v1: %+v %v", v, err)
	}
	// The rotation writes both fields as version 2; the same reader, with no restart, reads it whole.
	md, err = w.Put(ctx, ref, map[string]string{"login": "u2", "password": "p2"})
	if err != nil || md.Version != 2 {
		t.Fatalf("Put v2: %+v %v", md, err)
	}
	v, err = r.Read(ctx, ref)
	if err != nil || v.Version != 2 || v.Data["login"] != "u2" || v.Data["password"] != "p2" {
		t.Fatalf("Read latest after rotation: %+v %v", v, err)
	}
	// A pinned version is that version, whole.
	ref1 := ref
	ref1.Version = 1
	if v, err = r.Read(ctx, ref1); err != nil || v.Version != 1 || v.Data["login"] != "u1" || v.Data["password"] != "p1" {
		t.Fatalf("Read v1 pinned: %+v %v", v, err)
	}
	// Once version 2 is published the writer prunes version 1: it is gone, version 2 stays.
	if err := w.Prune(ctx, ref, 2); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err = r.Read(ctx, ref1); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("pruned version: %v", err)
	}
	if v, err = r.Read(ctx, ref); err != nil || v.Version != 2 {
		t.Fatalf("Read after prune: %+v %v", v, err)
	}
	paths, err := w.List(ctx, loc.Root(tenant))
	if err != nil || len(paths) != 1 || paths[0] != ref.Path {
		t.Fatalf("List: %v %v", paths, err)
	}
	if err := w.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := w.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
	if _, err = r.Read(ctx, ref); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("Read deleted: %v", err)
	}
	if paths, err = w.List(ctx, loc.Root(tenant)); err != nil || len(paths) != 0 {
		t.Fatalf("List after delete: %v %v", paths, err)
	}
}

func TestOpenBaoWriterRoleIsRefusedARead(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	wc := b.client(b.role("writer", b.writerRules(), 0))
	ref := secrets.DefaultLocator(b.prefix()).Locate(uuid.New(), uuid.New())
	if _, err := wc.Writer().Put(ctx, ref, map[string]string{"value": "s3cr3t-write-only"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// The Writer is not a Reader…
	if _, ok := wc.Writer().(secrets.Reader); ok {
		t.Fatal("the writer is also a reader")
	}
	// …and a reader over the writer's identity is refused by OpenBao's policy: an outage of the store to
	// the caller (the process is misconfigured), never a value.
	_, err := wc.Reader().Read(ctx, ref)
	if !errors.Is(err, secrets.ErrUnavailable) || !errors.Is(err, openbao.ErrPermissionDenied) {
		t.Fatalf("read with the writer role: %v", err)
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("the error carries the value: %v", err)
	}
	// The metadata is what it may see.
	md, err := wc.MetadataReader().Metadata(ctx, ref)
	if err != nil || md.Version != 1 || md.UpdatedAt.IsZero() {
		t.Fatalf("Metadata: %+v %v", md, err)
	}
}

func TestOpenBaoIntegrationIsRefusedAnotherTenantsRecord(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	loc := secrets.DefaultLocator(b.prefix())
	tenantA, tenantB := uuid.New(), uuid.New()
	w := b.client(b.role("writer", b.writerRules(), 0)).Writer()
	refA, refB := loc.Locate(tenantA, uuid.New()), loc.Locate(tenantB, uuid.New())
	for _, ref := range []secrets.Ref{refA, refB} {
		if _, err := w.Put(ctx, ref, map[string]string{"value": "v-" + ref.TenantID.String()}); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	// A reader whose policy is tenant A's.
	r := b.client(b.role("reader-a", b.readerRules(tenantA), 0)).Reader()
	if v, err := r.Read(ctx, refA); err != nil || v.Data["value"] != "v-"+tenantA.String() {
		t.Fatalf("own record: %+v %v", v, err)
	}
	// Tenant B's record under tenant B's ref: OpenBao's policy refuses it.
	if _, err := r.Read(ctx, refB); !errors.Is(err, openbao.ErrPermissionDenied) {
		t.Fatalf("another tenant's record by policy: %v", err)
	}
	// Tenant B's path asked for as tenant A: the adapter refuses it before any request, and does not tell
	// that it exists elsewhere.
	forged := secrets.Ref{TenantID: tenantA, Path: refB.Path}
	all := b.client(b.role("reader-all", b.readerRules(), 0)).Reader()
	if _, err := all.Read(ctx, forged); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("another tenant's path under this tenant: %v", err)
	}
	if _, err := w.Put(ctx, forged, map[string]string{"value": "x"}); !errors.Is(err, secrets.ErrForbidden) {
		t.Fatalf("write to another tenant's path: %v", err)
	}
}

func TestOpenBaoLogsInAgainWhenTheTokenExpires(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	w := b.client(b.role("writer", b.writerRules(), 0)).Writer()
	r := b.client(b.role("reader", b.readerRules(), 2*time.Second)).Reader()
	ref := secrets.DefaultLocator(b.prefix()).Locate(uuid.New(), uuid.New())
	if _, err := w.Put(ctx, ref, map[string]string{"value": "a"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := r.Read(ctx, ref); err != nil {
		t.Fatalf("Read: %v", err)
	}
	// Past the role's max TTL: the token is dead and cannot be renewed; the client logs in again.
	time.Sleep(3 * time.Second)
	if _, err := w.Put(ctx, ref, map[string]string{"value": "b"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if v, err := r.Read(ctx, ref); err != nil || v.Data["value"] != "b" {
		t.Fatalf("Read after expiry: %+v %v", v, err)
	}
}

func TestOpenBaoKeepsVersionsAndCountsConcurrentWrites(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	w := b.client(b.role("writer", b.writerRules(), 0)).Writer()
	ref := secrets.DefaultLocator(b.prefix()).Locate(uuid.New(), uuid.New())

	// Concurrent writes of one path each get a version of their own (check-and-set, retried).
	const n = 6
	got := make(chan int, n)
	errs := make(chan error, n)
	for i := range n {
		go func() {
			md, err := w.Put(ctx, ref, map[string]string{"value": fmt.Sprint(i)})
			errs <- err
			got <- md.Version
		}()
	}
	seen := map[int]bool{}
	for range n {
		if err := <-errs; err != nil {
			t.Fatalf("Put: %v", err)
		}
		seen[<-got] = true
	}
	for v := 1; v <= n; v++ {
		if !seen[v] {
			t.Fatalf("versions %v: %d is missing", seen, v)
		}
	}
	// The path's retention is the adapter's: 100 versions, none expiring.
	md := b.call(http.MethodGet, "secret/metadata/"+ref.Path, nil, http.StatusOK)["data"].(map[string]any)
	if md["max_versions"].(float64) != 100 || md["delete_version_after"] != "0s" {
		t.Fatalf("retention: %v %v", md["max_versions"], md["delete_version_after"])
	}
	// A path whose retention was changed outside the adapter is not written to.
	b.call(http.MethodPost, "secret/metadata/"+ref.Path, map[string]any{"max_versions": 3}, http.StatusNoContent)
	if _, err := w.Put(ctx, ref, map[string]string{"value": "x"}); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Put under foreign retention: %v", err)
	}
}

func TestOpenBaoRefusesWritesToAMountThatExpiresVersions(t *testing.T) {
	t.Parallel()
	b := realOpenBao(t)
	ctx := context.Background()
	mount := b.label + "-kv"
	b.call(http.MethodPost, "sys/mounts/"+mount, map[string]any{"type": "kv", "options": map[string]any{"version": "2"}}, http.StatusNoContent)
	t.Cleanup(func() { b.call(http.MethodDelete, "sys/mounts/"+mount, nil, http.StatusNoContent) })
	// A new KV v2 mount upgrades itself for a moment and answers 400 meanwhile.
	for i := 0; ; i++ {
		if out := b.call(http.MethodPost, mount+"/config", map[string]any{"delete_version_after": "1h"}, http.StatusNoContent, http.StatusBadRequest); out == nil {
			break
		}
		if i == 50 {
			t.Fatal("the mount does not finish its upgrade")
		}
		time.Sleep(100 * time.Millisecond)
	}
	c, err := openbao.New(openbao.Config{Address: b.addr, Mount: mount, Auth: openbao.TokenAuth(openbao.Literal(b.root))})
	if err != nil {
		t.Fatal(err)
	}
	ref := secrets.DefaultLocator("").Locate(uuid.New(), uuid.New())
	if _, err := c.Writer().Put(ctx, ref, map[string]string{"value": "x"}); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Put to an expiring mount: %v", err)
	}
	b.call(http.MethodPost, mount+"/config", map[string]any{"delete_version_after": "0s"}, http.StatusNoContent)
	c, _ = openbao.New(openbao.Config{Address: b.addr, Mount: mount, Auth: openbao.TokenAuth(openbao.Literal(b.root))})
	if md, err := c.Writer().Put(ctx, ref, map[string]string{"value": "x"}); err != nil || md.Version != 1 {
		t.Fatalf("Put: %+v %v", md, err)
	}
}
