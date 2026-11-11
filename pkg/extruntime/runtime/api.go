package runtime

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
)

// APIPrefix is where the management API is mounted.
const APIPrefix = "/_ext/manage"

const (
	maxJSONBody = 1 << 20
	maxTreeBody = 64 << 20
	maxFileBody = 9 << 20
)

// SubjectFunc identifies the acting user of a request (the session user for
// the SDK controller, a credential's user for other transports).
type SubjectFunc func(r *http.Request) (spi.Subject, error)

// API is the JSON transport of manage.Service: every route decodes its input,
// calls exactly one service method and encodes the result. Rights, validation
// and audit live in the service.
type API struct {
	svc     *manage.Service
	subject SubjectFunc
	log     *slog.Logger
}

// NewAPI builds the handler tree rooted at APIPrefix.
func NewAPI(svc *manage.Service, subject SubjectFunc, log *slog.Logger) *API {
	if log == nil {
		log = slog.Default()
	}
	return &API{svc: svc, subject: subject, log: log}
}

type handler func(w http.ResponseWriter, r *http.Request, actor spi.Subject) (status int, body any, err error)

func (a *API) wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, err := a.subject(r)
		if err != nil {
			a.fail(w, err)
			return
		}
		status, body, err := h(w, r, actor)
		if err != nil {
			a.fail(w, err)
			return
		}
		if body == nil {
			w.WriteHeader(status)
			return
		}
		if raw, ok := body.(rawResponse); ok {
			w.Header().Set("Content-Type", raw.contentType)
			for k, v := range raw.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(status)
			_, _ = w.Write(raw.data)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

type rawResponse struct {
	contentType string
	headers     map[string]string
	data        []byte
}

// errorBody is the wire form of a failure.
type errorBody struct {
	Error struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		Diagnostics any    `json:"diagnostics,omitempty"`
	} `json:"error"`
}

func leafMessage(err error) string {
	parts := strings.Split(err.Error(), ": ")
	for len(parts) > 1 && !strings.ContainsAny(parts[0], " ") && strings.Contains(parts[0], ".") {
		parts = parts[1:]
	}
	return strings.Join(parts, ": ")
}

func (a *API) fail(w http.ResponseWriter, err error) {
	var body errorBody
	code := serrors.CodeOf(err)
	body.Error.Code = strings.ToLower(code.String())
	body.Error.Message = leafMessage(err)
	if code == serrors.Internal {
		a.log.Error("extruntime manage API", "error", err)
		body.Error.Message = "internal error"
	}
	var de *manage.DiagnosticsError
	var se *data.SchemaError
	switch {
	case errors.As(err, &de):
		body.Error.Diagnostics = de.Diagnostics
	case errors.As(err, &se):
		body.Error.Diagnostics = se.Diagnostics
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(serrorhttp.Status(err))
	_ = json.NewEncoder(w).Encode(body)
}

func decode(r *http.Request, w http.ResponseWriter, into any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return serrors.New(serrors.Invalid, "invalid request body: "+err.Error())
	}
	return nil
}

func readBody(r *http.Request, w http.ResponseWriter, limit int64) ([]byte, error) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		return nil, serrors.New(serrors.Invalid, "cannot read request body: "+err.Error())
	}
	return b, nil
}

func vars(r *http.Request, name string) string {
	v := mux.Vars(r)[name]
	if u, err := url.PathUnescape(v); err == nil {
		return u
	}
	return v
}

