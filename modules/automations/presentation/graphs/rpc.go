package graphs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// Limits of the RPC.
const (
	maxListLimit    = 50
	maxRunsLimit    = 50
	maxSubjectLimit = 20
)

// bind implements the declared method name with fn.
func bind[Req, Resp any](name string, fn func(context.Context, Req) (Resp, error)) solid.Binding {
	for _, m := range screen.Methods() {
		if m.Name == name {
			return solid.Bind(m, fn)
		}
	}
	panic(fmt.Sprintf("graphs: RPC method %q is not declared on the screen", name))
}

func (c *Controller) bindings() []solid.Binding {
	return []solid.Binding{
		bind(MethodList, c.list),
		bind(MethodGet, c.get),
		bind(MethodCatalog, c.catalog),
		bind(MethodCreate, c.create),
		bind(MethodSave, c.save),
		bind(MethodSaveLayout, c.saveLayout),
		bind(MethodValidate, c.validate),
		bind(MethodEnableReview, c.enableReview),
		bind(MethodEnable, c.enable),
		bind(MethodDisable, c.disable),
		bind(MethodDelete, c.delete),
		bind(MethodLive, c.live),
		bind(MethodRunManual, c.runManual),
		bind(MethodDryRun, c.dryRun),
		bind(MethodTestNode, c.testNode),
		bind(MethodSearchSubjects, c.searchSubjects),
		bind(MethodRuns, c.runs),
		bind(MethodRun, c.run),
		bind(MethodRunCancel, c.runCancel),
		bind(MethodRunResolve, c.runResolve),
		bind(MethodRunRetry, c.runRetry),
		bind(MethodRunRestart, c.runRestart),
	}
}

var (
	permRead    = permissions.AutomationsRead.Name()
	permAuthor  = permissions.AutomationsAuthor.Name()
	permEnable  = permissions.AutomationsEnable.Name()
	permOperate = permissions.AutomationsOperate.Name()
)

// actor is the engine actor of the session's user when it holds every one of perms: the tenant, the
// user and the origin come from the session.
func actor(ctx context.Context, perms ...string) (engine.Actor, error) {
	sub, err := listext.SubjectFromContext(ctx)
	if err != nil {
		return engine.Actor{}, solid.NewRPCError(solid.CodeUnauthenticated, "sign in required", nil)
	}
	for _, p := range perms {
		if !sub.Has(p) {
			return engine.Actor{}, forbidden("permission " + strings.Join(perms, " and ") + " is required")
		}
	}
	return engine.Actor{TenantID: sub.TenantID, UserID: sub.UserID, Origin: originFrom(ctx), Subject: sub}, nil
}

func canOf(sub spi.Subject, agent bool) Can {
	return Can{Author: sub.Has(permAuthor), Enable: sub.Has(permEnable) && !agent, Operate: sub.Has(permOperate)}
}

func forbidden(msg string) error {
	return solid.NewRPCError(solid.CodeForbidden, msg, map[string]string{"code": solid.CodeForbidden})
}

func notFound() error {
	return solid.NewRPCError(solid.CodeNotFound, "automation not found", map[string]string{"code": solid.CodeNotFound})
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func rpcError(code ErrorCode, msg string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	data["code"] = string(code)
	return solid.NewRPCError(string(code), msg, data)
}

// conflictMessage reports an invalid input that is a state that moved on: a call no longer blocked, a
// run still active.
func conflictMessage(err error) bool {
	msg := err.Error()
	for _, s := range []string{"no longer", "not blocked", "attempt changed", "does not restart"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// mapErr turns engine errors into RPC errors the screen explains; anything else is internal.
func mapErr(err error) error {
	var verr *engine.ValidationError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, engine.ErrNotFound):
		return notFound()
	case errors.Is(err, engine.ErrForbidden):
		return forbidden(strings.TrimPrefix(err.Error(), engine.ErrForbidden.Error()+": "))
	case errors.Is(err, engine.ErrHumanRequired):
		return rpcError(ErrHumanRequired, "only a person enables an automation", nil)
	case errors.Is(err, engine.ErrNotEnabled):
		return rpcError(ErrNotEnabled, "the automation is not enabled", nil)
	case errors.Is(err, engine.ErrLimitExceeded):
		return solid.NewRPCError("limit_exceeded", "a limit of the engine is exceeded", map[string]string{"code": "limit_exceeded"})
	case errors.As(err, &verr):
		return rpcError(ErrInvalidInput, "the graph does not validate", map[string]any{"errors": graphErrors(verr.Errors)})
	case errors.Is(err, engine.ErrConnectionUnavailable):
		return rpcError(ErrConnectionGone, "the connection is unavailable", nil)
	case errors.Is(err, engine.ErrRunFinished):
		return rpcError(ErrRunFinished, "the run is finished", nil)
	case errors.Is(err, engine.ErrInvalidInput) && conflictMessage(err):
		return solid.NewRPCError("conflict", strings.TrimPrefix(err.Error(), engine.ErrInvalidInput.Error()+": "), map[string]string{"code": "conflict"})
	case errors.Is(err, engine.ErrInvalidInput):
		return rpcError(ErrInvalidInput, strings.TrimPrefix(err.Error(), engine.ErrInvalidInput.Error()+": "), nil)
	}
	return err
}

func graphErrors(list []graph.GraphError) []GraphError {
	out := make([]GraphError, 0, len(list))
	for _, e := range list {
		out = append(out, GraphError{Path: e.Path, Message: e.Message, Code: e.Code})
	}
	return out
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (c *Controller) names(ctx context.Context, tenant uuid.UUID, ids ...int64) map[int64]string {
	uniq := map[int64]bool{}
	list := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !uniq[id] {
			uniq[id] = true
			list = append(list, id)
		}
	}
	if len(list) == 0 {
		return map[int64]string{}
	}
	got, err := c.cfg.Users.Names(ctx, tenant, list)
	if err != nil {
		c.log.WarnContext(ctx, "automations: user names are unavailable", "error", err)
		return map[int64]string{}
	}
	return got
}

// nameOf is a user's name, or "#<id>" when it is unknown.
func nameOf(names map[int64]string, id int64) string {
	if n := strings.TrimSpace(names[id]); n != "" {
		return n
	}
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("#%d", id)
}

// authorView is the catalog the actor sees as an author, in the user's language.
func (c *Controller) authorView(ctx context.Context, a engine.Actor) (catalog.View, error) {
	return c.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject), catalog.WithLocale(listext.LangFromContext(ctx)))
}

