package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opService = serrors.Op("automation.engine.Service")

type service struct{ r *Runtime }

var _ Service = (*service)(nil)

func (s *service) authorize(a Actor, perms ...string) error {
	if a.TenantID == uuid.Nil || a.UserID <= 0 || !a.Origin.valid() {
		return ErrForbidden
	}
	for _, p := range perms {
		if !a.Subject.Has(p) {
			return ErrForbidden
		}
	}
	return nil
}

func (s *service) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.r.cfg.DB, fn)
}

func layoutJSON(l graph.Layout) ([]byte, error) {
	if l == nil {
		l = graph.Layout{}
	}
	return json.Marshal(l)
}

func (s *service) Create(ctx context.Context, a Actor, in CreateInput) (Automation, Version, error) {
	if err := s.authorize(a, PermAuthor); err != nil {
		return Automation{}, Version{}, err
	}
	if errs := graph.ValidateLayout(in.Layout); len(errs) > 0 {
		return Automation{}, Version{}, &ValidationError{Errors: errs}
	}
	v, err := s.r.validate(ctx, a, in.Graph)
	if err != nil {
		return Automation{}, Version{}, err
	}
	id := uuid.New()
	layout, err := layoutJSON(graph.LayoutOf(v.g, in.Layout))
	if err != nil {
		return Automation{}, Version{}, err
	}
	digest := codeDigest(s.r.codeNodes(v.g))
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation.automations (tenant_id, id, name, latest_version, layout, created_by, created_origin)
			VALUES ($1, $2, $3, 1, $4, $5, $6)`, a.TenantID, id, v.g.Name, layout, a.UserID, string(a.Origin)); err != nil {
			return serrors.FromDB(opService, err)
		}
		return insertVersion(ctx, tx, a, id, 1, v, digest, in.Note)
	})
	if err != nil {
		return Automation{}, Version{}, err
	}
	am, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false)
	if err != nil {
		return Automation{}, Version{}, err
	}
	ver, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, 1)
	return am.Automation, ver.Version, err
}

func insertVersion(ctx context.Context, tx pgx.Tx, a Actor, id uuid.UUID, version int, v validated, digest, note string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO automation.versions (tenant_id, automation_id, version, graph, catalog_revision, code_digest, run_as, saved_by, saved_origin, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.TenantID, id, version, []byte(v.raw), v.view.Catalog.Revision, digest, string(v.g.RunAs), a.UserID, string(a.Origin), note)
	return serrors.FromDB(opService, err)
}

func (s *service) SaveVersion(ctx context.Context, a Actor, id uuid.UUID, in SaveInput) (Version, error) {
	if err := s.authorize(a, PermAuthor); err != nil {
		return Version{}, err
	}
	if in.Layout != nil {
		if errs := graph.ValidateLayout(in.Layout); len(errs) > 0 {
			return Version{}, &ValidationError{Errors: errs}
		}
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false); err != nil {
		return Version{}, err
	}
	v, err := s.r.validate(ctx, a, in.Graph)
	if err != nil {
		return Version{}, err
	}
	digest := codeDigest(s.r.codeNodes(v.g))
	var next int
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		am, err := loadAutomation(ctx, tx, a.TenantID, id, true)
		if err != nil {
			return err
		}
		if in.BaseVersion != am.LatestVersion {
			latest, err := loadVersion(ctx, tx, a.TenantID, id, am.LatestVersion)
			if err != nil {
				return err
			}
			return &ConflictError{Latest: am.LatestVersion, By: latest.SavedBy}
		}
		next = am.LatestVersion + 1
		if err := insertVersion(ctx, tx, a, id, next, v, digest, in.Note); err != nil {
			return err
		}
		merged := graph.Layout{}
		for k, xy := range am.Layout {
			merged[k] = xy
		}
		for k, xy := range in.Layout {
			merged[k] = xy
		}
		layout, err := layoutJSON(graph.LayoutOf(v.g, merged))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE automation.automations SET name = $3, latest_version = $4, layout = $5, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			a.TenantID, id, v.g.Name, next, layout); err != nil {
			return serrors.FromDB(opService, err)
		}
		// A human who may enable switches a running graph to the new version when neither its code nor
		// its run-as changed; an agent's version waits for a human's Enable.
		if am.EnabledVersion == nil || a.Origin != OriginHuman || !a.Subject.Has(PermEnable) {
			return nil
		}
		enabled, err := loadVersion(ctx, tx, a.TenantID, id, *am.EnabledVersion)
		if err != nil {
			return err
		}
		if enabled.CodeDigest != digest || enabled.RunAs != string(v.g.RunAs) || !triggersReadable(v.view, v.g) {
			return nil
		}
		return s.r.enableTx(ctx, tx, a, id, next, v.view, v.g)
	})
	if err != nil {
		return Version{}, err
	}
	ver, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, next)
	return ver.Version, err
}

// triggersReadable reports whether the author may read every event the graph's triggers are on.
func triggersReadable(view catalog.View, g graph.Graph) bool {
	for _, t := range g.Triggers {
		if view.Unavailable[t.Event] == catalog.ReasonPermission {
			return false
		}
	}
	return true
}

// enableTx pins a version: enabled_version, the owner (trigger_owners) and the version's triggers with
// the current outbox watermark, so events published before do not start runs.
func (r *Runtime) enableTx(ctx context.Context, tx pgx.Tx, a Actor, id uuid.UUID, version int, view catalog.View, g graph.Graph) error {
	if _, err := tx.Exec(ctx, `UPDATE automation.automations SET enabled_version = $3, owner_user_id = $4, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		a.TenantID, id, version, a.UserID); err != nil {
		return serrors.FromDB(opService, err)
	}
	if err := triggerowner.Assign(ctx, tx, ownerTrigger(id), a.TenantID, a.UserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM automation.triggers WHERE tenant_id = $1 AND automation_id = $2`, a.TenantID, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	after, err := events.Watermark(ctx, tx)
	if err != nil {
		return err
	}
	for _, t := range triggersOf(view, g) {
		var event, filter, schedule any
		if t.Kind == KindEvent || t.Kind == KindInbound {
			event = t.Event
		}
		if t.Filter != "" {
			filter = t.Filter
		}
		if t.Schedule != nil {
			b, err := json.Marshal(t.Schedule)
			if err != nil {
				return err
			}
			schedule = b
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation.triggers (tenant_id, automation_id, version, trigger_id, kind, event_kind, filter_cel, schedule, after_seq)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, a.TenantID, id, version, t.ID, t.Kind, event, filter, schedule, after); err != nil {
			return serrors.FromDB(opService, err)
		}
	}
	return nil
}

func (s *service) SaveLayout(ctx context.Context, a Actor, id uuid.UUID, l graph.Layout) error {
	if err := s.authorize(a, PermAuthor); err != nil {
		return err
	}
	if errs := graph.ValidateLayout(l); len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		am, err := loadAutomation(ctx, tx, a.TenantID, id, true)
		if err != nil {
			return err
		}
		v, err := loadVersion(ctx, tx, a.TenantID, id, am.LatestVersion)
		if err != nil {
			return err
		}
		g, errs := graph.Decode(v.Graph)
		if len(errs) > 0 {
			return fmt.Errorf("automation %s: version %d does not decode", id, am.LatestVersion)
		}
		merged := graph.Layout{}
		for k, xy := range am.Layout {
			merged[k] = xy
		}
		for k, xy := range l {
			merged[k] = xy
		}
		layout, err := layoutJSON(graph.LayoutOf(g, merged))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE automation.automations SET layout = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, a.TenantID, id, layout)
		return serrors.FromDB(opService, err)
	})
}

func (s *service) Get(ctx context.Context, a Actor, id uuid.UUID) (Automation, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return Automation{}, err
	}
	am, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false)
	return am.Automation, err
}

func (s *service) GetVersion(ctx context.Context, a Actor, id uuid.UUID, version int) (Version, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return Version{}, err
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false); err != nil {
		return Version{}, err
	}
	v, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, version)
	return v.Version, err
}

// cursor is the position after the last item of a page: its time and id.
type cursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}

func encodeCursor(at time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(cursor{At: at, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: cursor", ErrInvalidInput)
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%w: cursor", ErrInvalidInput)
	}
	return &c, nil
}

func pageLimit(n int) int {
	switch {
	case n <= 0:
		return 50
	case n > 100:
		return 100
	}
	return n
}

func (s *service) List(ctx context.Context, a Actor, f ListFilter) (Page[Automation], error) {
	if err := s.authorize(a, PermRead); err != nil {
		return Page[Automation]{}, err
	}
	c, err := decodeCursor(f.Cursor)
	if err != nil {
		return Page[Automation]{}, err
	}
	limit := pageLimit(f.Limit)
	args := []any{a.TenantID}
	where := []string{"tenant_id = $1", "deleted_at IS NULL"}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)+"%")
		where = append(where, fmt.Sprintf("name ILIKE $%d", len(args)))
	}
	if f.Enabled != nil {
		if *f.Enabled {
			where = append(where, "enabled_version IS NOT NULL")
		} else {
			where = append(where, "enabled_version IS NULL")
		}
	}
	if c != nil {
		args = append(args, c.At, c.ID)
		where = append(where, fmt.Sprintf("(updated_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	rows, err := s.r.cfg.DB.Query(ctx, `SELECT `+automationCols+` FROM automation.automations WHERE `+strings.Join(where, " AND ")+
		fmt.Sprintf(` ORDER BY updated_at DESC, id DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return Page[Automation]{}, serrors.FromDB(opService, err)
	}
	defer rows.Close()
	page := Page[Automation]{Items: []Automation{}}
	for rows.Next() {
		am, err := scanAutomation(rows)
		if err != nil {
			return Page[Automation]{}, serrors.FromDB(opService, err)
		}
		page.Items = append(page.Items, am.Automation)
	}
	if err := rows.Err(); err != nil {
		return Page[Automation]{}, serrors.FromDB(opService, err)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[limit-1]
		page.Next = encodeCursor(last.UpdatedAt, last.ID)
	}
	return page, nil
}

