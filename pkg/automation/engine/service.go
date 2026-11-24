package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
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
		return fmt.Errorf("%w: the actor has no tenant, user or origin", ErrForbidden)
	}
	for _, p := range perms {
		if !a.Subject.Has(p) {
			return fmt.Errorf("%w: %s is required", ErrForbidden, p)
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
	var am automationRow
	var ver versionRow
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation.automations (tenant_id, id, name, latest_version, layout, created_by, created_origin)
			VALUES ($1, $2, $3, 1, $4, $5, $6)`, a.TenantID, id, v.g.Name, layout, a.UserID, string(a.Origin)); err != nil {
			return serrors.FromDB(opService, err)
		}
		if err := insertVersion(ctx, tx, a, id, 1, v, digest, in.Note); err != nil {
			return err
		}
		// The response is read in the transaction: a failed read rolls the graph back, so a retry
		// never creates a second one.
		if am, err = loadAutomation(ctx, tx, a.TenantID, id, false); err != nil {
			return err
		}
		ver, err = loadVersion(ctx, tx, a.TenantID, id, 1)
		return err
	})
	if err != nil {
		return Automation{}, Version{}, err
	}
	ver.Version.Warnings = v.warnings
	return am.Automation, ver.Version, nil
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
	var saved versionRow
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
		next := am.LatestVersion + 1
		if err := insertVersion(ctx, tx, a, id, next, v, digest, in.Note); err != nil {
			return err
		}
		merged := graph.Layout{}
		for k, xy := range am.Layout {
			merged[k] = xy
		}
		stale := in.LayoutBase != nil && *in.LayoutBase != am.LayoutRevision
		for k, xy := range in.Layout {
			if _, placed := am.Layout[k]; stale && placed {
				continue // another tab moved the layout since: its positions stay
			}
			merged[k] = xy
		}
		layout, err := layoutJSON(graph.LayoutOf(v.g, merged))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE automation.automations SET name = $3, latest_version = $4, layout = $5,
			layout_revision = layout_revision + 1, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			a.TenantID, id, v.g.Name, next, layout); err != nil {
			return serrors.FromDB(opService, err)
		}
		// The response is read in the transaction: a failed read rolls the version back, so a retry
		// with the same base never conflicts with a version this call saved.
		if saved, err = loadVersion(ctx, tx, a.TenantID, id, next); err != nil {
			return err
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
		err = s.r.enableTx(ctx, tx, a, id, next, v)
		if errors.Is(err, errConnectionGone) {
			// The connections the version uses changed since it was validated: the version is saved, the
			// graph keeps running the version it runs until a human enables this one.
			return nil
		}
		return err
	})
	if err != nil {
		return Version{}, err
	}
	saved.Version.Warnings = v.warnings
	return saved.Version, nil
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

// errConnectionGone: the connections the version was validated against changed (one it uses was
// deleted, left the connected status, or another one changed what it resolves to).
var errConnectionGone = errors.New("automation engine: the connections the version uses changed since it was validated")

// lockConnections locks the tenant connections v's graph uses FOR SHARE until the enable commits and
// checks that v's view still resolves them. A connection Delete locks its row FOR UPDATE from its
// UsageIndex check to its delete, so it either sees this enable committed (the use blocks it) or commits
// before the lock is granted, when the row is gone here. A status change of a used connection waits for
// the lock as well. Each used connection must still be connected, and the tenant's connections must be
// as they were when v was validated (connState): otherwise another connection may have become the one
// an omitted parameter resolves to (or been deleted meanwhile), and the enable is refused with
// errConnectionGone instead of publishing a version over a stale view.
func lockConnections(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, v validated) error {
	uses := connectionUses(v.view, v.g)
	if len(uses) == 0 {
		return nil
	}
	want := map[uuid.UUID]bool{}
	platform := map[string]bool{}
	for _, c := range uses {
		if kind, ok := strings.CutPrefix(c, catalog.PlatformPrefix); ok {
			platform[kind] = true // its availability may be an opt_in grant, locked below
			continue
		}
		id, err := uuid.Parse(c)
		if err != nil {
			return errConnectionGone // validation accepts only IDs it resolved: this one is not a row
		}
		want[id] = true
	}
	if len(want) > 0 {
		ids := make([]uuid.UUID, 0, len(want))
		for id := range want {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		rows, err := tx.Query(ctx, `SELECT id FROM automation.connections WHERE tenant_id = $1 AND id = ANY($2) AND status = 'connected' ORDER BY id FOR SHARE`, tenant, ids)
		if err != nil {
			return serrors.FromDB(opService, err)
		}
		found, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return serrors.FromDB(opService, err)
		}
		if len(found) != len(ids) {
			return errConnectionGone
		}
	}
	if len(platform) > 0 {
		// The grants of the platform credentials the graph uses are held FOR SHARE as well: revoking one
		// (PlatformGrants.SetEnabled) waits for this enable, or committed before and changed connState. A
		// kind without a grant row is available by its policy (auto), which no tenant write changes.
		kinds := make([]string, 0, len(platform))
		for k := range platform {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.platform_grants WHERE tenant_id = $1 AND kind = ANY($2) ORDER BY kind FOR SHARE`, tenant, kinds); err != nil {
			return serrors.FromDB(opService, err)
		}
	}
	now, err := connectionsState(ctx, tx, tenant)
	if err != nil {
		return err
	}
	if now != v.connState {
		return errConnectionGone
	}
	return nil
}