// triggerTitles are the titles of events by key, as the canvas names them.
func triggerTitles(ctx context.Context, view catalog.View) map[string]string {
	out := map[string]string{}
	for key, e := range view.Catalog.Events {
		title := e.Title
		if title == "" {
			title = key
		}
		out[key] = title
	}
	out[engine.EventManual] = intl.TWithDefault(ctx, "Automations.Triggers.Manual", "Manual start")
	out[engine.EventSchedule] = intl.TWithDefault(ctx, "Automations.Triggers.Schedule", "Schedule")
	return out
}

func triggerTitle(titles map[string]string, t engine.TriggerReview) string {
	if t.Kind == engine.KindSchedule && t.Schedule != "" {
		return titles[engine.EventSchedule] + " · " + t.Schedule
	}
	if s, ok := titles[t.Event]; ok {
		return s
	}
	return t.Event
}

func triggerKind(k string) TriggerKind {
	switch TriggerKind(k) {
	case TriggerSchedule, TriggerManual, TriggerInbound:
		return TriggerKind(k)
	case TriggerEvent:
	}
	return TriggerEvent
}

func (c *Controller) list(ctx context.Context, req ListRequest) (ListResponse, error) {
	a, err := actor(ctx, permRead)
	if err != nil {
		return ListResponse{}, err
	}
	limit := req.Limit
	if limit <= 0 || limit > maxListLimit {
		limit = maxListLimit
	}
	f := engine.ListFilter{Query: req.Query, Cursor: req.Cursor, Limit: limit}
	switch req.Status {
	case FilterEnabled:
		on := true
		f.Enabled = &on
	case FilterDisabled:
		off := false
		f.Enabled = &off
	case FilterAttention:
		f.Attention = true
	}
	page, err := c.cfg.Engine.Service.List(ctx, a, f)
	if err != nil {
		return ListResponse{}, mapErr(err)
	}
	ids := make([]uuid.UUID, 0, len(page.Items))
	for _, am := range page.Items {
		ids = append(ids, am.ID)
	}
	overviews, err := c.cfg.Engine.Service.Overviews(ctx, a, ids)
	if err != nil {
		return ListResponse{}, mapErr(err)
	}
	userIDs := []int64{}
	for _, ov := range overviews {
		userIDs = append(userIDs, ov.LatestSavedBy)
	}
	names := c.names(ctx, a.TenantID, userIDs...)
	view, err := c.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.WithLocale(listext.LangFromContext(ctx)))
	if err != nil {
		return ListResponse{}, err
	}
	titles := triggerTitles(ctx, view)
	out := ListResponse{Items: make([]ListItem, 0, len(page.Items)), Can: canOf(a.Subject, a.Origin != engine.OriginHuman)}
	for _, am := range page.Items {
		ov := overviews[am.ID]
		item := ListItem{
			ID: am.ID.String(), Name: am.Name, Enabled: am.EnabledVersion != nil, EnabledVersion: am.EnabledVersion,
			LatestVersion: am.LatestVersion, Triggers: []TriggerView{}, Attention: ov.Attention,
			UpdatedAt: stamp(am.UpdatedAt), UpdatedBy: nameOf(names, ov.LatestSavedBy),
			SavedByAgent: ov.LatestSavedOrigin == engine.OriginAgent,
		}
		if d := strings.TrimSpace(ov.LatestDescription); d != "" {
			item.Description = &d
		}
		item.PendingVersion = am.EnabledVersion != nil && *am.EnabledVersion != am.LatestVersion ||
			am.EnabledVersion == nil && ov.LatestSavedOrigin == engine.OriginAgent
		for _, t := range ov.Triggers {
			item.Triggers = append(item.Triggers, TriggerView{Kind: triggerKind(t.Kind), Title: triggerTitle(titles, t)})
		}
		if ov.LastRun != nil {
			at := ov.LastRun.UpdatedAt
			if at.IsZero() {
				at = ov.LastRun.CreatedAt
			}
			item.LastRun = &LastRun{RunID: ov.LastRun.RunID.String(), Status: RunStatus(ov.LastRun.Status), At: stamp(at)}
		}
		out.Items = append(out.Items, item)
	}
	if page.Next != "" {
		next := page.Next
		out.NextCursor = &next
	}
	return out, nil
}

