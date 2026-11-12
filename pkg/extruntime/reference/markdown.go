package reference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// Document paths inside the Help Center mount.
const (
	// IndexPath is the overview of the reference.
	IndexPath = "reference/index"
	// APIPath is the HTTP API document.
	APIPath = "api"
	// GuidesPath and ReferencePath are the categories of the tree.
	GuidesPath    = "guides"
	ReferencePath = "reference"
	// OpenAPIPath is where the OpenAPI 3.1 document is served.
	OpenAPIPath = "/_ext/openapi.json"
	// RPCPath is the browser transport for commands and queries.
	RPCPath = "/_ext/rpc"
)

// NamespacePath is the document path of a namespace's reference.
func NamespacePath(namespace string) string { return ReferencePath + "/" + namespace }

// docURL links from one document to another of the same mount. Documents are
// at most two segments deep, so the link is relative to the document: "../x/y"
// from a nested document, "x/y" from the top level.
func docURL(from, to string) string {
	if strings.Contains(from, "/") {
		return "../" + to
	}
	return to
}

// text makes untrusted package text safe to put in a markdown line: one line,
// no HTML, no markdown syntax.
func text(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = html.EscapeString(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '`', '*', '_', '[', ']', '|', '#', '~':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// code puts s in a code span, widening the delimiter around embedded backticks.
func code(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "``"
	}
	ticks := "`"
	for strings.Contains(s, ticks) {
		ticks += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return ticks + " " + s + " " + ticks
	}
	return ticks + s + ticks
}

// cell is a table cell: single line, pipes escaped.
func cell(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", `\|`)
}

// indentJSON pretty-prints a JSON document; an invalid one is returned as is.
func indentJSON(raw []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return raw
	}
	return buf.Bytes()
}

// fence renders a fenced code block whose fence is longer than any backtick
// run inside it.
func fence(lang string, body []byte) string {
	n, run := 3, 0
	for _, c := range body {
		if c == '`' {
			run++
			n = max(n, run+1)
		} else {
			run = 0
		}
	}
	f := strings.Repeat("`", n)
	return f + lang + "\n" + strings.TrimRight(string(body), "\n") + "\n" + f + "\n"
}

type md struct {
	strings.Builder
	m    *Model
	from string
}

func (w *md) line(format string, args ...any) { fmt.Fprintf(&w.Builder, format+"\n", args...) }
func (w *md) blank()                          { w.WriteByte('\n') }

// itemRef links to the item's namespace document when the model holds it.
func (w *md) itemRef(id string) string {
	if it, ok := w.m.Lookup(id); ok {
		if ns := namespaceOf(w.m, it.ID); ns != "" {
			return "[" + code(id) + "](" + docURL(w.from, NamespacePath(ns)) + ")"
		}
	}
	return code(id)
}

func namespaceOf(m *Model, id string) string {
	for _, ns := range m.Namespaces {
		for _, it := range ns.Items {
			if it.ID == id {
				return ns.Name
			}
		}
	}
	return ""
}

func (w *md) itemRefs(ids []string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = w.itemRef(id)
	}
	return strings.Join(parts, ", ")
}

// RenderIndex is the overview document of the reference.
func RenderIndex(m *Model, apiLinks bool) []byte {
	w := &md{m: m, from: IndexPath}
	w.line("# Extension reference")
	w.blank()
	w.line("What is installed in this application right now: the native contracts the application registers and the activated extension packages. It is generated from the live catalog and shows what **you** may use; a resource that needs a permission you do not hold is not listed.")
	w.blank()
	if apiLinks {
		w.line("The HTTP interface of the same contracts is described in [HTTP API](%s) and as an [OpenAPI 3.1 document](%s).", docURL(w.from, APIPath), OpenAPIPath)
		w.blank()
	}
	if len(m.Namespaces) == 0 {
		w.line("Nothing is visible to you yet: no package is active, or none of its contracts is open to your permissions.")
		return []byte(w.String())
	}
	w.line("| Namespace | Source | Resources | Endpoints |")
	w.line("| --- | --- | ---: | ---: |")
	for _, ns := range m.Namespaces {
		w.line("| [%s](%s) | %s | %d | %d |", code(ns.Name), docURL(w.from, NamespacePath(ns.Name)), sourceLabel(ns), len(ns.Items), len(ns.Endpoints))
	}
	w.blank()
	w.line("Resource kinds: entities and overlays hold data; commands, operations and queries are callable; events, workflows and notices describe what happens; routes, pages, slots, cells, columns and the shell shape the user interface.")
	return []byte(w.String())
}