// Handler returns the routes. Mount it so that requests reach it with the
// APIPrefix intact.
func (a *API) Handler() http.Handler {
	root := mux.NewRouter()
	r := root.PathPrefix(APIPrefix).Subrouter()
	p := "/packages/{ns}"

	r.HandleFunc("/packages", a.wrap(a.listPackages)).Methods(http.MethodGet)
	r.HandleFunc("/packages", a.wrap(a.createPackage)).Methods(http.MethodPost)
	r.HandleFunc(p, a.wrap(a.getPackage)).Methods(http.MethodGet)
	r.HandleFunc(p+"/files", a.wrap(a.listFiles)).Methods(http.MethodGet)
	r.HandleFunc(p+"/files/{path:.+}", a.wrap(a.getFile)).Methods(http.MethodGet)
	r.HandleFunc(p+"/files/{path:.+}", a.wrap(a.putFile)).Methods(http.MethodPut)
	r.HandleFunc(p+"/files/{path:.+}", a.wrap(a.deleteFile)).Methods(http.MethodDelete)
	r.HandleFunc(p+"/move", a.wrap(a.moveFile)).Methods(http.MethodPost)
	r.HandleFunc(p+"/check", a.wrap(a.check)).Methods(http.MethodPost)
	r.HandleFunc(p+"/build", a.wrap(a.build)).Methods(http.MethodPost)
	r.HandleFunc(p+"/test", a.wrap(a.test)).Methods(http.MethodPost)
	r.HandleFunc(p+"/publish", a.wrap(a.publish)).Methods(http.MethodPost)
	r.HandleFunc(p+"/versions", a.wrap(a.listVersions)).Methods(http.MethodGet)
	r.HandleFunc(p+"/versions/{version}", a.wrap(a.getVersion)).Methods(http.MethodGet)
	r.HandleFunc(p+"/versions/{version}/review", a.wrap(a.review)).Methods(http.MethodPost)
	r.HandleFunc(p+"/versions/{version}/activate", a.wrap(a.activate)).Methods(http.MethodPost)
	r.HandleFunc(p+"/deactivate", a.wrap(a.deactivate)).Methods(http.MethodPost)
	r.HandleFunc(p+"/purge", a.wrap(a.purge)).Methods(http.MethodPost)
	r.HandleFunc(p+"/activations", a.wrap(a.listActivations)).Methods(http.MethodGet)
	r.HandleFunc(p+"/export", a.wrap(a.export)).Methods(http.MethodGet)
	r.HandleFunc("/import/git", a.wrap(a.importGit)).Methods(http.MethodPost)
	r.HandleFunc("/import/tree", a.wrap(a.importTree)).Methods(http.MethodPost)
	r.HandleFunc("/bindings", a.wrap(a.listBindings)).Methods(http.MethodGet)
	r.HandleFunc("/bindings", a.wrap(a.bind)).Methods(http.MethodPost)
	r.HandleFunc("/bindings/{slot}/{target}", a.wrap(a.unbind)).Methods(http.MethodDelete)
	r.HandleFunc("/owners", a.wrap(a.listOwners)).Methods(http.MethodGet)
	r.HandleFunc("/triggers/{trigger}/owner", a.wrap(a.assignOwner)).Methods(http.MethodPut)
	r.HandleFunc("/triggers/{trigger}/owner", a.wrap(a.removeOwner)).Methods(http.MethodDelete)
	r.HandleFunc("/kit", a.wrap(a.kit)).Methods(http.MethodGet)
	r.HandleFunc("/kit/modules/{module:.+}", a.wrap(a.kitModule)).Methods(http.MethodGet)
	r.HandleFunc("/catalog/natives", a.wrap(a.nativeCatalog)).Methods(http.MethodGet)
	r.HandleFunc("/audit", a.wrap(a.audit)).Methods(http.MethodGet)

	r.HandleFunc("/workflows", a.wrap(a.listWorkflows)).Methods(http.MethodGet)
	r.HandleFunc("/workflows/{id}", a.wrap(a.getWorkflow)).Methods(http.MethodGet)
	r.HandleFunc("/workflows/{id}/signal", a.wrap(a.signalWorkflow)).Methods(http.MethodPost)
	r.HandleFunc("/workflows/{id}/terminate", a.wrap(a.terminateWorkflow)).Methods(http.MethodPost)
	r.HandleFunc("/workflows/{id}/start-v2", a.wrap(a.startWorkflowV2)).Methods(http.MethodPost)
	r.HandleFunc("/workflows/{id}/resolve", a.wrap(a.resolveOccurrence)).Methods(http.MethodPost)
	r.HandleFunc("/workflows/{id}/retry", a.wrap(a.retryOccurrence)).Methods(http.MethodPost)
	r.HandleFunc("/workflows/{id}/resume", a.wrap(a.resumeWorkflow)).Methods(http.MethodPost)
	r.HandleFunc("/tasks/{id}/decide", a.wrap(a.decideTask)).Methods(http.MethodPost)
	return root
}

func (a *API) listPackages(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListPackages(r.Context(), actor)
	return http.StatusOK, out, err
}

func (a *API) createPackage(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		Namespace string `json:"namespace"`
		NPMName   string `json:"npmName"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	out, err := a.svc.CreatePackage(r.Context(), actor, in.Namespace, in.NPMName)
	return http.StatusCreated, out, err
}

func (a *API) getPackage(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.GetPackage(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) listFiles(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListFiles(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) getFile(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	content, f, err := a.svc.GetFile(r.Context(), actor, vars(r, "ns"), vars(r, "path"))
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, rawResponse{contentType: "application/octet-stream", headers: map[string]string{"ETag": `"` + f.Hash + `"`}, data: content}, nil
}

// putFile takes the raw file as body. If-Match: "<hash>" requires the
// current content hash, If-None-Match: * requires a new file.
func (a *API) putFile(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	body, err := readBody(r, w, maxFileBody)
	if err != nil {
		return 0, nil, err
	}
	var expect *string
	if m := strings.Trim(r.Header.Get("If-Match"), `"`); m != "" {
		expect = &m
	} else if r.Header.Get("If-None-Match") == "*" {
		empty := ""
		expect = &empty
	}
	out, err := a.svc.PutFile(r.Context(), actor, vars(r, "ns"), vars(r, "path"), body, expect)
	return http.StatusOK, out, err
}