func layoutView(l graph.Layout) map[string]XY {
	out := make(map[string]XY, len(l))
	for k, xy := range l {
		out[k] = XY{X: xy.X, Y: xy.Y}
	}
	return out
}

func layoutOf(m map[string]XY) graph.Layout {
	out := make(graph.Layout, len(m))
	for k, xy := range m {
		out[k] = graph.XY{X: xy.X, Y: xy.Y}
	}
	return out
}

func (c *Controller) get(ctx context.Context, req GetRequest) (GetResponse, error) {
	a, err := actor(ctx, permRead)
	if err != nil {
		return GetResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return GetResponse{}, err
	}
	am, err := c.cfg.Engine.Service.Get(ctx, a, id)
	if err != nil {
		return GetResponse{}, mapErr(err)
	}
	version := am.LatestVersion
	if req.Version > 0 {
		version = req.Version
	}
	v, err := c.cfg.Engine.Service.GetVersion(ctx, a, id, version)
	if err != nil {
		return GetResponse{}, mapErr(err)
	}
	versions, err := c.cfg.Engine.Service.ListVersions(ctx, a, id)
	if err != nil {
		return GetResponse{}, mapErr(err)
	}
	view, err := c.authorView(ctx, a)
	if err != nil {
		return GetResponse{}, err
	}
	ids := []int64{v.SavedBy}
	if am.OwnerUserID != nil {
		ids = append(ids, *am.OwnerUserID)
	}
	for _, vi := range versions {
		ids = append(ids, vi.SavedBy)
	}
	names := c.names(ctx, a.TenantID, ids...)
	agent := a.Origin != engine.OriginHuman
	out := GetResponse{
		Automation: AutomationView{
			ID: am.ID.String(), Name: am.Name, Enabled: am.EnabledVersion != nil, EnabledVersion: am.EnabledVersion,
			LatestVersion: am.LatestVersion, Layout: layoutView(am.Layout), LayoutRevision: am.LayoutRevision,
		},
		Graph: v.Graph, Version: v.Version,
		SavedBy:     SavedBy{Name: nameOf(names, v.SavedBy), Origin: Origin(v.SavedOrigin)},
		Versions:    make([]VersionEntry, 0, len(versions)),
		Catalog:     editorCatalog(ctx, view, c.cfg.Behaviors, c.cfg.Catalog),
		Connections: connectionOptions(view),
		Can:         canOf(a.Subject, agent),
	}
	if am.OwnerUserID != nil {
		out.Automation.Owner = &UserRef{UserID: *am.OwnerUserID, Name: nameOf(names, *am.OwnerUserID)}
	}
	out.ManualTriggers = []ManualTrigger{}
	if am.EnabledVersion != nil {
		// A manual start runs the enabled version: its triggers and input schemas, whatever version is open.
		enabled := v
		if *am.EnabledVersion != v.Version {
			if enabled, err = c.cfg.Engine.Service.GetVersion(ctx, a, id, *am.EnabledVersion); err != nil {
				return GetResponse{}, mapErr(err)
			}
		}
		out.ManualTriggers = manualTriggersOf(enabled.Graph, triggerTitles(ctx, view)[engine.EventManual])
	}
	for _, vi := range versions {
		out.Versions = append(out.Versions, VersionEntry{
			Version: vi.Version, Note: vi.Note, Enabled: vi.Enabled, By: nameOf(names, vi.SavedBy), Origin: Origin(vi.SavedOrigin), At: stamp(vi.CreatedAt),
		})
	}
	switch {
	case !out.Can.Author:
		out.ReadOnly = &ReadOnly{Code: "no_author", Reason: intl.TWithDefault(ctx, "Automations.Editor.ReadOnly.NoAuthor", "You may not change automations")}
	case v.Version != am.LatestVersion:
		out.ReadOnly = &ReadOnly{Code: "old_version", Reason: intl.TWithDefault(ctx, "Automations.Editor.ReadOnly.OldVersion", "An old version is open (read only)")}
	}
	return out, nil
}

