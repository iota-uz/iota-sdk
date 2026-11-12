package runtime

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// JSON request bodies of the management API.
type (
	createPackageRequest struct {
		Namespace string `json:"namespace"`
		NPMName   string `json:"npmName"`
	}
	moveRequest struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	versionRequest struct {
		Version string `json:"version"`
	}
	reviewRequest struct {
		Approve            bool   `json:"approve"`
		Comment            string `json:"comment"`
		ExpectSourceHash   string `json:"expectSourceHash"`
		ExpectArtifactHash string `json:"expectArtifactHash"`
	}
	importGitRequest struct {
		RepoURL string `json:"repoUrl"`
		Commit  string `json:"commit"`
		Subdir  string `json:"subdir"`
	}
	bindRequest struct {
		SlotKind string `json:"slotKind"`
		TargetID string `json:"targetId"`
		Package  string `json:"package"`
		Version  string `json:"version"`
		Export   string `json:"export"`
	}
	assignOwnerRequest struct {
		UserID int64 `json:"userId"`
	}
	signalRequest struct {
		Name    string          `json:"name"`
		Payload json.RawMessage `json:"payload"`
		Key     string          `json:"key"`
	}
	occurrenceRequest struct {
		Name   string          `json:"name"`
		N      int             `json:"n"`
		Result json.RawMessage `json:"result"`
	}
	// retryRequest names the occurrence to retry; unlike resolve it carries no result.
	retryRequest struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	decideRequest struct {
		Decision json.RawMessage `json:"decision"`
	}
	requeueRequest struct {
		Event string `json:"event"`
	}
)

// Small JSON responses the handlers build as maps.
type (
	kitResponse struct {
		Hash    string   `json:"hash"`
		Modules []string `json:"modules"`
	}
	requeuedResponse struct {
		Requeued int64 `json:"requeued"`
	}
)

type handleFunc func(a *API, w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error)

// route is one entry of the management API: how it is mounted and how it is
// documented. The mux registration (API.Handler) and the OpenAPI document
// (ManageOperations) are both generated from the routes table, so they cannot
// drift apart.
type route struct {
	method string
	// path is the gorilla/mux template relative to APIPrefix.
	path   string
	handle handleFunc

	id      string
	summary string
	// perm is the permission the route requires ("" when a session is enough).
	// The API enforces it before the handler runs (see API.wrap), so the table
	// is the single source; the service keeps checking the same permission as
	// defense in depth.
	perm string
	// alt is who else may call the route besides a holder of perm ("" when
	// nobody). The service decides it, so the API does not enforce perm for
	// such a route.
	alt string
	// audit is the audit action of the service call; a refused request is
	// recorded under it, as the service records its own refusals ("" when the
	// service does not audit the call).
	audit string
	// refine completes the audit row of a refusal with what only the request
	// body or query carries (nil: the path variables are enough).
	refine func(d *manage.DeniedRequest, r *http.Request, body []byte)

	query []openapi.Param
	// in is the JSON body (nil: none); inOptional makes the body optional.
	in         reflect.Type
	inOptional bool
	// rawIn is a non-JSON body (content type, description).
	rawIn *rawBody

	// status is the success status; out the JSON result type, outSchema a
	// hand-written one (binary downloads, tiny maps), outType the content type
	// of a non-JSON result.
	status    int
	out       reflect.Type
	outSchema openapi.Schema
	outType   string
	outHeader map[string]string
	// description extends the summary.
	description string
}

type rawBody struct{ contentType, description string }

var (
	permRead    = manage.PermRead
	permAuthor  = manage.PermAuthor
	permReview  = manage.PermReview
	permPublish = manage.PermPublish
	permOperate = manage.PermOperate
)

func typeOf[T any]() reflect.Type { return reflect.TypeFor[T]() }

func q(name, desc string) openapi.Param {
	return openapi.Param{Name: name, In: "query", Description: desc, Schema: openapi.Schema{"type": "string"}}
}

func qInt(name, desc string) openapi.Param {
	return openapi.Param{Name: name, In: "query", Description: desc, Schema: openapi.Schema{"type": "integer"}}
}

var binary = openapi.Schema{"type": "string", "format": "binary"}

