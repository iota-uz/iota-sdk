package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// resp is an HTTP response read in full.
type resp struct {
	Status int
	Body   []byte
	Header http.Header
}

func (r resp) String() string { return string(r.Body) }

// httpClient never follows redirects: a redirect is an observable answer.
var httpClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Timeout:       2 * time.Minute,
}

// do sends a request to the application as the actor (nil: anonymous). The
// session travels as the session cookie the browser would hold.
func (e *env) do(a *actor, method, path string, body io.Reader, headers ...string) resp {
	e.t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, body)
	require.NoError(e.t, err)
	if a != nil {
		req.AddCookie(&http.Cookie{Name: "sid", Value: a.token})
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := httpClient.Do(req)
	require.NoError(e.t, err)
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	require.NoError(e.t, err)
	return resp{Status: res.StatusCode, Body: out, Header: res.Header}
}

func (e *env) get(a *actor, path string) resp { return e.do(a, http.MethodGet, path, nil) }

func (e *env) postForm(a *actor, path string, form url.Values) resp {
	return e.do(a, http.MethodPost, path, strings.NewReader(form.Encode()), "Content-Type", "application/x-www-form-urlencoded")
}

// mgmt performs a management API call and decodes a JSON answer into out.
func (e *env) mgmt(a *actor, method, path string, in any, want int, out any) resp {
	e.t.Helper()
	var body io.Reader
	switch v := in.(type) {
	case nil:
	case []byte:
		body = bytes.NewReader(v)
	default:
		raw, err := json.Marshal(in)
		require.NoError(e.t, err)
		body = bytes.NewReader(raw)
	}
	r := e.do(a, method, runtime.APIPrefix+path, body)
	require.Equal(e.t, want, r.Status, "%s %s: %s", method, path, r.Body)
	if out != nil {
		require.NoError(e.t, json.Unmarshal(r.Body, out), string(r.Body))
	}
	return r
}

// ---- package trees ----

// tree is a package source tree: relative path -> content.
type tree map[string]string

// loadTree reads testdata/<dir> as a package tree.
func loadTree(t testing.TB, dir string) tree {
	t.Helper()
	root := filepath.Join("pkg", "extruntime", "e2e", "testdata", dir)
	out := tree{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, out, dir)
	return out
}

// with returns a copy of the tree with every occurrence of old replaced in
// the named file (the file must contain it).
func (tr tree) with(file, old, repl string) tree {
	c := tree{}
	for k, v := range tr {
		c[k] = v
	}
	src, ok := c[file]
	if !ok || !strings.Contains(src, old) {
		panic("tree.with: " + file + " does not contain " + old)
	}
	c[file] = strings.ReplaceAll(src, old, repl)
	return c
}

// put replaces the whole draft of a package through the management API: it
// creates the package when needed, writes every file and removes the draft
// files the tree no longer has.
func (e *env) put(a *actor, ns string, tr tree) {
	e.t.Helper()
	status := e.do(a, http.MethodGet, runtime.APIPrefix+"/packages/"+ns, nil).Status
	if status == http.StatusNotFound {
		e.mgmt(a, http.MethodPost, "/packages", map[string]string{"namespace": ns}, http.StatusCreated, nil)
	}
	var existing []manage.File
	e.mgmt(a, http.MethodGet, "/packages/"+ns+"/files", nil, http.StatusOK, &existing)
	have := map[string]string{}
	for _, f := range existing {
		have[f.Path] = f.Hash
	}
	paths := make([]string, 0, len(tr))
	for p := range tr {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		delete(have, p)
		r := e.do(a, http.MethodPut, runtime.APIPrefix+"/packages/"+ns+"/files/"+p, strings.NewReader(tr[p]))
		require.Equal(e.t, http.StatusOK, r.Status, "put %s: %s", p, r.Body)
	}
	for p := range have {
		e.mgmt(a, http.MethodDelete, "/packages/"+ns+"/files/"+p, nil, http.StatusNoContent, nil)
	}
}

func (e *env) check(a *actor, ns string) manage.CheckReport {
	var out manage.CheckReport
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/check", nil, http.StatusOK, &out)
	return out
}

func (e *env) buildPkg(a *actor, ns string) manage.BuildReport {
	var out manage.BuildReport
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/build", nil, http.StatusOK, &out)
	return out
}

func (e *env) testPkg(a *actor, ns string) build.TestReport {
	var out build.TestReport
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/test", nil, http.StatusOK, &out)
	return out
}

func (e *env) publish(a *actor, ns, version string) manage.VersionDetail {
	e.t.Helper()
	var out manage.VersionDetail
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/publish", map[string]string{"version": version}, http.StatusCreated, &out)
	return out
}

func (e *env) review(a *actor, ns string, v manage.VersionDetail) {
	e.t.Helper()
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/versions/"+v.Version.Version+"/review", map[string]any{
		"approve": true, "comment": "reviewed by the author's team", "expectSourceHash": v.SourceHash, "expectArtifactHash": v.ArtifactHash,
	}, http.StatusCreated, nil)
}