func (c *Controller) catalog(ctx context.Context, _ CatalogRequest) (CatalogResponse, error) {
	a, err := actor(ctx, permRead, permAuthor)
	if err != nil {
		return CatalogResponse{}, err
	}
	view, err := c.authorView(ctx, a)
	if err != nil {
		return CatalogResponse{}, err
	}
	return CatalogResponse{Catalog: editorCatalog(ctx, view, c.cfg.Behaviors, c.cfg.Catalog), Connections: connectionOptions(view), Can: canOf(a.Subject, a.Origin != engine.OriginHuman)}, nil
}

// saveResult turns a refusal of create or save into the editor's SaveResult; other errors are RPC
// errors.
func (c *Controller) saveResult(ctx context.Context, a engine.Actor, err error) (SaveResult, error) {
	var (
		verr     *engine.ValidationError
		conflict *engine.ConflictError
	)
	switch {
	case errors.As(err, &verr):
		return SaveResult{Errors: graphErrors(verr.Errors), Warnings: graphErrors(verr.Warnings)}, nil
	case errors.As(err, &conflict):
		names := c.names(ctx, a.TenantID, conflict.By)
		return SaveResult{Conflict: &Conflict{By: nameOf(names, conflict.By), Version: conflict.Latest}}, nil
	}
	return SaveResult{}, mapErr(err)
}

func (c *Controller) create(ctx context.Context, req CreateRequest) (SaveResult, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return SaveResult{}, err
	}
	am, v, err := c.cfg.Engine.Service.Create(ctx, a, engine.CreateInput{Graph: req.Graph, Layout: layoutOf(req.Layout)})
	if err != nil {
		return c.saveResult(ctx, a, err)
	}
	return SaveResult{OK: true, ID: am.ID.String(), Graph: v.Graph, Version: v.Version, LayoutRevision: am.LayoutRevision}, nil
}

func (c *Controller) save(ctx context.Context, req SaveRequest) (SaveResult, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return SaveResult{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return SaveResult{}, err
	}
	var layout graph.Layout
	if req.Layout != nil {
		layout = layoutOf(req.Layout)
	}
	v, err := c.cfg.Engine.Service.SaveVersion(ctx, a, id, engine.SaveInput{BaseVersion: req.Base, Graph: req.Graph, Layout: layout, LayoutBase: req.LayoutRevision, Note: req.Note})
	if err != nil {
		return c.saveResult(ctx, a, err)
	}
	out := SaveResult{OK: true, Graph: v.Graph, Version: v.Version}
	am, err := c.cfg.Engine.Service.Get(ctx, a, id)
	if err != nil {
		return SaveResult{}, mapErr(err)
	}
	out.LayoutRevision = am.LayoutRevision
	if req.LayoutRevision != nil && am.LayoutRevision != *req.LayoutRevision+1 {
		// Another tab moved steps since: its positions were kept; the editor shows them on request.
		out.LayoutChanged = true
		out.Positions = layoutView(am.Layout)
	}
	// Saved but not running: the graph runs another version until a person reviews this one.
	out.PendingReview = am.EnabledVersion != nil && *am.EnabledVersion != v.Version
	return out, nil
}

func (c *Controller) saveLayout(ctx context.Context, req SaveLayoutRequest) (SaveLayoutResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return SaveLayoutResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return SaveLayoutResponse{}, err
	}
	rev, err := c.cfg.Engine.Service.SaveLayoutAt(ctx, a, id, req.BaseRevision, layoutOf(req.Positions))
	var conflict *engine.LayoutConflictError
	switch {
	case err == nil:
		return SaveLayoutResponse{OK: true, Revision: rev}, nil
	case errors.As(err, &conflict):
		return SaveLayoutResponse{Error: ErrVersionConflict, Revision: conflict.Revision, Positions: layoutView(conflict.Layout)}, nil
	}
	return SaveLayoutResponse{}, mapErr(err)
}

func (c *Controller) validate(ctx context.Context, req ValidateRequest) (ValidateResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return ValidateResponse{}, err
	}
	err = c.cfg.Engine.Authoring.Validate(ctx, a, req.Graph)
	var verr *engine.ValidationError
	switch {
	case err == nil:
		return ValidateResponse{OK: true, Errors: []GraphError{}, Warnings: []GraphError{}}, nil
	case errors.As(err, &verr):
		return ValidateResponse{Errors: graphErrors(verr.Errors), Warnings: graphErrors(verr.Warnings)}, nil
	}
	return ValidateResponse{}, mapErr(err)
}