// routes is the management API. Order is documentation order.
var routes = []route{
	{method: http.MethodGet, path: "/packages", handle: (*API).listPackages, id: "manage.listPackages", summary: "List packages", perm: "read", status: 200, out: typeOf[[]manage.Package]()},
	{method: http.MethodPost, path: "/packages", handle: (*API).createPackage, id: "manage.createPackage", refine: refineCreate, summary: "Create an empty package draft", perm: "author", audit: manage.ActionCreatePackage, in: typeOf[createPackageRequest](), status: 201, out: typeOf[manage.Package]()},
	{method: http.MethodGet, path: "/packages/{ns}", handle: (*API).getPackage, id: "manage.getPackage", summary: "Get a package", perm: "read", status: 200, out: typeOf[manage.Package]()},
	{method: http.MethodGet, path: "/packages/{ns}/files", handle: (*API).listFiles, id: "manage.listFiles", summary: "List the files of the draft", perm: "read", status: 200, out: typeOf[[]manage.File]()},
	{method: http.MethodGet, path: "/packages/{ns}/files/{path:.+}", handle: (*API).getFile, id: "manage.getFile", summary: "Read a draft file", perm: "read", status: 200, outSchema: binary, outType: "application/octet-stream", outHeader: map[string]string{"ETag": "The content hash of the file."}},
	{method: http.MethodPut, path: "/packages/{ns}/files/{path:.+}", handle: (*API).putFile, id: "manage.putFile", summary: "Write a draft file", perm: "author", audit: manage.ActionPutFile, rawIn: &rawBody{"application/octet-stream", "The file content. `If-Match: \"<hash>\"` requires the current hash, `If-None-Match: *` requires a new file."}, status: 200, out: typeOf[manage.File]()},
	{method: http.MethodDelete, path: "/packages/{ns}/files/{path:.+}", handle: (*API).deleteFile, id: "manage.deleteFile", summary: "Delete a draft file", perm: "author", audit: manage.ActionDeleteFile, status: 204},
	{method: http.MethodPost, path: "/packages/{ns}/move", handle: (*API).moveFile, id: "manage.moveFile", refine: refineMove, summary: "Move or rename a draft file", perm: "author", audit: manage.ActionMoveFile, in: typeOf[moveRequest](), status: 204},
	{method: http.MethodPost, path: "/packages/{ns}/check", handle: (*API).check, id: "manage.check", summary: "Typecheck the draft", perm: "author", audit: manage.ActionCheck, status: 200, out: typeOf[manage.CheckReport]()},
	{method: http.MethodPost, path: "/packages/{ns}/build", handle: (*API).build, id: "manage.build", summary: "Build the draft", perm: "author", audit: manage.ActionBuild, status: 200, out: typeOf[manage.BuildReport]()},
	{method: http.MethodPost, path: "/packages/{ns}/test", handle: (*API).test, id: "manage.test", summary: "Run the package tests", perm: "author", audit: manage.ActionTest, status: 200, out: typeOf[build.TestReport]()},
	{method: http.MethodPost, path: "/packages/{ns}/publish", handle: (*API).publish, id: "manage.publish", refine: refinePublish, summary: "Publish the draft as an immutable version", perm: "publish", audit: manage.ActionPublish, in: typeOf[versionRequest](), inOptional: true, status: 201, out: typeOf[manage.VersionDetail]()},
	{method: http.MethodGet, path: "/packages/{ns}/versions", handle: (*API).listVersions, id: "manage.listVersions", summary: "List versions", perm: "read", status: 200, out: typeOf[[]manage.Version]()},
	{method: http.MethodGet, path: "/packages/{ns}/versions/{version}", handle: (*API).getVersion, id: "manage.getVersion", summary: "Get a version with its manifest", perm: "read", status: 200, out: typeOf[manage.VersionDetail]()},
	{method: http.MethodPost, path: "/packages/{ns}/versions/{version}/review", handle: (*API).review, id: "manage.review", summary: "Approve or reject a version", perm: "review", audit: manage.ActionReview, in: typeOf[reviewRequest](), status: 201, out: typeOf[manage.Review]()},
	{method: http.MethodPost, path: "/packages/{ns}/versions/{version}/activate", handle: (*API).activate, id: "manage.activate", summary: "Activate a version", perm: "publish", audit: manage.ActionActivate, status: 200, out: typeOf[manage.ActivationResult]()},
	{method: http.MethodPost, path: "/packages/{ns}/deactivate", handle: (*API).deactivate, id: "manage.deactivate", summary: "Deactivate the active version", perm: "publish", audit: manage.ActionDeactivate, status: 204},
	{method: http.MethodPost, path: "/packages/{ns}/purge", handle: (*API).purge, id: "manage.purge", summary: "Purge a deactivated package's data", perm: "publish", audit: manage.ActionPurge, status: 204},
	{method: http.MethodGet, path: "/packages/{ns}/activations", handle: (*API).listActivations, id: "manage.listActivations", summary: "List the activation history", perm: "read", status: 200, out: typeOf[[]manage.Activation]()},
	{method: http.MethodGet, path: "/packages/{ns}/export", handle: (*API).export, id: "manage.export", refine: refineExport, summary: "Export the draft or a version as an archive", perm: "read", audit: manage.ActionExport,
		query:  []openapi.Param{q("format", "`tar` (default) or `zip`."), q("version", "Export this version instead of the draft.")},
		status: 200, outSchema: binary, outType: "application/octet-stream", outHeader: map[string]string{"X-Tree-Hash": "Hash of the exported tree.", "Content-Disposition": "The archive file name."}},
	{method: http.MethodPost, path: "/import/git", handle: (*API).importGit, id: "manage.importGit", summary: "Import a package from a Git repository", perm: "author", audit: manage.ActionImportGit, in: typeOf[importGitRequest](), status: 200, out: typeOf[manage.Package]()},
	{method: http.MethodPost, path: "/import/tree", handle: (*API).importTree, id: "manage.importTree", summary: "Import a package from an uploaded tar or zip archive", perm: "author", audit: manage.ActionImportTree, rawIn: &rawBody{"application/octet-stream", "A tar or zip archive of the package tree."}, status: 200, out: typeOf[manage.Package]()},
	{method: http.MethodGet, path: "/bindings", handle: (*API).listBindings, id: "manage.listBindings", summary: "List explicit bindings", perm: "read",
		query: []openapi.Param{{Name: "history", In: "query", Description: "`true` includes ended bindings.", Schema: openapi.Schema{"type": "boolean"}}}, status: 200, out: typeOf[[]manage.Binding]()},
	{method: http.MethodPost, path: "/bindings", handle: (*API).bind, id: "manage.bind", refine: refineBind, summary: "Bind a package export to a command, screen or shell slot", perm: "publish", audit: manage.ActionBind, in: typeOf[bindRequest](), status: 201, out: typeOf[manage.Binding]()},
	{method: http.MethodDelete, path: "/bindings/{slot}/{target}", handle: (*API).unbind, id: "manage.unbind", summary: "Remove a binding", perm: "publish", audit: manage.ActionUnbind, status: 204},
	{method: http.MethodGet, path: "/owners", handle: (*API).listOwners, id: "manage.listOwners", summary: "List trigger owners", perm: "read", status: 200, out: typeOf[[]manage.TriggerOwner]()},
	{method: http.MethodPut, path: "/triggers/{trigger}/owner", handle: (*API).assignOwner, id: "manage.assignOwner", summary: "Assign the owner-user of a trigger", perm: "publish", audit: manage.ActionAssignOwner, in: typeOf[assignOwnerRequest](), status: 200, out: typeOf[manage.TriggerOwner]()},
	{method: http.MethodDelete, path: "/triggers/{trigger}/owner", handle: (*API).removeOwner, id: "manage.removeOwner", summary: "Remove the owner-user of a trigger", perm: "publish", audit: manage.ActionRemoveOwner, status: 204},
	{method: http.MethodGet, path: "/kit", handle: (*API).kit, id: "manage.kit", summary: "Describe the developer kit", perm: "read", status: 200, out: typeOf[kitResponse]()},
	{method: http.MethodGet, path: "/kit/modules/{module:.+}", handle: (*API).kitModule, id: "manage.kitModule", refine: refineKitModule, summary: "Download a developer kit module", perm: "read", audit: manage.ActionKit,
		query:  []openapi.Param{q("hash", "The kit hash the module belongs to.")},
		status: 200, outSchema: binary, outType: "application/octet-stream", outHeader: map[string]string{"Content-Disposition": "The tarball file name."}},
	{method: http.MethodGet, path: "/catalog/natives", handle: (*API).nativeCatalog, id: "manage.nativeCatalog", summary: "Snapshot the native contract catalog", perm: "read", status: 200, out: nil, outSchema: openapi.Schema{"type": "object", "description": "iota.catalog/1 snapshot, the input of `iota-ext kit generate --catalog`."}},
	{method: http.MethodGet, path: "/audit", handle: (*API).audit, id: "manage.audit", summary: "Read the management audit log", perm: "read",
		query:  []openapi.Param{q("namespace", "Only this package."), q("action", "Only this action."), qInt("limit", "At most this many entries (default 100, max 500)."), qInt("before", "Entries older than this ID.")},
		status: 200, out: typeOf[[]manage.AuditEntry]()},
	{method: http.MethodGet, path: "/reference", handle: (*API).reference, id: "manage.reference", summary: "The live API reference", perm: "read",
		description: "The reference of everything installed (native contracts and the active package versions) with every namespace whole, as the extension configurator renders it. It serves holders of `Extensions.Read`; the Help Center and the MCP tools show the filtered reference to other users.",
		query:       []openapi.Param{q("locale", "Locale of the labels (default: the user's).")}, status: 200, out: typeOf[reference.Model]()},

	{method: http.MethodGet, path: "/workflows", handle: (*API).listWorkflows, id: "manage.listWorkflows", summary: "List workflow instances", perm: "read",
		query:  []openapi.Param{q("status", "Only instances in this status."), q("workflow", "Only this workflow."), qInt("limit", "Page size."), qInt("offset", "Page offset.")},
		status: 200, out: typeOf[[]workflow.Instance]()},
	{method: http.MethodGet, path: "/workflows/{id}", handle: (*API).getWorkflow, id: "manage.getWorkflow", summary: "Get a workflow instance with its history", perm: "read", status: 200, out: typeOf[workflow.History]()},
	{method: http.MethodPost, path: "/workflows/{id}/signal", handle: (*API).signalWorkflow, id: "manage.signalWorkflow", summary: "Send a signal to an instance", perm: "operate", audit: manage.ActionWorkflowSignal, in: typeOf[signalRequest](), status: 204},
	{method: http.MethodPost, path: "/workflows/{id}/terminate", handle: (*API).terminateWorkflow, id: "manage.terminateWorkflow", summary: "Terminate an instance", perm: "operate", audit: manage.ActionWorkflowTerminate, status: 204},
	{method: http.MethodPost, path: "/workflows/{id}/start-v2", handle: (*API).startWorkflowV2, id: "manage.startWorkflowV2", summary: "Start a successor instance (optionally on another version)", perm: "operate", audit: manage.ActionWorkflowStartV2, in: typeOf[versionRequest](), inOptional: true, status: 201, outSchema: openapi.Schema{"type": "object", "properties": openapi.Schema{"instance": openapi.Schema{"type": "string", "format": "uuid"}}, "required": []any{"instance"}}},
	{method: http.MethodPost, path: "/workflows/{id}/resolve", handle: (*API).resolveOccurrence, id: "manage.resolveOccurrence", summary: "Resolve a stuck occurrence with a result", perm: "operate", audit: manage.ActionWorkflowResolve, in: typeOf[occurrenceRequest](), status: 204},
	{method: http.MethodPost, path: "/workflows/{id}/retry", handle: (*API).retryOccurrence, id: "manage.retryOccurrence", summary: "Retry a failed occurrence", perm: "operate", audit: manage.ActionWorkflowRetry, in: typeOf[retryRequest](), status: 204},
	{method: http.MethodPost, path: "/workflows/{id}/resume", handle: (*API).resumeWorkflow, id: "manage.resumeWorkflow", summary: "Resume a blocked instance", perm: "operate", audit: manage.ActionWorkflowResume, status: 204},
	{method: http.MethodPost, path: "/tasks/{id}/decide", handle: (*API).decideTask, id: "manage.decideTask", summary: "Decide a human task", perm: "operate", alt: "you are the assignee of the task", in: typeOf[decideRequest](), status: 204},
	{method: http.MethodPost, path: "/subscriptions/{subscription}/requeue", handle: (*API).requeueReceipts, id: "manage.requeueReceipts", summary: "Give dead-lettered and blocked deliveries a fresh start", perm: "operate", audit: manage.ActionEventsRequeue, in: typeOf[requeueRequest](), inOptional: true, status: 200, out: typeOf[requeuedResponse]()},
}