func (e *env) activate(a *actor, ns, version string) manage.ActivationResult {
	e.t.Helper()
	var out manage.ActivationResult
	e.mgmt(a, http.MethodPost, "/packages/"+ns+"/versions/"+version+"/activate", nil, http.StatusOK, &out)
	return out
}

// release is the author's path from a draft to a live version: check, build,
// publish, self-review, activate.
func (e *env) release(a *actor, ns, version string) manage.ActivationResult {
	e.t.Helper()
	chk := e.check(a, ns)
	require.False(e.t, chk.HasErrors, "check: %+v", chk.Diagnostics)
	v := e.publish(a, ns, version)
	e.review(a, ns, v)
	return e.activate(a, ns, version)
}

// publishFrom puts a tree, publishes, reviews and activates it.
func (e *env) deploy(a *actor, ns, version string, tr tree) manage.ActivationResult {
	e.t.Helper()
	e.put(a, ns, tr)
	return e.release(a, ns, version)
}

func (e *env) bind(a *actor, slot, target, pkg, version, export string, want int) {
	e.t.Helper()
	e.mgmt(a, http.MethodPost, "/bindings", map[string]string{
		"slotKind": slot, "targetId": target, "package": pkg, "version": version, "export": export,
	}, want, nil)
}

func (e *env) unbind(a *actor, slot, target string) {
	e.t.Helper()
	e.mgmt(a, http.MethodDelete, "/bindings/"+slot+"/"+target, nil, http.StatusNoContent, nil)
}

func (e *env) assignOwner(a *actor, trigger string, owner *actor) {
	e.t.Helper()
	e.mgmt(a, http.MethodPut, "/triggers/"+trigger+"/owner", map[string]int64{"userId": int64(owner.id)}, http.StatusOK, nil)
}

// ---- browser RPC ----

// rpcResult is the answer of POST /_ext/rpc.
type rpcResult struct {
	Status int
	OK     bool            `json:"ok"`
	Value  json.RawMessage `json:"value"`
	Error  *struct {
		Kind    string          `json:"kind"`
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Params  json.RawMessage `json:"params"`
		Issues  []struct {
			Code   string          `json:"code"`
			Phase  string          `json:"phase"`
			Source string          `json:"source"`
			Path   json.RawMessage `json:"path"`
			Params json.RawMessage `json:"params"`
		} `json:"issues"`
	} `json:"error"`
	Invocation string `json:"invocation"`
}

// rpc calls a command or query as the browser does.
func (e *env) rpc(a *actor, kind, resource string, input any) rpcResult {
	e.t.Helper()
	raw, err := json.Marshal(map[string]any{"kind": kind, "resource": resource, "input": input})
	require.NoError(e.t, err)
	r := e.do(a, http.MethodPost, "/_ext/rpc", bytes.NewReader(raw), "Content-Type", "application/json")
	var out rpcResult
	require.NoError(e.t, json.Unmarshal(r.Body, &out), string(r.Body))
	out.Status = r.Status
	return out
}

// mustRPC fails unless the call succeeds, and decodes its value.
func (e *env) mustRPC(a *actor, kind, resource string, input any, into any) {
	e.t.Helper()
	res := e.rpc(a, kind, resource, input)
	require.True(e.t, res.OK, "rpc %s %s: status %d error %+v", kind, resource, res.Status, res.Error)
	if into != nil {
		require.NoError(e.t, json.Unmarshal(res.Value, into), string(res.Value))
	}
}

// ---- workflows through the management API ----

func (e *env) workflows(a *actor, query string) []workflow.Instance {
	e.t.Helper()
	var out []workflow.Instance
	path := "/workflows"
	if query != "" {
		path += "?" + query
	}
	e.mgmt(a, http.MethodGet, path, nil, http.StatusOK, &out)
	return out
}

func (e *env) workflow(a *actor, id uuid.UUID) workflow.History {
	e.t.Helper()
	var out workflow.History
	e.mgmt(a, http.MethodGet, "/workflows/"+id.String(), nil, http.StatusOK, &out)
	return out
}

// waitStatus waits until the instance has the status and returns it.
func (e *env) waitStatus(a *actor, id uuid.UUID, status string) workflow.Instance {
	e.t.Helper()
	var last workflow.Instance
	e.eventually("workflow "+id.String()+" reaches "+status, 60*time.Second, func() bool {
		last = e.workflow(a, id).Instance
		return last.Status == status
	})
	return last
}

// instancesOf lists the instances of a workflow, oldest first.
func (e *env) instancesOf(a *actor, wf string) []workflow.Instance {
	e.t.Helper()
	list := e.workflows(a, "workflow="+url.QueryEscape(wf)+"&limit=500")
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list
}
