package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// liveStatuses are the workflow instance states that still need their pinned
// package version and data.
const liveStatuses = `('running', 'waiting', 'blocked')`

// workflowInstancesAvailable reports whether the workflow slice's table
// exists with the columns the management checks read.
func workflowInstancesAvailable(ctx context.Context, q querier) (bool, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'extruntime' AND table_name = 'workflow_instances' AND column_name IN ('package', 'version', 'status')`).Scan(&n)
	return n == 3, serrors.FromDB("extruntime.manage.workflowInstancesAvailable", err)
}

// pinnedVersions lists the versions of ns that live workflow instances pin.
func pinnedVersions(ctx context.Context, q querier, ns string) ([]string, error) {
	const op = serrors.Op("extruntime.manage.pinnedVersions")
	ok, err := workflowInstancesAvailable(ctx, q)
	if err != nil || !ok {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT version FROM extruntime.workflow_instances WHERE package = $1 AND status IN `+liveStatuses, ns)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, v)
	}
	return out, serrors.FromDB(op, rows.Err())
}

func blocked(err error) error {
	return serrors.New(serrors.FailedPrecondition, "activation blocked: "+err.Error()).WithCause(err)
}

// schemaPlan plans (and, when apply is set, applies) the data schema of a
// version and checks it against the versions live workflow instances pin.
func (s *Service) schemaPlan(ctx context.Context, tx pgx.Tx, ns, version string, m *manifest.Manifest, apply bool) (data.MigrationPlan, error) {
	applied, err := data.LoadApplied(ctx, tx, ns)
	if err != nil {
		return data.MigrationPlan{}, err
	}
	desired, err := s.compiler.Compile(ctx, ns, version, m)
	if err != nil {
		return data.MigrationPlan{}, blocked(err)
	}
	plan, err := data.Plan(applied, desired)
	if err != nil {
		return plan, blocked(err)
	}
	pinned, err := pinnedVersions(ctx, tx, ns)
	if err != nil {
		return plan, err
	}
	var views []*data.Schema
	for _, v := range pinned {
		if v == version {
			continue
		}
		view, found, err := data.LoadView(ctx, tx, ns, v)
		if err != nil {
			return plan, err
		}
		if found {
			views = append(views, view)
		}
	}
	if err := plan.CompatibleWithPinned(views...); err != nil {
		return plan, blocked(fmt.Errorf("a running workflow instance pins an incompatible version: %w", err))
	}
	if apply {
		if err := data.Apply(ctx, tx, plan); err != nil {
			return plan, blocked(err)
		}
	}
	return plan, nil
}

// checkActivatable validates what does not need the database transaction:
// namespace ownership, resource ID collisions, dependencies and the
// consistency of implementations.
func (s *Service) checkActivatable(ctx context.Context, ns string, m *manifest.Manifest) error {
	if err := m.Validate(); err != nil {
		return serrors.New(serrors.FailedPrecondition, err.Error())
	}
	if m.Namespace != ns {
		return serrors.New(serrors.FailedPrecondition, "manifest namespace differs from the package")
	}
	if s.catalog.Natives().Reserved(ns) {
		return serrors.New(serrors.FailedPrecondition, "namespace "+ns+" is reserved by a native module")
	}
	for _, r := range m.Resources {
		cur, found, err := s.catalog.Resolve(ctx, r.ID)
		if err != nil {
			return err
		}
		if found && (cur.Native || cur.Package != ns) {
			owner := cur.Package
			if cur.Native {
				owner = "native"
			}
			return serrors.New(serrors.FailedPrecondition, fmt.Sprintf("resource %s is already declared by %s", r.ID, owner))
		}
	}
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return err
	}
	for dep := range m.Dependencies {
		if s.catalog.Natives().Reserved(dep) {
			continue
		}
		found := false
		for _, av := range active {
			if av.Namespace == dep {
				found = true
			}
		}
		if !found && dep != ns {
			return serrors.New(serrors.FailedPrecondition, "dependency "+dep+" is not active")
		}
	}
	seen := map[string]bool{}
	for _, i := range m.Implementations {
		if !ownsID(ns, i.Implements) || !bindsAsCommand(i.Kind) {
			continue
		}
		res, ok := m.Find(i.Implements)
		if !ok {
			return serrors.New(serrors.FailedPrecondition, "implementation "+i.Export+" implements unknown "+i.Implements)
		}
		if !implMatches(i, res) {
			return serrors.New(serrors.FailedPrecondition, fmt.Sprintf("implementation %s (%s) does not match %s %s (atomic=%v)", i.Export, i.Kind, res.Kind, res.ID, res.Atomic))
		}
		if seen[i.Implements] {
			return serrors.New(serrors.FailedPrecondition, i.Implements+" has more than one implementation in the package")
		}
		seen[i.Implements] = true
	}
	return nil
}