func (a *API) deleteFile(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	return http.StatusNoContent, nil, a.svc.DeleteFile(r.Context(), actor, vars(r, "ns"), vars(r, "path"))
}

func (a *API) moveFile(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct{ From, To string }
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.MoveFile(r.Context(), actor, vars(r, "ns"), in.From, in.To)
}

func (a *API) check(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.Check(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) build(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.Build(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) test(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.Test(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) publish(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		Version string `json:"version"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, w, &in); err != nil && !errors.Is(err, io.EOF) {
			return 0, nil, err
		}
	}
	out, err := a.svc.Publish(r.Context(), actor, vars(r, "ns"), in.Version)
	return http.StatusCreated, out, err
}

func (a *API) listVersions(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListVersions(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) getVersion(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.GetVersion(r.Context(), actor, vars(r, "ns"), vars(r, "version"))
	return http.StatusOK, out, err
}

func (a *API) review(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		Approve            bool   `json:"approve"`
		Comment            string `json:"comment"`
		ExpectSourceHash   string `json:"expectSourceHash"`
		ExpectArtifactHash string `json:"expectArtifactHash"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	out, err := a.svc.ReviewExact(r.Context(), actor, vars(r, "ns"), vars(r, "version"), manage.ReviewInput{
		Approve: in.Approve, Comment: in.Comment, ExpectSourceHash: in.ExpectSourceHash, ExpectArtifactHash: in.ExpectArtifactHash,
	})
	return http.StatusCreated, out, err
}

func (a *API) activate(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.Activate(r.Context(), actor, vars(r, "ns"), vars(r, "version"))
	return http.StatusOK, out, err
}

func (a *API) deactivate(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	return http.StatusNoContent, nil, a.svc.Deactivate(r.Context(), actor, vars(r, "ns"))
}

func (a *API) purge(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	return http.StatusNoContent, nil, a.svc.Purge(r.Context(), actor, vars(r, "ns"))
}

func (a *API) listActivations(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListActivations(r.Context(), actor, vars(r, "ns"))
	return http.StatusOK, out, err
}

func (a *API) export(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	format := manage.ExportFormat(r.URL.Query().Get("format"))
	var arc manage.Archive
	var err error
	if v := r.URL.Query().Get("version"); v != "" {
		arc, err = a.svc.ExportVersion(r.Context(), actor, vars(r, "ns"), v, format)
	} else {
		arc, err = a.svc.ExportDraft(r.Context(), actor, vars(r, "ns"), format)
	}
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, rawResponse{contentType: arc.ContentType, data: arc.Data, headers: map[string]string{
		"Content-Disposition": `attachment; filename="` + arc.Filename + `"`, "X-Tree-Hash": arc.TreeHash,
	}}, nil
}

func (a *API) importGit(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		RepoURL string `json:"repoUrl"`
		Commit  string `json:"commit"`
		Subdir  string `json:"subdir"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	out, err := a.svc.ImportGit(r.Context(), actor, in.RepoURL, in.Commit, in.Subdir)
	return http.StatusOK, out, err
}

// importTree takes an uploaded tar or zip archive as the body.
func (a *API) importTree(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	body, err := readBody(r, w, maxTreeBody)
	if err != nil {
		return 0, nil, err
	}
	tree, err := manage.UnpackArchive(body)
	if err != nil {
		return 0, nil, err
	}
	out, err := a.svc.ImportTree(r.Context(), actor, build.Tree(tree), manage.ImportMeta{Provenance: manage.ProvenanceRuntime})
	return http.StatusOK, out, err
}

func (a *API) listBindings(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListBindings(r.Context(), actor, r.URL.Query().Get("history") == "true")
	return http.StatusOK, out, err
}

func (a *API) bind(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		SlotKind string `json:"slotKind"`
		TargetID string `json:"targetId"`
		Package  string `json:"package"`
		Version  string `json:"version"`
		Export   string `json:"export"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	out, err := a.svc.Bind(r.Context(), actor, in.SlotKind, in.TargetID, in.Package, in.Version, in.Export)
	return http.StatusCreated, out, err
}

func (a *API) unbind(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	return http.StatusNoContent, nil, a.svc.Unbind(r.Context(), actor, vars(r, "slot"), vars(r, "target"))
}

func (a *API) listOwners(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	out, err := a.svc.ListOwners(r.Context(), actor)
	return http.StatusOK, out, err
}

func (a *API) assignOwner(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	var in struct {
		UserID int64 `json:"userId"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	out, err := a.svc.AssignOwner(r.Context(), actor, vars(r, "trigger"), in.UserID)
	return http.StatusOK, out, err
}

func (a *API) removeOwner(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	return http.StatusNoContent, nil, a.svc.RemoveOwner(r.Context(), actor, vars(r, "trigger"))
}

func (a *API) kit(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	hash, modules, err := a.svc.KitModules(r.Context(), actor)
	return http.StatusOK, map[string]any{"hash": hash, "modules": modules}, err
}

// nativeCatalog serves the snapshot of the application's native contracts, the
// offline input of `iota-ext kit generate --catalog`.
func (a *API) nativeCatalog(_ http.ResponseWriter, _ *http.Request, actor spi.Subject) (int, any, error) {
	snap, err := a.svc.NativeSnapshot(actor)
	return http.StatusOK, snap, err
}

func (a *API) kitModule(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	module := strings.TrimSuffix(vars(r, "module"), ".tgz")
	arc, err := a.svc.KitTarball(r.Context(), actor, r.URL.Query().Get("hash"), module)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, rawResponse{contentType: arc.ContentType, data: arc.Data, headers: map[string]string{
		"Content-Disposition": `attachment; filename="` + arc.Filename + `"`,
	}}, nil
}

func (a *API) audit(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	q := r.URL.Query()
	f := manage.AuditFilter{Namespace: q.Get("namespace"), Action: q.Get("action")}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, nil, serrors.New(serrors.Invalid, "invalid limit")
		}
		f.Limit = n
	}
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, nil, serrors.New(serrors.Invalid, "invalid before")
		}
		f.BeforeID = n
	}
	out, err := a.svc.ListAudit(r.Context(), actor, f)
	return http.StatusOK, out, err
}

func uuidVar(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(vars(r, name))
	if err != nil {
		return uuid.Nil, serrors.New(serrors.Invalid, "invalid "+name)
	}
	return id, nil
}

func (a *API) listWorkflows(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	q := r.URL.Query()
	f := workflow.ListFilter{Status: q.Get("status"), Workflow: q.Get("workflow")}
	var err error
	if v := q.Get("limit"); v != "" {
		if f.Limit, err = strconv.Atoi(v); err != nil {
			return 0, nil, serrors.New(serrors.Invalid, "invalid limit")
		}
	}
	if v := q.Get("offset"); v != "" {
		if f.Offset, err = strconv.Atoi(v); err != nil {
			return 0, nil, serrors.New(serrors.Invalid, "invalid offset")
		}
	}
	out, err := a.svc.ListWorkflows(r.Context(), actor, f)
	return http.StatusOK, out, err
}

func (a *API) getWorkflow(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	out, err := a.svc.GetWorkflow(r.Context(), actor, id)
	return http.StatusOK, out, err
}

func (a *API) signalWorkflow(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	var in struct {
		Name    string          `json:"name"`
		Payload json.RawMessage `json:"payload"`
		Key     string          `json:"key"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.SignalWorkflow(r.Context(), actor, id, in.Name, in.Payload, in.Key)
}

func (a *API) terminateWorkflow(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.TerminateWorkflow(r.Context(), actor, id)
}

func (a *API) startWorkflowV2(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	var in struct {
		Version string `json:"version"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, w, &in); err != nil {
			return 0, nil, err
		}
	}
	out, err := a.svc.StartWorkflowV2(r.Context(), actor, id, in.Version)
	return http.StatusCreated, map[string]string{"instance": out.String()}, err
}

type occurrenceBody struct {
	Name   string          `json:"name"`
	N      int             `json:"n"`
	Result json.RawMessage `json:"result"`
}

func (a *API) resolveOccurrence(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	var in occurrenceBody
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.ResolveWorkflowOccurrence(r.Context(), actor, id, in.Name, in.N, in.Result)
}

func (a *API) retryOccurrence(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	var in occurrenceBody
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.RetryWorkflowOccurrence(r.Context(), actor, id, in.Name, in.N)
}

func (a *API) resumeWorkflow(_ http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.ResumeWorkflow(r.Context(), actor, id)
}

func (a *API) decideTask(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
	id, err := uuidVar(r, "id")
	if err != nil {
		return 0, nil, err
	}
	var in struct {
		Decision json.RawMessage `json:"decision"`
	}
	if err := decode(r, w, &in); err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, a.svc.DecideTask(r.Context(), actor, id, in.Decision)
}