// Audit rows of refused requests carry the same fields as the service's own,
// so a refusal reads the same whichever transport made it.

func refineCreate(d *manage.DeniedRequest, _ *http.Request, body []byte) {
	var in createPackageRequest
	_ = json.Unmarshal(body, &in)
	d.Namespace = in.Namespace
}

func refinePublish(d *manage.DeniedRequest, _ *http.Request, body []byte) {
	var in versionRequest
	_ = json.Unmarshal(body, &in)
	d.Version = in.Version
}

func refineBind(d *manage.DeniedRequest, _ *http.Request, body []byte) {
	var in bindRequest
	_ = json.Unmarshal(body, &in)
	d.Namespace, d.Version, d.Target = in.Package, in.Version, in.SlotKind+":"+in.TargetID
}

func refineMove(d *manage.DeniedRequest, _ *http.Request, body []byte) {
	var in moveRequest
	_ = json.Unmarshal(body, &in)
	d.Target = in.From + " -> " + in.To
}

func refineExport(d *manage.DeniedRequest, r *http.Request, _ []byte) {
	if v := r.URL.Query().Get("version"); v != "" {
		d.Version, d.Target = v, "version"
		return
	}
	d.Target = "draft"
}

func refineKitModule(d *manage.DeniedRequest, r *http.Request, _ []byte) {
	d.Target = strings.TrimSuffix(vars(r, "module"), ".tgz")
}