func sourceLabel(ns Namespace) string {
	switch {
	case ns.Package != nil && ns.Native:
		return "package " + text(ns.Package.Version) + " + native"
	case ns.Package != nil:
		return "package " + text(ns.Package.Version)
	}
	return "native"
}

var kindTitles = []struct {
	kind  manifest.ResourceKind
	title string
}{
	{manifest.ResEntity, "Entities"}, {manifest.ResOverlay, "Overlays"}, {manifest.ResCommand, "Commands"},
	{manifest.ResOperation, "Operations"}, {manifest.ResQuery, "Queries"}, {manifest.ResEvent, "Events"},
	{manifest.ResWorkflow, "Workflows"}, {manifest.ResNotice, "Notices"}, {manifest.ResIssue, "Issues"},
	{manifest.ResDomainError, "Domain errors"}, {manifest.ResPermission, "Permissions"}, {manifest.ResRoute, "Routes"},
	{manifest.ResPage, "Pages"}, {manifest.ResNavigation, "Navigation"}, {manifest.ResSlot, "Slots"},
	{manifest.ResCell, "Cells"}, {manifest.ResColumn, "Columns"}, {manifest.ResShell, "Shell"},
	{manifest.ResContactResolver, "Contact resolvers"}, {manifest.ResDeliveryPolicy, "Delivery policies"},
}