// enableTx pins a version: enabled_version, the owner (trigger_owners) and the version's triggers with
// the current outbox watermark, so events published before do not start runs. The caller holds the
// graph's row lock; the connections the version uses are locked first (lockConnections).
func (r *Runtime) enableTx(ctx context.Context, tx pgx.Tx, a Actor, id uuid.UUID, version int, v validated) error {
	view, g := v.view, v.g
	if err := lockConnections(ctx, tx, a.TenantID, v); err != nil {
		return err
	}
	// The triggers become active now, after the graph's lock was granted: the transaction's start
	// (now(), the column's default) can lie before it, and a schedule slot between the two would
	// count as one of this enable. One time for every trigger of the enable.
	var activeFrom time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&activeFrom); err != nil {
		return serrors.FromDB(opService, err)
	}
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
			INSERT INTO automation.triggers (tenant_id, automation_id, version, trigger_id, kind, event_kind, filter_cel, schedule, after_seq, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, a.TenantID, id, version, t.ID, t.Kind, event, filter, schedule, after, activeFrom); err != nil {
			return serrors.FromDB(opService, err)
		}
	}
	return nil
}

func (s *service) SaveLayout(ctx context.Context, a Actor, id uuid.UUID, l graph.Layout) error {
	_, err := s.saveLayout(ctx, a, id, -1, l)
	return err
}

func (s *service) SaveLayoutAt(ctx context.Context, a Actor, id uuid.UUID, baseRevision int, l graph.Layout) (int, error) {
	if baseRevision < 0 {
		return 0, fmt.Errorf("%w: a layout revision is never negative", ErrInvalidInput)
	}
	return s.saveLayout(ctx, a, id, baseRevision, l)
}