func notifyCatalog(ctx context.Context, tx pgx.Tx, what string) error {
	_, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, catalog.NotifyChannel, what)
	return serrors.FromDB("extruntime.manage.notifyCatalog", err)
}

// Activate makes an approved version the live one of its package: its schema
// plan is applied in the same transaction (refused when a live workflow
// instance pins an incompatible version), the package's own command
// implementations are bound, replacement bindings are carried over, and the
// catalog is invalidated on commit. Application-wide: tenants need only a
// trigger owner (AssignOwner) for subscriptions and schedules.
func (s *Service) Activate(ctx context.Context, actor spi.Subject, ns, version string) (ActivationResult, error) {
	const op = serrors.Op("extruntime.manage.Activate")
	e := &auditEntry{Action: ActionActivate, Namespace: ns, Version: version, Detail: map[string]any{}}
	var out ActivationResult
	err := s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		v, err := s.versionRow(ctx, tx, ns, version)
		if err != nil {
			return err
		}
		e.SourceHash, e.ArtifactHash = v.SourceHash, v.ArtifactHash
		if !v.Approved {
			return serrors.New(serrors.FailedPrecondition, "version "+ns+"@"+version+" is not approved")
		}
		if v.Active {
			return serrors.New(serrors.AlreadyExists, "version "+ns+"@"+version+" is already active")
		}
		m := v.Manifest
		if err := s.checkActivatable(ctx, ns, &m); err != nil {
			return err
		}
		plan, err := s.schemaPlan(ctx, tx, ns, version, &m, true)
		if err != nil {
			return err
		}
		steps := make([]string, len(plan.Steps))
		for i, st := range plan.Steps {
			steps[i] = st.SQL
		}
		out.SchemaSteps = steps
		stepsJSON, _ := json.Marshal(steps)

		if _, err := tx.Exec(ctx, `UPDATE extruntime.activations SET state = 'disabled', ended_by = $2, ended_at = CURRENT_TIMESTAMP
			WHERE namespace = $1 AND state = 'active'`, ns, actor.UserID); err != nil {
			return serrors.FromDB(op, err)
		}
		var act Activation
		var state string
		if err := tx.QueryRow(ctx, `INSERT INTO extruntime.activations (namespace, version_id, state, activated_by, schema_steps)
			VALUES ($1, $2, 'active', $3, $4::jsonb) RETURNING id, namespace, state, activated_by, activated_at`,
			ns, v.ID, actor.UserID, string(stepsJSON)).Scan(&act.ID, &act.Namespace, &state, &act.ActivatedBy, &act.ActivatedAt); err != nil {
			return serrors.FromDB(op, err)
		}
		act.State, act.Version, act.SchemaSteps = ActivationState(state), version, steps
		out.Activation = act

		if err := s.rebind(ctx, tx, actor.UserID, ns, version, &m, &out); err != nil {
			return err
		}
		if err := notifyCatalog(ctx, tx, ns+"@"+version); err != nil {
			return err
		}
		e.Detail["schemaSteps"] = len(steps)
		e.Detail["bound"], e.Detail["rebound"], e.Detail["released"] = len(out.Bound), len(out.Rebound), len(out.Released)
		e.After = s.catalog.Invalidate
		return nil
	})
	return out, err
}