// RenderNamespace is the reference document of one namespace.
func RenderNamespace(m *Model, name string) ([]byte, bool) {
	ns, ok := m.Namespace(name)
	if !ok {
		return nil, false
	}
	w := &md{m: m, from: NamespacePath(name)}
	w.line("# %s", code(ns.Name))
	w.blank()
	switch {
	case ns.Package != nil:
		w.line("Package %s, active version **%s**.", code(ns.Package.Namespace), text(ns.Package.Version))
	default:
		w.line("Native contracts registered by the application.")
	}
	w.blank()
	w.line("[All namespaces](%s)", docURL(w.from, IndexPath))
	w.blank()
	for _, kt := range kindTitles {
		var items []Item
		for _, it := range ns.Items {
			if it.Kind == kt.kind {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			continue
		}
		w.line("## %s", kt.title)
		w.blank()
		for _, it := range items {
			w.item(it)
		}
	}
	w.namespaceTables(ns)
	return []byte(w.String()), true
}

func (w *md) item(it Item) {
	title := code(it.ID)
	if it.Label != "" {
		title += " — " + text(it.Label)
	}
	w.line("### %s", title)
	w.blank()
	meta := []string{string(it.Kind)}
	if it.Atomic {
		meta = append(meta, "atomic")
	}
	switch {
	case it.Native:
		meta = append(meta, "native")
	case it.Package != "":
		meta = append(meta, "package "+code(it.Package)+" "+text(it.Version))
	}
	if it.Permission != "" {
		meta = append(meta, "permission "+code(it.Permission))
	}
	if it.KeyType != "" {
		meta = append(meta, "key "+code(it.KeyType))
	}
	if it.Subject != "" {
		meta = append(meta, "subject "+w.itemRef(it.Subject))
	}
	if it.Base != "" {
		meta = append(meta, "extends "+w.itemRef(it.Base))
	}
	w.line("%s", strings.Join(meta, " · "))
	w.blank()
	if it.Description != "" {
		w.line("%s", text(it.Description))
		w.blank()
	}
	w.access(it)
	w.fields(it)
	w.types(it)
	w.relations(it)
}

func (w *md) access(it Item) {
	if it.Access == nil || len(it.Access.Read) == 0 {
		return
	}
	var rules []string
	for _, r := range it.Access.Read {
		who := "everyone"
		if r.Permission != "" {
			who = code(r.Permission)
		}
		switch {
		case r.AllRows:
			rules = append(rules, who+": all rows")
		case r.Filtered:
			rules = append(rules, who+": matching rows")
		default:
			rules = append(rules, who+": no rows")
		}
	}
	w.line("Read access: %s.", strings.Join(rules, "; "))
	w.blank()
}

func (w *md) fields(it Item) {
	if len(it.Fields) == 0 {
		return
	}
	w.line("| Field | Type | Required | Notes |")
	w.line("| --- | --- | :---: | --- |")
	for _, f := range it.Fields {
		var notes []string
		if f.Label != "" {
			notes = append(notes, text(f.Label))
		}
		if f.Unique {
			notes = append(notes, "unique")
		}
		if f.Indexed {
			notes = append(notes, "indexed")
		}
		if len(f.Default) > 0 {
			notes = append(notes, "default "+code(string(f.Default)))
		}
		if len(f.Permissions) > 0 {
			perms := make([]string, len(f.Permissions))
			for i, p := range f.Permissions {
				perms[i] = code(p)
			}
			notes = append(notes, "needs "+strings.Join(perms, " + "))
		}
		if len(f.RenamedAs) > 0 {
			notes = append(notes, "was "+code(strings.Join(f.RenamedAs, ", ")))
		}
		req := ""
		if f.Required {
			req = "yes"
		}
		w.line("| %s | %s | %s | %s |", code(f.ID), cell(code(f.Type)), req, cell(strings.Join(notes, "; ")))
	}
	w.blank()
	if it.Record != nil {
		w.typeBlock("Record", it.Record)
	}
}

func (w *md) types(it Item) {
	for _, t := range []struct {
		title string
		doc   *TypeDoc
	}{{"Input", it.Input}, {"Output", it.Output}, {"Payload", it.Payload}, {"Parameters", it.Params}, {"Props", it.Props}} {
		if t.doc != nil {
			w.typeBlock(t.title, t.doc)
		}
	}
	for _, s := range it.Signals {
		sig := s.Payload
		w.typeBlock("Signal "+code(s.Name), &sig)
	}
}

func (w *md) typeBlock(title string, t *TypeDoc) {
	w.line("**%s** %s", title, code(t.Signature))
	w.blank()
	w.line("Example:")
	w.blank()
	w.WriteString(fence("json", indentJSON(t.Example)))
	w.blank()
	w.line("JSON Schema:")
	w.blank()
	w.WriteString(fence("json", Marshal(t.Schema)))
	w.blank()
}

func (w *md) relations(it Item) {
	if len(it.Errors) > 0 {
		w.line("Errors: %s.", w.itemRefs(it.Errors))
		w.blank()
	}
	if it.Path != "" || it.Route != "" || it.Target != "" {
		var parts []string
		if it.Path != "" {
			parts = append(parts, "path "+code(it.Path))
		}
		if it.Route != "" {
			parts = append(parts, "route "+w.itemRef(it.Route))
		}
		if it.Target != "" {
			parts = append(parts, "target "+w.itemRef(it.Target))
		}
		if it.Order != 0 {
			parts = append(parts, "order "+strconv.Itoa(it.Order))
		}
		w.line("%s.", strings.Join(parts, ", "))
		w.blank()
	}
	if it.Nav != nil {
		var parts []string
		if it.Nav.Label != "" {
			parts = append(parts, text(it.Nav.Label))
		}
		if it.Nav.Group != "" {
			parts = append(parts, "group "+code(it.Nav.Group))
		}
		if it.Nav.Workspace != "" {
			parts = append(parts, "workspace "+code(it.Nav.Workspace))
		}
		w.line("Navigation: %s.", strings.Join(parts, ", "))
		w.blank()
	}
	if it.List != "" || it.Overlay != "" || it.Field != "" {
		var flags []string
		for _, f := range []struct {
			on   bool
			name string
		}{{it.Filterable, "filterable"}, {it.Sortable, "sortable"}, {it.Exportable, "exportable"}} {
			if f.on {
				flags = append(flags, f.name)
			}
		}
		w.line("Column of list %s, field %s of %s; %s.", code(it.List), code(it.Field), w.itemRef(it.Overlay), strings.Join(flags, ", "))
		w.blank()
	}
	if nav := it.Navigation; nav != nil {
		for _, g := range []struct {
			title string
			items []IDLabel
		}{{"Groups", nav.Groups}, {"Workspaces", nav.Workspaces}, {"Palette", nav.Palette}} {
			if len(g.items) == 0 {
				continue
			}
			parts := make([]string, len(g.items))
			for i, x := range g.items {
				parts[i] = code(x.ID)
				if x.Label != "" {
					parts[i] += " (" + text(x.Label) + ")"
				}
			}
			w.line("%s: %s.", g.title, strings.Join(parts, ", "))
			w.blank()
		}
		if len(nav.Patches) > 0 {
			w.line("Patches: %s.", w.itemRefs(nav.Patches))
			w.blank()
		}
	}
	if len(it.Channels) > 0 {
		w.line("Template channels: %s.", strings.Join(it.Channels, ", "))
		w.blank()
	}
	if len(it.Guards) > 0 {
		parts := make([]string, len(it.Guards))
		for i, g := range it.Guards {
			parts[i] = code(g.ID) + " (" + g.Phase + ", " + code(g.Package) + ")"
		}
		w.line("Guards: %s.", strings.Join(parts, ", "))
		w.blank()
	}
	if len(it.ExtendedBy) > 0 {
		w.line("Extended by: %s.", w.itemRefs(it.ExtendedBy))
		w.blank()
	}
	if len(it.Subscribers) > 0 {
		codes := make([]string, len(it.Subscribers))
		for i, s := range it.Subscribers {
			codes[i] = code(s)
		}
		w.line("Subscribed by: %s.", strings.Join(codes, ", "))
		w.blank()
	}
	if refs := refTargets(it.res); len(refs) > 0 {
		w.line("Refers to: %s.", w.itemRefs(refs))
		w.blank()
	}
}

// refTargets lists the entities the resource's types point at with ref.
func refTargets(r manifest.Resource) []string {
	seen := map[string]bool{}
	var walk func(t *manifest.Type)
	walk = func(t *manifest.Type) {
		if t == nil {
			return
		}
		if t.Kind == manifest.KindRef && t.Entity != "" {
			seen[t.Entity] = true
		}
		walk(t.Items)
		for i := range t.Properties {
			walk(&t.Properties[i].Type)
		}
	}
	for _, t := range []*manifest.Type{r.Input, r.Output, r.Payload, r.Params, r.Props} {
		walk(t)
	}
	for _, s := range r.Signals {
		walk(&s)
	}
	for _, f := range r.Fields {
		walk(&f.Type)
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		if id != r.ID {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (w *md) namespaceTables(ns Namespace) {
	if len(ns.Endpoints) > 0 {
		w.line("## Endpoints")
		w.blank()
		w.line("| Method | URL | Access |")
		w.line("| --- | --- | --- |")
		for _, e := range ns.Endpoints {
			access := "signed-in user"
			if e.Public {
				access = "public (runs as the trigger owner)"
			}
			w.line("| %s | %s | %s |", cell(e.Method), code(e.URL), access)
		}
		w.blank()
	}
	if len(ns.Schedules) > 0 {
		w.line("## Schedules")
		w.blank()
		w.line("| ID | Cron | Time zone |")
		w.line("| --- | --- | --- |")
		for _, s := range ns.Schedules {
			w.line("| %s | %s | %s |", code(s.ID), code(s.Cron), cell(s.Timezone))
		}
		w.blank()
	}
	if len(ns.Subscriptions) > 0 {
		w.line("## Subscriptions")
		w.blank()
		w.line("| ID | Event |")
		w.line("| --- | --- |")
		for _, s := range ns.Subscriptions {
			w.line("| %s | %s |", code(s.ID), w.itemRef(s.Event))
		}
		w.blank()
	}
	if len(ns.Guards) > 0 {
		w.line("## Guards")
		w.blank()
		w.line("| ID | Target | Phase |")
		w.line("| --- | --- | --- |")
		for _, g := range ns.Guards {
			w.line("| %s | %s | %s |", code(g.ID), w.itemRef(g.Target), cell(g.Phase))
		}
		w.blank()
	}
}

// APIRoute summarizes one route of the management API for the API document.
type APIRoute struct {
	Method     string
	Path       string
	Summary    string
	Permission string
}

// RenderAPI is the HTTP API document: the browser RPC, the package endpoints
// and (for readers who may use it) the management API.
func RenderAPI(m *Model, manage []APIRoute) []byte {
	w := &md{m: m, from: APIPath}
	w.line("# HTTP API")
	w.blank()
	w.line("Everything on this page is also available as an [OpenAPI 3.1 document](%s) (JSON, signed-in users only; it lists what you may use). Import it into Swagger UI, Postman or a code generator.", OpenAPIPath)
	w.blank()
	w.line("All endpoints use the browser session cookie; state-changing requests must come from this application's origin.")
	w.blank()

	w.line("## Calling commands and queries")
	w.blank()
	w.line("`POST %s` invokes a command or query as the signed-in user, with the permissions the user holds now. It is a same-origin browser transport: cross-site requests are refused, and it is not a public integration API. Use a package endpoint for integrations.", RPCPath)
	w.blank()
	w.WriteString(fence("json", []byte(`{ "kind": "command", "resource": "acme.crm.renew", "input": { } }`)))
	w.blank()
	w.line("`kind` is `query`, `command` or `atomic` (an atomic command). The response is `{ \"ok\": true, \"value\": ..., \"invocation\": \"...\" }` or `{ \"ok\": false, \"error\": { \"kind\", \"code\", \"message\", \"params\", \"issues\" } }`. Operations are internal and cannot be called.")
	w.blank()
	var callable []Item
	for _, ns := range m.Namespaces {
		for _, it := range ns.Items {
			if rpcKind(it) != "" {
				callable = append(callable, it)
			}
		}
	}
	if len(callable) > 0 {
		w.line("| Resource | Kind | Permission |")
		w.line("| --- | --- | --- |")
		for _, it := range callable {
			perm := ""
			if it.Permission != "" {
				perm = code(it.Permission)
			}
			w.line("| %s | %s | %s |", w.itemRef(it.ID), rpcKind(it), perm)
		}
		w.blank()
	}

	var endpoints []EndpointDoc
	for _, ns := range m.Namespaces {
		endpoints = append(endpoints, ns.Endpoints...)
	}
	w.line("## Package endpoints")
	w.blank()
	w.line("Packages publish HTTP endpoints under `/_ext/endpoints/<namespace><path>`. A public endpoint answers without a session and runs as the trigger owner assigned for the tenant; any other runs as the signed-in user. Endpoint bodies are not described by a schema: the package defines them.")
	w.blank()
	if len(endpoints) > 0 {
		w.line("| Method | URL | Access | Package |")
		w.line("| --- | --- | --- | --- |")
		for _, e := range endpoints {
			access := "signed-in user"
			if e.Public {
				access = "public"
			}
			w.line("| %s | %s | %s | %s |", cell(e.Method), code(e.URL), access, code(e.Package))
		}
		w.blank()
	}

	if len(manage) > 0 {
		w.line("## Management API")
		w.blank()
		w.line("The management API under `/_ext/manage` drives the lifecycle of packages: drafts, builds, versions, reviews, activation, bindings, workflows and the audit log. It is what the extension configurator and the MCP tools use; every call is authorized and audited by the same service. It is listed here because you hold `%s`.", PermRead)
		w.blank()
		w.line("| Method | Path | Permission | Summary |")
		w.line("| --- | --- | --- | --- |")
		for _, r := range manage {
			perm := ""
			if r.Permission != "" {
				perm = code(r.Permission)
			}
			w.line("| %s | %s | %s | %s |", cell(r.Method), code(r.Path), perm, cell(r.Summary))
		}
		w.blank()
	}
	return []byte(w.String())
}

// rpcKind is the `kind` a resource is called with on /_ext/rpc, "" when the
// browser cannot call it.
func rpcKind(it Item) string {
	switch {
	case it.Kind == manifest.ResQuery:
		return "query"
	case it.Kind == manifest.ResCommand && it.Atomic:
		return "atomic"
	case it.Kind == manifest.ResCommand:
		return "command"
	}
	return ""
}
