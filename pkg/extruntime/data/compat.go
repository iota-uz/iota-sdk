package data

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// CompatibleWithPinned reports whether a package version's schema view can
// still be served by the applied physical tables: every column it maps still
// exists with a compatible type, reads cannot surface nulls it does not
// expect, and writes cannot be rejected by columns it does not know. Run it
// for each pinned (running-instance) version before activating a new one.
// The error is a *SchemaError.
func CompatibleWithPinned(applied *Applied, pinned *Schema) error {
	var physical map[string]*PhysTable
	if applied != nil {
		physical = applied.Tables
	}
	return pinnedError(checkPinned(physical, pinned))
}

// CompatibleWithPinned checks pinned versions against the physical state this
// plan produces, so activation can be refused before Apply.
func (p MigrationPlan) CompatibleWithPinned(pinned ...*Schema) error {
	var diags []Diagnostic
	for _, s := range pinned {
		diags = append(diags, checkPinned(p.Physical, s)...)
	}
	return pinnedError(diags)
}

func pinnedError(diags []Diagnostic) error {
	if len(diags) == 0 {
		return nil
	}
	return &SchemaError{Diagnostics: diags}
}

func checkPinned(physical map[string]*PhysTable, pinned *Schema) []Diagnostic {
	var bag diagBag
	label := pinned.Package + "@" + pinned.Version
	for _, t := range pinned.Tables {
		pt, ok := physical[t.ID]
		if !ok {
			bag.add(t.ID, "", DiagEntityDropped, "pinned version %s uses this entity but its table is no longer applied", label)
			continue
		}
		mapped := map[string]bool{}
		for _, f := range t.Fields {
			mapped[f.Column] = true
			pc, ok := pt.column(f.Column)
			if !ok {
				bag.add(t.ID, f.Key, DiagFieldDropped, "pinned version %s uses column %s which no longer exists", label, f.Column)
				continue
			}
			for _, reason := range fieldIncompat(f, pc) {
				bag.add(t.ID, f.Key, DiagIncompatiblePinned, "pinned version %s: %s", label, reason)
			}
		}
		for i := range pt.Columns {
			c := &pt.Columns[i]
			if !mapped[c.Name] && c.NotNull && c.DefaultSQL == "" {
				bag.add(t.ID, c.currentKey(), DiagRequiredNoDefault, "pinned version %s does not know required column %s, its inserts would fail", label, c.Name)
			}
		}
	}
	return bag.list
}

func fieldIncompat(f FieldSpec, pc *PhysColumn) []string {
	var out []string
	if f.Type.Kind != pc.Type.Kind {
		return []string{"type is now " + string(pc.Type.Kind) + ", pinned expects " + string(f.Type.Kind)}
	}
	if f.Ref != nil && (pc.Ref == nil || pc.Ref.Entity != f.Ref.Entity) {
		return []string{"ref target changed to " + refName(pc.Ref)}
	}
	if f.NotNull && !pc.NotNull {
		out = append(out, "column became nullable, pinned expects a value on every row")
	}
	if !f.NotNull && pc.NotNull && pc.DefaultSQL == "" {
		out = append(out, "column became required without default, pinned may omit it")
	}
	switch f.Type.Kind {
	case manifest.KindString:
		if pc.Type.MaxLength > 0 && (f.Type.MaxLength == 0 || f.Type.MaxLength > pc.Type.MaxLength) {
			out = append(out, "maxLength shrank below what pinned may write")
		}
	case manifest.KindDecimal:
		if pc.Type.Scale != f.Type.Scale || (pc.Type.Precision > 0 && pc.Type.Precision < f.Type.Precision) {
			out = append(out, "decimal precision or scale no longer holds pinned values")
		}
	case manifest.KindEnum:
		have := map[string]bool{}
		for _, v := range pc.Type.Values {
			have[v] = true
		}
		var gone []string
		for _, v := range f.Type.Values {
			if !have[v] {
				gone = append(gone, v)
			}
		}
		if len(gone) > 0 {
			out = append(out, "enum values no longer accepted: "+strings.Join(gone, ", "))
		}
	case manifest.KindInt, manifest.KindInt64, manifest.KindBool, manifest.KindDate,
		manifest.KindDateTime, manifest.KindUUID, manifest.KindJSON, manifest.KindRef,
		manifest.KindObject, manifest.KindArray:
		// No type parameter beyond the kind itself to compare.
	}
	return out
}