// denied is the audit row of a request refused for lack of the route's
// permission.
func (rt route) denied(w http.ResponseWriter, r *http.Request) manage.DeniedRequest {
	d := manage.DeniedRequest{Action: rt.audit, Namespace: vars(r, "ns"), Version: vars(r, "version"), Target: deniedTarget(r)}
	if rt.refine != nil {
		body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBody))
		rt.refine(&d, r, body)
	}
	return d
}

// bind returns the route's handler bound to a.
func (rt route) bind(a *API) handler {
	h := rt.handle
	return func(w http.ResponseWriter, r *http.Request, actor spi.Subject) (int, any, error) {
		return h(a, w, r, actor)
	}
}

// permission resolves the short permission label of the table to its name.
func (rt route) permission() string {
	switch rt.perm {
	case "read":
		return permRead
	case "author":
		return permAuthor
	case "review":
		return permReview
	case "publish":
		return permPublish
	case "operate":
		return permOperate
	}
	return ""
}

var pathParamDocs = map[string]string{
	"ns": "Package namespace.", "path": "File path inside the package tree.", "version": "Version label.",
	"slot": "Binding slot kind: `command`, `screen` or `shell`.", "target": "The ID of the bound target.",
	"trigger": "Trigger ID (a public endpoint, event subscription or schedule).", "id": "Workflow instance or task ID (uuid).",
	"subscription": "Subscription ID.", "module": "Kit module name, optionally with `.tgz`.",
}