// rebind moves the package's bindings to the newly active version.
func (s *Service) rebind(ctx context.Context, tx pgx.Tx, by int64, ns, version string, m *manifest.Manifest, out *ActivationResult) error {
	old, err := openBindingsOf(ctx, tx, ns)
	if err != nil {
		return err
	}
	for _, b := range old {
		if b.SlotKind == SlotCommand && ownsID(ns, b.TargetID) {
			if err := closeBinding(ctx, tx, b.ID); err != nil {
				return err
			}
			continue
		}
		export := ""
		switch b.SlotKind {
		case SlotCommand:
			target, found, err := s.catalog.Resolve(ctx, b.TargetID)
			if err != nil {
				return err
			}
			var candidates []string
			for _, i := range m.Implementations {
				// An export the target's atomicity does not fit never serves it.
				if i.Implements == b.TargetID && found && implMatches(i, target.Resource) {
					candidates = append(candidates, i.Export)
				}
			}
			// The replacement keeps its export when the new version still
			// has it; a single other fitting export takes over; otherwise
			// the choice is ambiguous and the binding is released.
			for _, c := range candidates {
				if c == b.Export {
					export = c
				}
			}
			if export == "" && len(candidates) == 1 {
				export = candidates[0]
			}
		case SlotScreen, SlotShell:
			// A client replacement keeps its export name across versions.
			want := manifest.ResPage
			if b.SlotKind == SlotShell {
				want = manifest.ResShell
			}
			for _, r := range m.Resources {
				if r.Kind == want && r.Export == b.Export {
					export = r.Export
				}
			}
		}
		if export == "" {
			if _, err := release(ctx, tx, b, by); err != nil {
				return err
			}
			out.Released = append(out.Released, b)
			continue
		}
		chain, err := displacedOf(ctx, tx, b.ID)
		if err != nil {
			return err
		}
		if err := closeBinding(ctx, tx, b.ID); err != nil {
			return err
		}
		nb, err := insertBinding(ctx, tx, b.SlotKind, b.TargetID, ns, version, export, by, false)
		if err != nil {
			return err
		}
		if chain != nil {
			if err := setDisplaced(ctx, tx, nb.ID, *chain); err != nil {
				return err
			}
		}
		out.Rebound = append(out.Rebound, nb)
	}
	for _, i := range m.Implementations {
		if !ownsID(ns, i.Implements) || !bindsAsCommand(i.Kind) {
			continue
		}
		holder, err := slotHolder(ctx, tx, SlotCommand, i.Implements)
		if err != nil {
			return err
		}
		if holder != nil {
			// A replacement from another package holds the slot: keep it,
			// and remember this implementation as what it displaced.
			ph, err := insertBinding(ctx, tx, SlotCommand, i.Implements, ns, version, i.Export, by, true)
			if err != nil {
				return err
			}
			if err := setDisplaced(ctx, tx, holder.ID, ph.ID); err != nil {
				return err
			}
			continue
		}
		nb, err := insertBinding(ctx, tx, SlotCommand, i.Implements, ns, version, i.Export, by, false)
		if err != nil {
			return err
		}
		out.Bound = append(out.Bound, nb)
	}
	return nil
}