func (s *service) ListVersions(ctx context.Context, a Actor, id uuid.UUID) ([]VersionInfo, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return nil, err
	}
	am, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.r.cfg.DB.Query(ctx, `SELECT version, saved_by, saved_origin, note, created_at FROM automation.versions WHERE tenant_id = $1 AND automation_id = $2 ORDER BY version DESC`, a.TenantID, id)
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var vi VersionInfo
		var origin string
		if err := rows.Scan(&vi.Version, &vi.SavedBy, &origin, &vi.Note, &vi.CreatedAt); err != nil {
			return nil, serrors.FromDB(opService, err)
		}
		vi.SavedOrigin = Origin(origin)
		vi.Enabled = am.EnabledVersion != nil && *am.EnabledVersion == vi.Version
		out = append(out, vi)
	}
	return out, serrors.FromDB(opService, rows.Err())
}

func (s *service) EnableReview(ctx context.Context, a Actor, id uuid.UUID, version int) (EnableReview, error) {
	if err := s.authorize(a, PermEnable); err != nil {
		return EnableReview{}, err
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false); err != nil {
		return EnableReview{}, err
	}
	v, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, version)
	if err != nil {
		return EnableReview{}, err
	}
	g, errs := graph.Decode(v.Graph)
	if len(errs) > 0 {
		return EnableReview{}, &ValidationError{Errors: errs}
	}
	view, err := s.r.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return EnableReview{}, err
	}
	return reviewOf(s.r, view, v, g), nil
}