// openAPIPath converts a mux template ("/files/{path:.+}") to OpenAPI form.
func openAPIPath(template string) string {
	var b strings.Builder
	for i := 0; i < len(template); i++ {
		if template[i] != '{' {
			b.WriteByte(template[i])
			continue
		}
		j := strings.IndexByte(template[i:], '}')
		if j < 0 {
			b.WriteString(template[i:])
			break
		}
		name, _, _ := strings.Cut(template[i+1:i+j], ":")
		b.WriteString("{" + name + "}")
		i += j
	}
	return b.String()
}

// ErrorBodySchema is the JSON error envelope of the management API.
func errorEnvelope(reg *openapi.Registry) openapi.Schema {
	return reg.Add("ManageError", openapi.Schema{
		"type": "object",
		"properties": openapi.Schema{"error": openapi.Schema{
			"type": "object",
			"properties": openapi.Schema{
				"code":        openapi.Schema{"type": "string", "description": "Lower-case error class: `invalid`, `not_found`, `permission_denied`, `conflict`, `unauthenticated`, `internal`, ..."},
				"message":     openapi.Schema{"type": "string"},
				"diagnostics": openapi.Schema{"description": "Build or schema diagnostics, when the failure has them."},
			},
			"required": []any{"code", "message"},
		}},
		"required": []any{"error"},
	})
}