// saveLayout merges l into the layout; base < 0 saves over any revision. It returns the new revision.
func (s *service) saveLayout(ctx context.Context, a Actor, id uuid.UUID, base int, l graph.Layout) (int, error) {
	if err := s.authorize(a, PermAuthor); err != nil {
		return 0, err
	}
	if errs := graph.ValidateLayout(l); len(errs) > 0 {
		return 0, &ValidationError{Errors: errs}
	}
	var revision int
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		am, err := loadAutomation(ctx, tx, a.TenantID, id, true)
		if err != nil {
			return err
		}
		if base >= 0 && base != am.LayoutRevision {
			return &LayoutConflictError{Revision: am.LayoutRevision, Layout: am.Layout}
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
		err = tx.QueryRow(ctx, `UPDATE automation.automations SET layout = $3, layout_revision = layout_revision + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 RETURNING layout_revision`, a.TenantID, id, layout).Scan(&revision)
		return serrors.FromDB(opService, err)
	})
	if err != nil {
		return 0, err
	}
	return revision, nil
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
	b, err := json.Marshal(cursor{At: at, ID: id})
	if err != nil {
		panic(fmt.Sprintf("automation engine: marshal a cursor: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor reads a cursor; "" is the first page (ok false).
func decodeCursor(s string) (cursor, bool, error) {
	if s == "" {
		return cursor{}, false, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, false, fmt.Errorf("%w: cursor", ErrInvalidInput)
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return cursor{}, false, fmt.Errorf("%w: cursor", ErrInvalidInput)
	}
	return c, true, nil
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
	c, after, err := decodeCursor(f.Cursor)
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
	if f.Attention {
		where = append(where, `EXISTS (SELECT 1`+runFrom+` WHERE r.tenant_id = automations.tenant_id AND r.automation_id = automations.id AND `+attentionSQL+`)`)
	}
	if after {
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
	// One snapshot: the enabled flag and the versions describe the same committed configuration.
	tx, err := s.r.cfg.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	am, err := loadAutomation(ctx, tx, a.TenantID, id, false)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT version, saved_by, saved_origin, note, created_at FROM automation.versions WHERE tenant_id = $1 AND automation_id = $2 ORDER BY version DESC`, a.TenantID, id)
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
	rv := reviewOf(s.r, view, v, g)
	rv.Problems = []graph.GraphError{}
	// The check Enable makes, told before the person presses Enable.
	var verr *ValidationError
	if _, err := s.r.validate(ctx, a, v.Graph, storedParams); errors.As(err, &verr) {
		rv.Problems = verr.Errors
	} else if err != nil {
		return EnableReview{}, err
	}
	if !triggersReadable(view, g) {
		rv.Problems = append(rv.Problems, graph.GraphError{Code: CodeTriggersUnreadable, Message: "the triggers' events need a permission you do not have"})
	}
	return rv, nil
}

func (s *service) Enable(ctx context.Context, a Actor, id uuid.UUID, in EnableInput) (Automation, error) {
	if a.Origin != OriginHuman {
		return Automation{}, ErrHumanRequired
	}
	if err := s.authorize(a, PermEnable); err != nil {
		return Automation{}, err
	}
	current, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false)
	if err != nil {
		return Automation{}, err
	}
	// A version superseded since the review is reviewed again before anything about it is judged (its
	// digest, its validation): the newer version may already fix what the older one lacks.
	if in.Latest && current.LatestVersion != in.Version {
		return Automation{}, ErrReviewRequired
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
	// Enable and Disable answer under their own authorization (the caller may lack Automations.Read),
	// with the graph as their transaction committed it.
	var am automationRow
	for attempt := 0; ; attempt++ {
		checked, err := s.r.validate(ctx, a, v.Graph, storedParams)
		if err != nil {
			return Automation{}, err
		}
		err = s.inTx(ctx, func(tx pgx.Tx) error {
			locked, err := loadAutomation(ctx, tx, a.TenantID, id, true)
			if err != nil {
				return err
			}
			if in.Latest && locked.LatestVersion != in.Version {
				return ErrReviewRequired
			}
			if err := s.r.enableTx(ctx, tx, a, id, in.Version, checked); err != nil {
				return err
			}
			am, err = loadAutomation(ctx, tx, a.TenantID, id, false)
			return err
		})
		if errors.Is(err, errConnectionGone) && attempt == 0 {
			continue // validated again against the catalog of now: a connection that is gone is a validation error
		}
		if errors.Is(err, errConnectionGone) {
			return Automation{}, fmt.Errorf("%w: the connections the version uses changed meanwhile", ErrInvalidInput)
		}
		if err != nil {
			return Automation{}, err
		}
		return am.Automation, nil
	}
}

func (s *service) Disable(ctx context.Context, a Actor, id uuid.UUID) (Automation, error) {
	if err := s.authorize(a, PermEnable); err != nil {
		return Automation{}, err
	}
	// Enable and Disable answer under their own authorization (the caller may lack Automations.Read),
	// with the graph as their transaction committed it: an Enable after it is not this call's answer.
	var am automationRow
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := loadAutomation(ctx, tx, a.TenantID, id, true); err != nil {
			return err
		}
		if err := disableTx(ctx, tx, a.TenantID, id); err != nil {
			return err
		}
		var err error
		am, err = loadAutomation(ctx, tx, a.TenantID, id, false)
		return err
	})
	if err != nil {
		return Automation{}, err
	}
	if err := s.r.cancelRunsOf(ctx, a.TenantID, id); err != nil {
		return Automation{}, err
	}
	return am.Automation, nil
}

// disableTx stops a graph in the caller's transaction, which holds the graph's row lock: no trigger
// starts it any more, its queued runs are cancelled, and every live run is marked cancel_requested, so the
// cancellation survives a process failure before cancelRunsOf and a later Enable. Queued runs are
// promoted only under the same row lock (dequeue), so none starts once this commits.
func disableTx(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE automation.automations SET enabled_version = NULL, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenant, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM automation.triggers WHERE tenant_id = $1 AND automation_id = $2`, tenant, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM automation.subject_queue q USING automation.runs r
		WHERE q.tenant_id = $1 AND r.tenant_id = q.tenant_id AND r.id = q.run_id AND r.automation_id = $2`, tenant, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE automation.runs SET start_state = 'cancelled', updated_at = now()
		WHERE tenant_id = $1 AND automation_id = $2 AND instance_id IS NULL AND start_state = 'queued'`, tenant, id); err != nil {
		return serrors.FromDB(opService, err)
	}
	_, err := tx.Exec(ctx, `
		UPDATE automation.runs r SET cancel_requested = true, updated_at = now()
		FROM extruntime.workflow_instances i
		WHERE r.tenant_id = $1 AND r.automation_id = $2 AND i.id = r.instance_id AND i.status IN ('running', 'waiting', 'blocked')`, tenant, id)
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
	rows, err := r.cfg.DB.Query(ctx, `SELECT r.id, r.instance_id, r.start_state`+runFrom+` WHERE r.tenant_id = $1 AND r.automation_id = $2 AND r.cancel_requested AND `+liveSQL, tenant, id)
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
			// The subject's lock first, as a promotion (dequeue) holds it before it deletes the queue row
			// and starts the run: both then change the queue row and the run row in one order, and
			// neither aborts the other as a deadlock.
			var s SubjectRef
			if err := tx.QueryRow(ctx, `SELECT subject_entity, subject_key FROM automation.runs WHERE tenant_id = $1 AND id = $2`, tenant, id).Scan(&s.Entity, &s.Key); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrNotFound
				}
				return serrors.FromDB(opService, err)
			}
			if normSubject(s).Entity == graph.SubjectConversation {
				if err := lockSubjects(ctx, tx, subjectLockKey(tenant, s, uuid.Nil)); err != nil {
					return err
				}
			}
			// The queue row before the run row, as promotion (dequeue) and Disable (disableTx) change them:
			// no two of them wait for each other's row.
			if _, err := tx.Exec(ctx, `DELETE FROM automation.subject_queue WHERE tenant_id = $1 AND run_id = $2`, tenant, id); err != nil {
				return serrors.FromDB(opService, err)
			}
			// A queued run, or one recorded blocked without an instance (no owner, depth).
			tag, err := tx.Exec(ctx, `UPDATE automation.runs SET start_state = 'cancelled', updated_at = now()
				WHERE tenant_id = $1 AND id = $2 AND instance_id IS NULL AND start_state IN ('queued', 'blocked')`, tenant, id)
			if err != nil {
				return serrors.FromDB(opService, err)
			}
			n = tag.RowsAffected()
			return nil
		})
		if err != nil || n > 0 {
			return err
		}
		// Nothing to cancel without an instance: a promotion may have started the run meanwhile.
		run, err := loadRun(ctx, r.cfg.DB, tenant, id)
		if err != nil {
			return err
		}
		if run.InstanceID == nil || run.status().finished() {
			return ErrRunFinished
		}
		inst = run.InstanceID
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
		// Locks in the order steps and the router take them: the subject, then the graph's row. The
		// run-as is resolved under the graph's lock, so it is the owner of the enable that admits the
		// run (eligible): an Enable that commits a new owner of the same version waits for this start or
		// is seen whole by it.
		if normSubject(subject).Entity != graph.SubjectNone {
			if err := lockSubjects(ctx, tx, subjectLockKey(a.TenantID, subject, uuid.Nil)); err != nil {
				return err
			}
		}
		if err := lockGraphShared(ctx, tx, a.TenantID, id, v.Version.Version); err != nil {
			return err
		}
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
		ref, err = s.r.startRun(ctx, tx, req)
		return err
	})
	return ref, err
}

