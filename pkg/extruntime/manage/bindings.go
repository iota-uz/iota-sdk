package manage

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const bindingColumns = `id, slot_kind, target_id, package, version, export, activated_by, active_from, active_to`

func scanBinding(row pgx.Row) (Binding, error) {
	var b Binding
	err := row.Scan(&b.ID, &b.SlotKind, &b.TargetID, &b.Package, &b.Version, &b.Export, &b.ActivatedBy, &b.ActiveFrom, &b.ActiveTo)
	return b, err
}

func openBindingsOf(ctx context.Context, tx pgx.Tx, pkg string) ([]Binding, error) {
	const op = serrors.Op("extruntime.manage.openBindingsOf")
	rows, err := tx.Query(ctx, `SELECT `+bindingColumns+` FROM extruntime.bindings WHERE package = $1 AND active_to IS NULL ORDER BY slot_kind, target_id`, pkg)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []Binding
	for rows.Next() {
		b, err := scanBinding(rows)
		if err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, b)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// slotHolder returns the open binding of a slot, nil when the slot is free.
func slotHolder(ctx context.Context, tx pgx.Tx, kind, target string) (*Binding, error) {
	b, err := scanBinding(tx.QueryRow(ctx, `SELECT `+bindingColumns+` FROM extruntime.bindings
		WHERE slot_kind = $1 AND target_id = $2 AND active_to IS NULL FOR UPDATE`, kind, target))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, serrors.FromDB("extruntime.manage.slotHolder", err)
	}
	return &b, nil
}

func closeBinding(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE extruntime.bindings SET active_to = CURRENT_TIMESTAMP WHERE id = $1 AND active_to IS NULL`, id)
	return serrors.FromDB("extruntime.manage.closeBinding", err)
}

func insertBinding(ctx context.Context, tx pgx.Tx, kind, target, pkg, version, export string, by int64, closed bool) (Binding, error) {
	sql := `INSERT INTO extruntime.bindings (id, slot_kind, target_id, package, version, export, activated_by) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + bindingColumns
	if closed {
		// A placeholder for an implementation that never ran because a
		// replacement already held the slot: it exists so the replacement
		// can fall back to it.
		sql = `INSERT INTO extruntime.bindings (id, slot_kind, target_id, package, version, export, activated_by, active_to)
			VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP) RETURNING ` + bindingColumns
	}
	b, err := scanBinding(tx.QueryRow(ctx, sql, uuid.New(), kind, target, pkg, version, export, by))
	return b, serrors.FromDB("extruntime.manage.insertBinding", err)
}

func displacedOf(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*uuid.UUID, error) {
	var d uuid.UUID
	err := tx.QueryRow(ctx, `SELECT displaced_binding_id FROM extruntime.binding_displaced WHERE binding_id = $1`, id).Scan(&d)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, serrors.FromDB("extruntime.manage.displacedOf", err)
	}
	return &d, nil
}

func setDisplaced(ctx context.Context, tx pgx.Tx, id, displaced uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO extruntime.binding_displaced (binding_id, displaced_binding_id) VALUES ($1, $2)
		ON CONFLICT (binding_id) DO UPDATE SET displaced_binding_id = EXCLUDED.displaced_binding_id`, id, displaced)
	return serrors.FromDB("extruntime.manage.setDisplaced", err)
}

func bindingByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Binding, error) {
	b, err := scanBinding(tx.QueryRow(ctx, `SELECT `+bindingColumns+` FROM extruntime.bindings WHERE id = $1`, id))
	return b, serrors.FromDB("extruntime.manage.bindingByID", err)
}

func versionActive(ctx context.Context, tx pgx.Tx, ns, version string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM extruntime.activations a JOIN extruntime.versions v ON v.id = a.version_id
		WHERE a.state = 'active' AND v.namespace = $1 AND v.version = $2)`, ns, version).Scan(&ok)
	return ok, serrors.FromDB("extruntime.manage.versionActive", err)
}

// release ends a binding and gives the slot back to the implementation it
// displaced, when that implementation's version is still active. It returns
// the reinstated binding, if any. There is no fallback to anything else.
func release(ctx context.Context, tx pgx.Tx, b Binding, by int64) (*Binding, error) {
	if err := closeBinding(ctx, tx, b.ID); err != nil {
		return nil, err
	}
	d, err := displacedOf(ctx, tx, b.ID)
	for err == nil && d != nil {
		var prev Binding
		if prev, err = bindingByID(ctx, tx, *d); err != nil {
			break
		}
		var ok bool
		if ok, err = versionActive(ctx, tx, prev.Package, prev.Version); err != nil {
			break
		}
		if ok {
			nb, ierr := insertBinding(ctx, tx, prev.SlotKind, prev.TargetID, prev.Package, prev.Version, prev.Export, by, false)
			if ierr != nil {
				return nil, ierr
			}
			if chain, cerr := displacedOf(ctx, tx, prev.ID); cerr != nil {
				return nil, cerr
			} else if chain != nil {
				if serr := setDisplaced(ctx, tx, nb.ID, *chain); serr != nil {
					return nil, serr
				}
			}
			return &nb, nil
		}
		d, err = displacedOf(ctx, tx, prev.ID)
	}
	return nil, err
}

// invocable kinds whose own implementations the host resolves through
// slot_kind 'command' bindings.
func bindsAsCommand(k manifest.ImplKind) bool {
	switch k {
	case manifest.ImplCommand, manifest.ImplAtomic, manifest.ImplOperation, manifest.ImplQuery:
		return true
	}
	return false
}