func (c *Controller) enableReview(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	a, err := actor(ctx, permEnable)
	if err != nil {
		return ReviewResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return ReviewResponse{}, err
	}
	version := req.Version
	if version <= 0 {
		// The latest version: what a person enables from the list or the toggle.
		am, err := c.cfg.Engine.Service.Get(ctx, readAs(a), id)
		if err != nil {
			return ReviewResponse{}, mapErr(err)
		}
		version = am.LatestVersion
	}
	rv, err := c.cfg.Engine.Service.EnableReview(ctx, a, id, version)
	if err != nil {
		return ReviewResponse{}, mapErr(err)
	}
	live, err := c.cfg.Engine.Service.CountLive(ctx, a, id)
	if err != nil {
		return ReviewResponse{}, mapErr(err)
	}
	view, err := c.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.WithLocale(listext.LangFromContext(ctx)))
	if err != nil {
		return ReviewResponse{}, err
	}
	titles := triggerTitles(ctx, view)
	out := ReviewResponse{
		Version: rv.Version, CodeNodes: make([]CodeNodeView, 0, len(rv.CodeNodes)), Triggers: make([]ReviewTrigger, 0, len(rv.Triggers)),
		RunAs: RunAsOwner, CodeDigest: rv.CodeDigest, Live: LiveView{Runs: live.Runs, Waits: live.Waits}, Problems: graphErrors(rv.Problems),
	}
	if rv.RunAs == string(RunAsInitiator) {
		out.RunAs = RunAsInitiator
	}
	for _, n := range rv.CodeNodes {
		lang := LangTS
		if n.Lang == string(LangJS) {
			lang = LangJS
		}
		secrets := n.Secrets
		if secrets == nil {
			secrets = []string{}
		}
		out.CodeNodes = append(out.CodeNodes, CodeNodeView{NodeID: n.NodeID, Title: n.NodeID, Lang: lang, Source: n.Source, Secrets: secrets, Fetch: n.Fetch})
	}
	for _, t := range rv.Triggers {
		tr := ReviewTrigger{TriggerID: t.TriggerID, Kind: triggerKind(t.Kind), Title: triggerTitle(titles, t)}
		if t.Event != "" && t.Kind != engine.KindSchedule && t.Kind != engine.KindManual {
			ev := t.Event
			tr.Event = &ev
		}
		if t.Schedule != "" {
			s := t.Schedule
			tr.Schedule = &s
		}
		out.Triggers = append(out.Triggers, tr)
	}
	return out, nil
}

func (c *Controller) enable(ctx context.Context, req EnableRequest) (EnableResponse, error) {
	a, err := actor(ctx, permEnable)
	if err != nil {
		return EnableResponse{Error: ErrForbidden}, nil //nolint:nilerr // the refusal is a result of the contract
	}
	id, err := parseID(req.ID)
	if err != nil {
		return EnableResponse{}, err
	}
	// A person enables the latest version they reviewed: a version saved after the review (other code)
	// is reviewed again, never skipped by enabling an older one. The engine checks it under the graph's
	// lock, together with the enable.
	am, err := c.cfg.Engine.Service.Enable(ctx, a, id, engine.EnableInput{Version: req.Version, CodeDigest: req.CodeDigest, Latest: true})
	var verr *engine.ValidationError
	switch {
	case err == nil:
		ev := 0
		if am.EnabledVersion != nil {
			ev = *am.EnabledVersion
		}
		return EnableResponse{OK: true, EnabledVersion: ev}, nil
	case errors.Is(err, engine.ErrReviewRequired):
		return EnableResponse{Error: ErrReviewRequired}, nil
	case errors.Is(err, engine.ErrHumanRequired):
		return EnableResponse{Error: ErrHumanRequired}, nil
	case errors.As(err, &verr):
		return EnableResponse{Error: ErrValidationFailed, Errors: graphErrors(verr.Errors)}, nil
	case errors.Is(err, engine.ErrInvalidInput):
		return EnableResponse{Error: ErrValidationFailed, Errors: []GraphError{{Code: string(ErrConnectionGone), Message: strings.TrimPrefix(err.Error(), engine.ErrInvalidInput.Error()+": ")}}}, nil
	case errors.Is(err, engine.ErrForbidden):
		return EnableResponse{Error: ErrForbidden}, nil
	}
	return EnableResponse{}, mapErr(err)
}

// readAs is the actor reading the graph a review or an enable is about: a person who may enable
// automations reads the graph they enable, with or without Automations.Read.
func readAs(a engine.Actor) engine.Actor {
	if a.Subject.Has(permRead) {
		return a
	}
	s := a.Subject
	s.Permissions = append(append([]string(nil), s.Permissions...), permRead)
	a.Subject = s
	return a
}