func (s *service) Enable(ctx context.Context, a Actor, id uuid.UUID, in EnableInput) (Automation, error) {
	if a.Origin != OriginHuman {
		return Automation{}, ErrHumanRequired
	}
	if err := s.authorize(a, PermEnable); err != nil {
		return Automation{}, err
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false); err != nil {
		return Automation{}, err
	}
	v, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, in.Version)
	if err != nil {
		return Automation{}, err
	}
	g, errs := graph.Decode(v.Graph)
	if len(errs) > 0 {
		return Automation{}, &ValidationError{Errors: errs}
	}
	if digest := codeDigest(s.r.codeNodes(g)); digest != "" && digest != in.CodeDigest {
		return Automation{}, ErrReviewRequired
	}
	view, err := s.r.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return Automation{}, err
	}
	if !triggersReadable(view, g) {
		return Automation{}, fmt.Errorf("%w: the triggers' events need a permission you do not have", ErrForbidden)
	}
	checked, err := s.r.validate(ctx, a, v.Graph)
	if err != nil {
		return Automation{}, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := loadAutomation(ctx, tx, a.TenantID, id, true); err != nil {
			return err
		}
		return s.r.enableTx(ctx, tx, a, id, in.Version, checked.view, checked.g)
	})
	if err != nil {
		return Automation{}, err
	}
	return s.Get(ctx, a, id)
}