// lockGraphShared locks the graph's row FOR SHARE (the lock eligible takes; Enable and Disable hold it
// FOR UPDATE) and requires version enabled on it.
func lockGraphShared(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID, version int) error {
	var enabled *int
	err := tx.QueryRow(ctx, `SELECT enabled_version FROM automation.automations WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR SHARE`,
		tenant, id).Scan(&enabled)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return serrors.FromDB(opService, err)
	case enabled == nil || *enabled != version:
		return fmt.Errorf("%w: version %d of %s is not enabled", ErrNotEnabled, version, id)
	}
	return nil
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
	if err := restartable(old); err != nil {
		return RunRef{}, err
	}
	am, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, old.AutomationID, false)
	if err != nil {
		return RunRef{}, err
	}
	// Only code a human enabled runs: the enabled version, or the version the run already ran on.
	if version != old.Version && (am.EnabledVersion == nil || version != *am.EnabledVersion) {
		return RunRef{}, fmt.Errorf("%w: version %d is neither enabled nor the run's own", ErrForbidden, version)
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
	kind, _ := event["kind"].(string)
	req := startRequest{
		tenant: a.TenantID, automationID: old.AutomationID, version: version, in: ie.in, triggerID: old.TriggerID, kind: old.TriggerKind,
		key: "restart:" + uuid.NewString(), subject: old.Subject, initiator: old.InitiatorID,
		depth: old.Depth, cause: old.CauseRunID, restarted: &old.ID, predecessor: old.InstanceID, restartOf: old.Version,
	}
	if old.TriggerEventID != nil {
		req.eventID = *old.TriggerEventID
	}
	var ref RunRef
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		// Locks in the order steps take them: the subject, then the predecessor's instance (shared, so its
		// status cannot change under us; no run row is locked while waiting for it). The status is checked
		// again under them. The predecessor is never touched (StartV2).
		if normSubject(old.Subject).Entity != graph.SubjectNone {
			if err := lockSubjects(ctx, tx, subjectLockKey(a.TenantID, old.Subject, uuid.Nil)); err != nil {
				return err
			}
		}
		if old.InstanceID != nil {
			if _, err := tx.Exec(ctx, `SELECT 1 FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2 FOR SHARE`, a.TenantID, *old.InstanceID); err != nil {
				return serrors.FromDB(opService, err)
			}
		}
		now, err := loadRun(ctx, tx, a.TenantID, old.ID)
		if err != nil {
			return err
		}
		if err := restartable(now); err != nil {
			return err
		}
		// The owner is read under the graph's row lock (the lock eligible takes), as for a manual start.
		var deleted bool
		if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR SHARE`,
			a.TenantID, old.AutomationID).Scan(&deleted); errors.Is(err, pgx.ErrNoRows) || (err == nil && deleted) {
			return ErrNotFound
		} else if err != nil {
			return serrors.FromDB(opService, err)
		}
		// Run-as by the chosen version's policy: its owner now, or the original initiator.
		ev := events.Envelope{ID: req.key, TenantID: a.TenantID, Event: kind, InitiatorUserID: old.InitiatorID}
		mode := events.RunAsOwner
		if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
			mode = events.RunAsInitiator
		}
		sub, err := events.ResolveSubject(ctx, tx, s.r.cfg.Users, ev, ownerTrigger(old.AutomationID), mode)
		var as *spi.Subject
		switch {
		case errors.Is(err, events.ErrNoSubject):
			req.startState, req.reason = startBlocked, "no_owner"
		case err != nil:
			return err
		default:
			as = &sub
			req.runAs = as
		}
		// The retained input is what the new run-as may read of it.
		payload := map[string]any{}
		for k, val := range event {
			if k != "kind" {
				payload[k] = val
			}
		}
		// A value the original run recorded as sensitive whose access rule the catalog no longer has
		// (the field lost it, or its event left) cannot be checked: history of the original run hides it,
		// and the replay leaves it out instead of carrying it on unrestricted. Fields still classified
		// keep their rule: the projection below and the new run's provenance (startRun) apply it.
		// Only the input's own provenance counts: runs.sensitive also holds the fields of events the run's
		// waits matched later, whose paths name fields of other events, not of this input.
		proj := s.r.projector(a.TenantID)
		for _, ref := range old.Sensitive {
			if ref.Event != kind {
				continue
			}
			ok, err := proj.resolvable(ctx, ref)
			if err != nil {
				return &host.InfraError{Err: err}
			}
			if !ok {
				removePath(payload, strings.Split(ref.Path, "."))
			}
		}
		if kind != "" {
			if payload, err = proj.project(ctx, kind, payload, as); err != nil {
				return err
			}
		}
		gev := graph.Event{}
		for k, val := range payload {
			gev[k] = val
		}
		if kind != "" {
			gev["kind"] = kind
		}
		req.event = gev
		if req.runAs != nil && req.depth > s.r.cfg.Limits.MaxDepth {
			req.runAs, req.startState, req.reason = nil, startBlocked, "depth_exceeded"
		}
		ref, err = s.r.startRun(ctx, tx, req)
		return err
	})
	return ref, err
}

// restartable reports whether a run may restart: finished, or blocked (also with a live blocked
// instance, which StartV2 leaves as it is).
func restartable(r runRow) error {
	switch r.status() {
	case RunDone, RunFailed, RunCancelled, RunBlocked:
		return nil
	case RunQueued, RunRunning, RunWaiting, RunFilterFailed:
	}
	return fmt.Errorf("%w: a %s run does not restart", ErrInvalidInput, r.status())
}