func (c *Controller) disable(ctx context.Context, req IDRequest) (DisableResponse, error) {
	a, err := actor(ctx, permEnable)
	if err != nil {
		return DisableResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return DisableResponse{}, err
	}
	live, err := c.cfg.Engine.Service.CountLive(ctx, a, id)
	if err != nil {
		return DisableResponse{}, mapErr(err)
	}
	if _, err := c.cfg.Engine.Service.Disable(ctx, a, id); err != nil {
		return DisableResponse{}, mapErr(err)
	}
	return DisableResponse{OK: true, CancelledRuns: live.Runs}, nil
}

func (c *Controller) delete(ctx context.Context, req IDRequest) (OKResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := c.cfg.Engine.Service.Delete(ctx, a, id); err != nil {
		return OKResponse{}, mapErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) live(ctx context.Context, req IDRequest) (LiveView, error) {
	a, err := actor(ctx, permRead)
	if err != nil {
		return LiveView{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return LiveView{}, err
	}
	live, err := c.cfg.Engine.Service.CountLive(ctx, a, id)
	if err != nil {
		return LiveView{}, mapErr(err)
	}
	return LiveView{Runs: live.Runs, Waits: live.Waits}, nil
}

func subjectRef(s *Subject) *engine.SubjectRef {
	if s == nil || strings.TrimSpace(s.Entity) == "" {
		return nil
	}
	return &engine.SubjectRef{Entity: s.Entity, Key: s.Key}
}

func (c *Controller) runManual(ctx context.Context, req RunManualRequest) (RunManualResponse, error) {
	a, err := actor(ctx, permOperate)
	if err != nil {
		return RunManualResponse{}, err
	}
	id, err := parseID(req.ID)
	if err != nil {
		return RunManualResponse{}, err
	}
	input := req.Input
	if len(input) == 0 || string(input) == "null" {
		input = json.RawMessage(`{}`)
	}
	ref, err := c.cfg.Engine.Service.StartManual(ctx, a, id, engine.ManualStart{TriggerID: req.TriggerID, Input: input, Subject: subjectRef(req.Subject)})
	switch {
	case err == nil:
		return RunManualResponse{OK: true, RunID: ref.RunID.String()}, nil
	case errors.Is(err, engine.ErrNotEnabled):
		return RunManualResponse{Error: ErrNotEnabled}, nil
	case errors.Is(err, engine.ErrInvalidInput):
		return RunManualResponse{Error: ErrInvalidInput, Fields: map[string]string{"input": strings.TrimPrefix(err.Error(), engine.ErrInvalidInput.Error()+": ")}}, nil
	}
	return RunManualResponse{}, mapErr(err)
}

func (c *Controller) dryRun(ctx context.Context, req DryRunRequest) (DryRunResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return DryRunResponse{}, err
	}
	if len(req.Events) > engine.MaxDryRunEvents {
		return DryRunResponse{}, solid.NewRPCError("limit_exceeded", fmt.Sprintf("at most %d events", engine.MaxDryRunEvents), map[string]string{"code": "limit_exceeded"})
	}
	subject := engine.SubjectRef{Entity: graph.SubjectNone}
	if s := subjectRef(&req.Subject); s != nil {
		subject = *s
	}
	switch req.Decisions {
	case "", DecisionsScripted, DecisionsCached, DecisionsLive:
	default:
		return DryRunResponse{}, rpcError(ErrInvalidInput, "decisions is scripted, cached or live", nil)
	}
	res, err := c.cfg.Engine.Authoring.DryRun(ctx, a, engine.DryRunInput{Graph: req.Graph, Subject: subject, Context: req.Context, Events: req.Events, Decisions: string(req.Decisions)})
	if err != nil {
		return DryRunResponse{}, mapErr(err)
	}
	out := DryRunResponse{Trace: make([]TraceStep, 0, len(res.Trace)), LiveCalls: res.LiveCalls}
	for _, s := range res.Trace {
		out.Trace = append(out.Trace, TraceStep{Node: s.Node, Token: s.Token, Kind: s.Kind, Outcome: s.Outcome, Path: s.Path, Message: s.Message, Status: s.Status, Effects: s.Effects, Output: s.Output})
	}
	return out, nil
}

func (c *Controller) testNode(ctx context.Context, req TestNodeRequest) (TestNodeResponse, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return TestNodeResponse{}, err
	}
	res, err := c.cfg.Engine.Authoring.TestNode(ctx, a, engine.TestNodeInput{Graph: req.Graph, NodeID: req.NodeID, Subject: subjectRef(req.Subject), Context: req.Context, Key: req.IdempotencyKey})
	if err != nil {
		return TestNodeResponse{}, mapErr(err)
	}
	out := TestNodeResponse{Outcome: res.Outcome, Output: res.Output, Logs: res.Logs, DurationMS: res.DurationMS}
	if res.Error != nil {
		out.Error = &TestNodeError{Reason: res.Error.Code, Message: res.Error.Message}
	}
	return out, nil
}