// Deactivate takes the active version of a package out of service: its
// contributions leave the catalog and its bindings end (slots fall back to
// the implementations they displaced). Tables and data are kept; Purge is a
// separate explicit action.
func (s *Service) Deactivate(ctx context.Context, actor spi.Subject, ns string) error {
	const op = serrors.Op("extruntime.manage.Deactivate")
	e := &auditEntry{Action: ActionDeactivate, Namespace: ns, Detail: map[string]any{}}
	return s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		var version string
		err := tx.QueryRow(ctx, `SELECT v.version FROM extruntime.activations a JOIN extruntime.versions v ON v.id = a.version_id
			WHERE a.namespace = $1 AND a.state = 'active'`, ns).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return serrors.New(serrors.FailedPrecondition, "package "+ns+" has no active version")
		}
		if err != nil {
			return serrors.FromDB(op, err)
		}
		e.Version = version
		active, err := s.catalog.Active(ctx)
		if err != nil {
			return err
		}
		for _, av := range active {
			if _, dep := av.Manifest.Dependencies[ns]; dep && av.Namespace != ns {
				return serrors.New(serrors.FailedPrecondition, "active package "+av.Namespace+" depends on "+ns)
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE extruntime.activations SET state = 'disabled', ended_by = $2, ended_at = CURRENT_TIMESTAMP
			WHERE namespace = $1 AND state = 'active'`, ns, actor.UserID); err != nil {
			return serrors.FromDB(op, err)
		}
		open, err := openBindingsOf(ctx, tx, ns)
		if err != nil {
			return err
		}
		for _, b := range open {
			if _, err := release(ctx, tx, b, actor.UserID); err != nil {
				return err
			}
		}
		e.Detail["released"] = len(open)
		if err := notifyCatalog(ctx, tx, ns); err != nil {
			return err
		}
		e.After = s.catalog.Invalidate
		return nil
	})
}

// Purge drops the data of a deactivated package: its entity tables and the
// recorded schema state. It is refused while the package is active or live
// workflow instances still pin it.
func (s *Service) Purge(ctx context.Context, actor spi.Subject, ns string) error {
	const op = serrors.Op("extruntime.manage.Purge")
	e := &auditEntry{Action: ActionPurge, Namespace: ns, Detail: map[string]any{}}
	return s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		if _, err := s.packageRow(ctx, tx, ns, true); err != nil {
			return err
		}
		var active bool
		var disabled int
		if err := tx.QueryRow(ctx, `SELECT bool_or(state = 'active') IS TRUE, count(*) FILTER (WHERE state = 'disabled')
			FROM extruntime.activations WHERE namespace = $1`, ns).Scan(&active, &disabled); err != nil {
			return serrors.FromDB(op, err)
		}
		if active {
			return serrors.New(serrors.FailedPrecondition, "package "+ns+" is active; deactivate it before purging")
		}
		if disabled == 0 {
			return serrors.New(serrors.FailedPrecondition, "package "+ns+" has no deactivated data to purge")
		}
		pinned, err := pinnedVersions(ctx, tx, ns)
		if err != nil {
			return err
		}
		if len(pinned) > 0 {
			return serrors.New(serrors.FailedPrecondition, "live workflow instances still pin "+ns+" ("+strings.Join(pinned, ", ")+")")
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "extruntime.schema:"+ns); err != nil {
			return serrors.FromDB(op, err)
		}
		rows, err := tx.Query(ctx, `SELECT schema_name, table_name FROM extruntime.entity_schemas WHERE package = $1 ORDER BY kind DESC, entity_id`, ns)
		if err != nil {
			return serrors.FromDB(op, err)
		}
		var tables []pgx.Identifier
		for rows.Next() {
			var sch, tbl string
			if err := rows.Scan(&sch, &tbl); err != nil {
				rows.Close()
				return serrors.FromDB(op, err)
			}
			tables = append(tables, pgx.Identifier{sch, tbl})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return serrors.FromDB(op, err)
		}
		for _, t := range tables {
			if _, err := tx.Exec(ctx, `DROP TABLE IF EXISTS `+t.Sanitize()+` CASCADE`); err != nil {
				return serrors.FromDB(op, err)
			}
		}
		for _, q := range []string{
			`DELETE FROM extruntime.entity_schema_versions WHERE package = $1`,
			`DELETE FROM extruntime.entity_schemas WHERE package = $1`,
		} {
			if _, err := tx.Exec(ctx, q, ns); err != nil {
				return serrors.FromDB(op, err)
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE extruntime.activations SET state = 'purged' WHERE namespace = $1 AND state = 'disabled'`, ns); err != nil {
			return serrors.FromDB(op, err)
		}
		e.Detail["tablesDropped"] = len(tables)
		return nil
	})
}