// ManageTag is the OpenAPI tag of the management API.
const ManageTag = "manage"

// ManageOperations lists the management API as OpenAPI operations, in
// documentation order, with request and response schemas registered in reg.
func ManageOperations(reg *openapi.Registry) []openapi.Operation {
	ops := make([]openapi.Operation, 0, len(routes))
	errSchema := errorEnvelope(reg)
	for _, rt := range routes {
		path := APIPrefix + openAPIPath(rt.path)
		op := openapi.Operation{
			Method: rt.method, Path: path, ID: rt.id, Summary: rt.summary, Description: rt.description,
			Tags: []string{ManageTag}, Permission: rt.permission(), Alternative: rt.alt,
		}
		for _, name := range openapi.PathParams(path) {
			op.Params = append(op.Params, openapi.Param{Name: name, In: "path", Required: true, Description: pathParamDocs[name], Schema: openapi.Schema{"type": "string"}})
		}
		op.Params = append(op.Params, rt.query...)
		switch {
		case rt.in != nil:
			op.Request = &openapi.Body{Schema: reg.Of(rt.in), Required: !rt.inOptional}
		case rt.rawIn != nil:
			op.Request = &openapi.Body{ContentType: rt.rawIn.contentType, Schema: binary, Description: rt.rawIn.description, Required: true}
		}
		resp := openapi.Response{Status: rt.status, Headers: rt.outHeader, ContentType: rt.outType}
		switch {
		case rt.out != nil:
			resp.Schema = reg.Of(rt.out)
		case rt.outSchema != nil:
			resp.Schema = rt.outSchema
		}
		op.Responses = []openapi.Response{resp}
		op.Default = &openapi.Response{Description: "The error envelope.", Schema: errSchema}
		ops = append(ops, op)
	}
	return ops
}

// ManageRoutes summarizes the management API for the Help Center API document.
func ManageRoutes() []reference.APIRoute {
	out := make([]reference.APIRoute, 0, len(routes))
	for _, rt := range routes {
		out = append(out, reference.APIRoute{
			Method: rt.method, Path: APIPrefix + openAPIPath(rt.path), Summary: rt.summary, Permission: rt.permission(), Alternative: rt.alt,
		})
	}
	return out
}

// OpenAPIInfo describes the generated API.
func openAPIInfo(sessionCookie string) openapi.Info {
	return openapi.Info{
		Title:   "Extension runtime API",
		Version: "1",
		Description: "The HTTP interface of the extension runtime: the browser RPC for commands and queries, the endpoints " +
			"published by extension packages and, for holders of Extensions.Read, the management API. Generated from the live " +
			"catalog; it lists only what the requesting user may use.",
		SessionCookie: sessionCookie,
	}
}

// BuildOpenAPI composes the OpenAPI 3.1 document for a reference model: the
// contracts and endpoints the model holds, plus the management API when the
// model is the full one (the reader holds Extensions.Read).
func BuildOpenAPI(m *reference.Model, sessionCookie string) ([]byte, error) {
	b := openapi.New(openAPIInfo(sessionCookie))
	reg := b.Schemas()
	reference.AddContracts(b, m)
	if m.Full {
		b.Tag(ManageTag, "Package lifecycle: drafts, builds, versions, reviews, activation, bindings, workflows, audit. Requires Extensions.* permissions.")
		for _, op := range ManageOperations(reg) {
			b.Add(op)
		}
	}
	return b.JSON()
}