func (c *Controller) searchSubjects(ctx context.Context, req SearchSubjectsRequest) ([]SubjectOption, error) {
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > maxSubjectLimit {
		limit = maxSubjectLimit
	}
	list, err := c.cfg.Engine.Authoring.SearchSubjects(ctx, a, req.Entity, req.Query, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]SubjectOption, 0, len(list))
	for _, s := range list {
		out = append(out, SubjectOption{Subject: Subject{Entity: s.Subject.Entity, Key: s.Subject.Key, Label: s.Label}, Label: s.Label, Note: s.Note})
	}
	return out, nil
}

// stateOf is the editor's state of a run status: queued waits, a filter that failed is a failure.
func stateOf(s engine.RunStatus) RunState {
	switch s {
	case engine.RunQueued, engine.RunWaiting:
		return StateWaiting
	case engine.RunRunning:
		return StateRunning
	case engine.RunBlocked:
		return StateBlocked
	case engine.RunDone:
		return StateDone
	case engine.RunFailed, engine.RunFilterFailed:
		return StateFailed
	case engine.RunCancelled:
		return StateCancelled
	}
	return StateRunning
}

// statusesOf are the engine statuses of editor states.
func statusesOf(states []RunState) []engine.RunStatus {
	seen := map[engine.RunStatus]bool{}
	var out []engine.RunStatus
	add := func(s ...engine.RunStatus) {
		for _, x := range s {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	for _, st := range states {
		switch st {
		case StateWaiting:
			add(engine.RunQueued, engine.RunWaiting)
		case StateRunning:
			add(engine.RunRunning)
		case StateBlocked:
			add(engine.RunBlocked)
		case StateDone:
			add(engine.RunDone)
		case StateFailed:
			add(engine.RunFailed, engine.RunFilterFailed)
		case StateCancelled:
			add(engine.RunCancelled)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func stepError(e *engine.StepError) *StepError {
	if e == nil {
		return nil
	}
	return &StepError{Code: e.Code, Message: e.Message, Path: e.Path, Retryable: e.Retryable}
}

func runSummary(s engine.RunSummary) RunSummary {
	out := RunSummary{
		RunID: s.RunID.String(), AutomationID: s.AutomationID.String(), Version: s.Version, State: stateOf(s.Status), Status: RunStatus(s.Status),
		Reason: s.Reason, TriggerKind: s.TriggerKind, TriggerID: s.TriggerID, Subject: Subject{Entity: s.Subject.Entity, Key: s.Subject.Key},
		CurrentNodes: append([]string{}, s.CurrentNodes...), Error: stepError(s.Error), CreatedAt: stamp(s.CreatedAt), UpdatedAt: stamp(s.UpdatedAt),
	}
	if s.Status == engine.RunFilterFailed && out.Error == nil {
		out.Error = &StepError{Code: string(RunFilterFailed), Message: s.Reason}
	} else if s.Status == engine.RunFilterFailed {
		out.Error.Code = string(RunFilterFailed)
	}
	return out
}

func (c *Controller) runs(ctx context.Context, req RunsRequest) (RunsResponse, error) {
	a, err := actor(ctx, permRead)
	if err != nil {
		return RunsResponse{}, err
	}
	id, err := parseID(req.AutomationID)
	if err != nil {
		return RunsResponse{}, err
	}
	limit := req.Limit
	if limit <= 0 || limit > maxRunsLimit {
		limit = maxRunsLimit
	}
	// A graph that is deleted or another tenant's is not found, not an empty page.
	if _, err := c.cfg.Engine.Service.Get(ctx, a, id); err != nil {
		return RunsResponse{}, mapErr(err)
	}
	page, err := c.cfg.Engine.Service.ListRuns(ctx, a, engine.RunFilter{AutomationID: &id, Version: req.Version, Status: statusesOf(req.State), SubjectKeyPrefix: strings.TrimSpace(req.Subject), Cursor: req.Cursor, Limit: limit})
	if err != nil {
		return RunsResponse{}, mapErr(err)
	}
	out := RunsResponse{Items: make([]RunSummary, 0, len(page.Items))}
	for _, s := range page.Items {
		out.Items = append(out.Items, runSummary(s))
	}
	if page.Next != "" {
		next := page.Next
		out.NextCursor = &next
	}
	return out, nil
}

func parseRun(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, solid.NewRPCError(solid.CodeNotFound, "run not found", map[string]string{"code": solid.CodeNotFound})
	}
	return id, nil
}

func optStamp(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := stamp(*t)
	return &s
}

func (c *Controller) run(ctx context.Context, req RunRequest) (RunData, error) {
	a, err := actor(ctx, permRead)
	if err != nil {
		return RunData{}, err
	}
	id, err := parseRun(req.RunID)
	if err != nil {
		return RunData{}, err
	}
	d, err := c.cfg.Engine.Service.GetRun(ctx, a, id)
	if err != nil {
		return RunData{}, mapErr(err)
	}
	out := RunData{Run: runSummary(d.RunSummary), Event: d.Event, Tokens: []TokenView{}, Steps: []StepView{}, Waits: []WaitView{}}
	if c.cfg.Links != nil && d.Subject.Entity != "" && d.Subject.Entity != graph.SubjectNone {
		if link, err := c.cfg.Links.SubjectURL(ctx, a.TenantID, d.Subject); err == nil {
			out.Run.SubjectURL = link
		} else {
			c.log.DebugContext(ctx, "automations: no link for the run's subject", "entity", d.Subject.Entity, "error", err)
		}
	}
	for _, t := range d.Tokens {
		tv := TokenView{ID: t.ID.String(), NodeID: t.NodeID, Status: t.Status, Branch: t.Branch, ForeachIndex: t.ForeachIndex}
		if t.Parent != nil {
			p := t.Parent.String()
			tv.Parent = &p
		}
		out.Tokens = append(out.Tokens, tv)
	}
	for _, s := range d.Steps {
		out.Steps = append(out.Steps, StepView{
			TokenID: s.TokenID.String(), Seq: s.Seq, NodeID: s.NodeID, Attempt: s.Attempt, Input: s.Input, Output: s.Output, Outcome: s.Outcome,
			Error: stepError(s.Error), StartedAt: stamp(s.StartedAt), EndedAt: optStamp(s.EndedAt), Warnings: s.Warnings, Logs: s.Logs, Hidden: s.Hidden,
		})
	}
	for _, w := range d.Waits {
		out.Waits = append(out.Waits, WaitView{TokenID: w.TokenID.String(), NodeID: w.NodeID, WaitID: w.WaitID, Kind: w.Kind, EventKind: w.EventKind, TimeoutAt: optStamp(w.TimeoutAt), Status: w.Status})
	}
	return out, nil
}

func (c *Controller) runCancel(ctx context.Context, req RunRequest) (RunCancelResponse, error) {
	a, err := actor(ctx, permOperate)
	if err != nil {
		return RunCancelResponse{}, err
	}
	id, err := parseRun(req.RunID)
	if err != nil {
		return RunCancelResponse{}, err
	}
	err = c.cfg.Engine.Service.Cancel(ctx, a, id)
	switch {
	case err == nil:
		return RunCancelResponse{OK: true}, nil
	case errors.Is(err, engine.ErrRunFinished):
		return RunCancelResponse{Error: ErrRunFinished}, nil
	}
	return RunCancelResponse{}, mapErr(err)
}

func (c *Controller) runResolve(ctx context.Context, req RunResolveRequest) (OKResponse, error) {
	a, err := actor(ctx, permOperate)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseRun(req.RunID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := c.cfg.Engine.Operations.Resolve(ctx, a, id, req.WaitID, req.Outcome, req.Output); err != nil {
		return OKResponse{}, mapErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) runRetry(ctx context.Context, req RunRetryRequest) (OKResponse, error) {
	a, err := actor(ctx, permOperate)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseRun(req.RunID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := c.cfg.Engine.Operations.Retry(ctx, a, id, req.WaitID); err != nil {
		return OKResponse{}, mapErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) runRestart(ctx context.Context, req RunRestartRequest) (RunRef, error) {
	a, err := actor(ctx, permOperate)
	if err != nil {
		return RunRef{}, err
	}
	id, err := parseRun(req.RunID)
	if err != nil {
		return RunRef{}, err
	}
	ref, err := c.cfg.Engine.Service.Restart(ctx, a, id, req.Version)
	if err != nil {
		return RunRef{}, mapErr(err)
	}
	return RunRef{RunID: ref.RunID.String()}, nil
}

// manualTriggersOf are the manual triggers of a version with their input schemas.
func manualTriggersOf(raw json.RawMessage, title string) []ManualTrigger {
	g, errs := graph.Decode(raw)
	if len(errs) > 0 {
		return []ManualTrigger{}
	}
	out := []ManualTrigger{}
	for _, t := range g.Triggers {
		if t.Event != engine.EventManual {
			continue
		}
		mt := ManualTrigger{TriggerID: t.ID, Title: title}
		if schema, ok := t.Params["input_schema"].(map[string]any); ok {
			mt.InputSchema = schema
		}
		out = append(out, mt)
	}
	return out
}