func implMatches(i manifest.Implementation, r manifest.Resource) bool {
	switch r.Kind {
	case manifest.ResCommand:
		if r.Atomic {
			return i.Kind == manifest.ImplAtomic
		}
		return i.Kind == manifest.ImplCommand
	case manifest.ResOperation:
		return i.Kind == manifest.ImplOperation
	case manifest.ResQuery:
		return i.Kind == manifest.ImplQuery
	case manifest.ResWorkflow:
		return i.Kind == manifest.ImplWorkflow
	case manifest.ResContactResolver:
		return i.Kind == manifest.ImplContactResolver
	case manifest.ResDeliveryPolicy:
		return i.Kind == manifest.ImplDeliveryPolicy
	}
	return false
}

// ownsID says whether the resource belongs to the package's own namespace.
func ownsID(ns, id string) bool { return strings.HasPrefix(id, ns+".") }

// Bind makes package@version/export the active implementation of a foreign
// or native slot: a command, a screen (page) or the shell. Exactly one
// binding is active per slot, there is no fallback on failure, and
// unbinding (or deactivating the package) returns the slot to the
// implementation it displaced. A package's own commands are bound by
// activation, not here.
func (s *Service) Bind(ctx context.Context, actor spi.Subject, slotKind, targetID, ns, version, export string) (Binding, error) {
	const op = serrors.Op("extruntime.manage.Bind")
	e := &auditEntry{Action: ActionBind, Namespace: ns, Version: version, Target: slotKind + ":" + targetID, Detail: map[string]any{"export": export}}
	var out Binding
	err := s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		active, err := versionActive(ctx, tx, ns, version)
		if err != nil {
			return err
		}
		if !active {
			return serrors.New(serrors.FailedPrecondition, "only an active version can hold a binding: "+ns+"@"+version+" is not active")
		}
		m, err := s.meta(ctx, ns, version)
		if err != nil {
			return err
		}
		var target spi.ResolvedResource
		if slotKind != SlotShell {
			var found bool
			if target, found, err = s.catalog.Resolve(ctx, targetID); err != nil {
				return err
			}
			if !found {
				return notFound("target " + targetID)
			}
		} else if targetID == "" {
			return serrors.New(serrors.Invalid, "a shell binding needs a target id")
		}
		switch slotKind {
		case SlotCommand:
			if target.Resource.Kind != manifest.ResCommand {
				return serrors.New(serrors.Invalid, targetID+" is not a command")
			}
			if ownsID(ns, targetID) {
				return serrors.New(serrors.Invalid, targetID+" is a command of "+ns+"; its own implementation is bound by activation")
			}
			ok := false
			for _, i := range m.manifest.Implementations {
				if i.Export == export && i.Implements == targetID {
					if !implMatches(i, target.Resource) {
						return serrors.New(serrors.Invalid, "export "+export+" does not match the atomicity of "+targetID)
					}
					ok = true
				}
			}
			if !ok {
				return serrors.New(serrors.Invalid, "version "+version+" has no implementation "+export+" of "+targetID)
			}
		case SlotScreen, SlotShell:
			want := manifest.ResPage
			if slotKind == SlotShell {
				want = manifest.ResShell
			}
			// A screen target is a native route ID or a package page ID; the
			// shell has no catalog identity, the newest open row wins.
			if slotKind == SlotScreen && target.Resource.Kind != manifest.ResPage && target.Resource.Kind != manifest.ResRoute {
				return serrors.New(serrors.Invalid, targetID+" is not a screen")
			}
			ok := false
			for _, r := range m.manifest.Resources {
				if r.Kind == want && r.Export == export {
					ok = true
				}
			}
			if !ok {
				return serrors.New(serrors.Invalid, "version "+version+" has no client export "+export+" of kind "+string(want))
			}
		default:
			return serrors.New(serrors.Invalid, "unknown slot kind "+slotKind)
		}
		holder, err := slotHolder(ctx, tx, slotKind, targetID)
		if err != nil {
			return err
		}
		var chain *uuid.UUID
		if holder != nil {
			if holder.Package == ns && holder.Version == version && holder.Export == export {
				return serrors.New(serrors.AlreadyExists, "that implementation already holds the slot")
			}
			if err := closeBinding(ctx, tx, holder.ID); err != nil {
				return err
			}
			if holder.Package == ns {
				if chain, err = displacedOf(ctx, tx, holder.ID); err != nil {
					return err
				}
			} else {
				id := holder.ID
				chain = &id
			}
		}
		if out, err = insertBinding(ctx, tx, slotKind, targetID, ns, version, export, actor.UserID, false); err != nil {
			return err
		}
		if chain != nil {
			if err := setDisplaced(ctx, tx, out.ID, *chain); err != nil {
				return err
			}
		}
		return nil
	})
	_ = op
	return out, err
}

// Unbind ends a replacement binding; the slot returns to the implementation
// it displaced (or to the native one when there was none).
func (s *Service) Unbind(ctx context.Context, actor spi.Subject, slotKind, targetID string) error {
	e := &auditEntry{Action: ActionUnbind, Target: slotKind + ":" + targetID}
	return s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		holder, err := slotHolder(ctx, tx, slotKind, targetID)
		if err != nil {
			return err
		}
		if holder == nil {
			return notFound("binding " + slotKind + ":" + targetID)
		}
		e.Namespace, e.Version = holder.Package, holder.Version
		if slotKind == SlotCommand && ownsID(holder.Package, targetID) {
			return serrors.New(serrors.FailedPrecondition, "the binding is the package's own implementation; deactivate the package instead")
		}
		_, err = release(ctx, tx, *holder, actor.UserID)
		return err
	})
}