func (s *service) Disable(ctx context.Context, a Actor, id uuid.UUID) (Automation, error) {
	if err := s.authorize(a, PermEnable); err != nil {
		return Automation{}, err
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := loadAutomation(ctx, tx, a.TenantID, id, true); err != nil {
			return err
		}
		return disableTx(ctx, tx, a.TenantID, id)
	})
	if err != nil {
		return Automation{}, err
	}
	if err := s.r.cancelRunsOf(ctx, a.TenantID, id); err != nil {
		return Automation{}, err
	}
	return s.Get(ctx, a, id)
}

func disableTx(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE automation.automations SET enabled_version = NULL, disabled_at = now(), updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenant, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	_, err := tx.Exec(ctx, `DELETE FROM automation.triggers WHERE tenant_id = $1 AND automation_id = $2`, tenant, id)
	return serrors.FromDB(opService, err)
}

func (s *service) Delete(ctx context.Context, a Actor, id uuid.UUID) error {
	if err := s.authorize(a, PermAuthor); err != nil {
		return err
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		am, err := loadAutomation(ctx, tx, a.TenantID, id, true)
		if err != nil {
			return err
		}
		if am.EnabledVersion != nil && !a.Subject.Has(PermEnable) {
			return ErrForbidden
		}
		if err := disableTx(ctx, tx, a.TenantID, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE automation.automations SET deleted_at = now() WHERE tenant_id = $1 AND id = $2`, a.TenantID, id)
		return serrors.FromDB(opService, err)
	})
	if err != nil {
		return err
	}
	return s.r.cancelRunsOf(ctx, a.TenantID, id)
}

// cancelRunsOf cancels the live runs of a graph: queued ones never start, the instances of the others
// are terminated with their children (a step that is running finishes; it cannot write after).
func (r *Runtime) cancelRunsOf(ctx context.Context, tenant, id uuid.UUID) error {
	rows, err := r.cfg.DB.Query(ctx, `SELECT r.id, r.instance_id, r.start_state`+runFrom+` WHERE r.tenant_id = $1 AND r.automation_id = $2 AND `+liveSQL, tenant, id)
	if err != nil {
		return serrors.FromDB(opService, err)
	}
	type live struct {
		id    uuid.UUID
		inst  *uuid.UUID
		state *string
	}
	var list []live
	for rows.Next() {
		var l live
		if err := rows.Scan(&l.id, &l.inst, &l.state); err != nil {
			rows.Close()
			return serrors.FromDB(opService, err)
		}
		list = append(list, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opService, err)
	}
	for _, l := range list {
		if err := r.cancelRun(ctx, tenant, l.id, l.inst); err != nil && !errors.Is(err, ErrRunFinished) {
			return err
		}
	}
	return nil
}

// cancelRun cancels one live run.
func (r *Runtime) cancelRun(ctx context.Context, tenant, id uuid.UUID, inst *uuid.UUID) error {
	if inst == nil {
		var n int64
		err := pgx.BeginFunc(ctx, r.cfg.DB, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `UPDATE automation.runs SET start_state = 'cancelled', updated_at = now() WHERE tenant_id = $1 AND id = $2 AND start_state = 'queued'`, tenant, id)
			if err != nil {
				return serrors.FromDB(opService, err)
			}
			n = tag.RowsAffected()
			_, err = tx.Exec(ctx, `DELETE FROM automation.subject_queue WHERE tenant_id = $1 AND run_id = $2`, tenant, id)
			return serrors.FromDB(opService, err)
		})
		if err == nil && n == 0 {
			return ErrRunFinished
		}
		return err
	}
	err := r.cfg.Workflows.TerminateTree(ctx, tenant, *inst)
	if errors.Is(err, workflow.ErrNotAccepting) || errors.Is(err, workflow.ErrNotFound) {
		return ErrRunFinished
	}
	return err
}

func (s *service) StartManual(ctx context.Context, a Actor, id uuid.UUID, in ManualStart) (RunRef, error) {
	if err := s.authorize(a, PermOperate); err != nil {
		return RunRef{}, err
	}
	am, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false)
	if err != nil {
		return RunRef{}, err
	}
	if am.EnabledVersion == nil {
		return RunRef{}, ErrNotEnabled
	}
	v, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, id, *am.EnabledVersion)
	if err != nil {
		return RunRef{}, err
	}
	ie, err := s.r.interpreter(ctx, a.TenantID, id, v.Version.Version, v.Graph)
	if err != nil {
		return RunRef{}, err
	}
	var trig *graph.Trigger
	for _, t := range ie.in.Graph().Triggers {
		if t.ID == in.TriggerID {
			t := t
			trig = &t
		}
	}
	if trig == nil || trig.Event != EventManual {
		return RunRef{}, fmt.Errorf("%w: %s is not a manual trigger of the enabled version", ErrInvalidInput, in.TriggerID)
	}
	input, err := decodeMap(in.Input)
	if err != nil {
		return RunRef{}, fmt.Errorf("%w: the input is not a JSON object", ErrInvalidInput)
	}
	if schema, ok := trig.Params["input_schema"].(map[string]any); ok && len(schema) > 0 {
		raw, err := json.Marshal(schema)
		if err != nil {
			return RunRef{}, err
		}
		ps, err := graph.SchemaParams(raw)
		if err != nil {
			return RunRef{}, fmt.Errorf("%w: the trigger's input schema: %v", ErrInvalidInput, err)
		}
		parsed, issues := ps.Parse(input)
		if len(issues) > 0 {
			return RunRef{}, fmt.Errorf("%w: input %v: %s", ErrInvalidInput, issues[0].Path, issues[0].Message)
		}
		input = parsed
	}
	subject := SubjectRef{Entity: graph.SubjectNone}
	if in.Subject != nil {
		subject = normSubject(*in.Subject)
	}
	user := a.UserID
	ev := events.Envelope{ID: uuid.NewString(), TenantID: a.TenantID, Event: EventManual, InitiatorUserID: &user, OccurredAt: time.Now()}
	gev := graph.Event{"kind": EventManual, "input": input, "initiator_user_id": a.UserID}
	req := startRequest{
		tenant: a.TenantID, automationID: id, version: v.Version.Version, in: ie.in, triggerID: trig.ID, kind: KindManual,
		key: "manual:" + ev.ID, event: gev, subject: subject, initiator: &user,
	}
	mode := events.RunAsOwner
	if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
		mode = events.RunAsInitiator
	}
	var ref RunRef
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		runAs, err := events.ResolveSubject(ctx, tx, s.r.cfg.Users, ev, ownerTrigger(id), mode)
		switch {
		case errors.Is(err, events.ErrNoSubject):
			req.startState, req.reason = startBlocked, "no_owner"
		case err != nil:
			return err
		default:
			req.runAs = &runAs
		}
		if req.runAs != nil {
			gctx := graph.Context{Event: gev, Now: ev.OccurredAt, Tenant: map[string]any{"id": a.TenantID.String()}}
			mismatch, st := ie.in.TriggerMismatch(trig.ID, gev, gctx)
			switch {
			case st != nil:
				req.runAs, req.startState, req.reason = nil, startFilterFailed, "filter_error: "+stepErr(*st).Message
			case mismatch != "":
				return fmt.Errorf("%w: the trigger's %s rejects the input", ErrInvalidInput, mismatch)
			}
		}
		if normSubject(subject).Entity != graph.SubjectNone {
			if err := lockSubjects(ctx, tx, subjectLockKey(a.TenantID, subject, uuid.Nil)); err != nil {
				return err
			}
		}
		ref, err = s.r.startRun(ctx, tx, req)
		return err
	})
	return ref, err
}

func (s *service) Cancel(ctx context.Context, a Actor, runID uuid.UUID) error {
	if err := s.authorize(a, PermOperate); err != nil {
		return err
	}
	run, err := loadRun(ctx, s.r.cfg.DB, a.TenantID, runID)
	if err != nil {
		return err
	}
	if run.status().finished() {
		return ErrRunFinished
	}
	return s.r.cancelRun(ctx, a.TenantID, run.ID, run.InstanceID)
}

func (s *service) Restart(ctx context.Context, a Actor, runID uuid.UUID, version int) (RunRef, error) {
	if err := s.authorize(a, PermOperate); err != nil {
		return RunRef{}, err
	}
	old, err := loadRun(ctx, s.r.cfg.DB, a.TenantID, runID)
	if err != nil {
		return RunRef{}, err
	}
	switch old.status() {
	case RunDone, RunFailed, RunCancelled, RunBlocked:
	case RunQueued, RunRunning, RunWaiting, RunFilterFailed:
		return RunRef{}, fmt.Errorf("%w: a %s run does not restart", ErrInvalidInput, old.status())
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, old.AutomationID, false); err != nil {
		return RunRef{}, err
	}
	v, err := loadVersion(ctx, s.r.cfg.DB, a.TenantID, old.AutomationID, version)
	if err != nil {
		return RunRef{}, err
	}
	ie, err := s.r.interpreter(ctx, a.TenantID, old.AutomationID, version, v.Graph)
	if err != nil {
		return RunRef{}, err
	}
	if _, ok := ie.in.Next(old.TriggerID, graph.TriggerOutcome); !ok {
		return RunRef{}, fmt.Errorf("%w: version %d has no trigger %s", ErrInvalidInput, version, old.TriggerID)
	}
	event, err := decodeMap(old.Input)
	if err != nil {
		return RunRef{}, err
	}
	req := startRequest{
		tenant: a.TenantID, automationID: old.AutomationID, version: version, in: ie.in, triggerID: old.TriggerID, kind: old.TriggerKind,
		key: "restart:" + uuid.NewString(), event: graph.Event(event), subject: old.Subject, initiator: old.InitiatorID,
		depth: old.Depth, cause: old.CauseRunID, restarted: &old.ID, predecessor: old.InstanceID,
	}
	if old.TriggerEventID != nil {
		req.eventID = *old.TriggerEventID
	}
	var ref RunRef
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if old.RunAsUserID != nil && old.RunAsKind != nil {
			sub, err := s.r.cfg.Users.LoadSubject(ctx, a.TenantID, *old.RunAsUserID, spi.SubjectKind(*old.RunAsKind))
			if err != nil {
				return fmt.Errorf("%w: the run's run-as user: %v", ErrForbidden, err)
			}
			sub.TenantID, sub.UserID, sub.Kind = a.TenantID, *old.RunAsUserID, spi.SubjectKind(*old.RunAsKind)
			req.runAs = &sub
		} else {
			ev := events.Envelope{ID: req.key, TenantID: a.TenantID, Event: old.TriggerKind, InitiatorUserID: old.InitiatorID}
			mode := events.RunAsOwner
			if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
				mode = events.RunAsInitiator
			}
			sub, err := events.ResolveSubject(ctx, tx, s.r.cfg.Users, ev, ownerTrigger(old.AutomationID), mode)
			switch {
			case errors.Is(err, events.ErrNoSubject):
				req.startState, req.reason = startBlocked, "no_owner"
			case err != nil:
				return err
			default:
				req.runAs = &sub
			}
		}
		if normSubject(old.Subject).Entity != graph.SubjectNone {
			if err := lockSubjects(ctx, tx, subjectLockKey(a.TenantID, old.Subject, uuid.Nil)); err != nil {
				return err
			}
		}
		var err error
		ref, err = s.r.startRun(ctx, tx, req)
		return err
	})
	return ref, err
}
