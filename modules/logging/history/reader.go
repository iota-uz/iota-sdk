package history

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
)

// ForReader hides the field values a reader may not see. Values of fields the registry no
// longer describes are hidden as well, so a missing definition never discloses data.
func (r *Registry) ForReader(e businessevent.Event, can func(permission.Permission) bool) businessevent.Event {
	def, known := r.Get(e.Type, e.SchemaVersion)
	changes := make([]businessevent.Change, len(e.Changes))
	for i, c := range e.Changes {
		resource, _ := def.resource(c.Resource)
		fields := make([]fielddiff.Diff, len(c.Fields))
		for j, d := range c.Fields {
			field, ok := resource.field(d.Field)
			if !known || !ok || (field.ReadPermission != nil && !can(*field.ReadPermission)) {
				d.Before, d.After = hide(d.Before), hide(d.After)
			}
			fields[j] = d
		}
		changes[i] = businessevent.Change{Resource: c.Resource, ID: c.ID, Fields: fields}
	}
	e.Changes = changes
	return e
}

func hide(v fielddiff.FieldValue) fielddiff.FieldValue {
	if v.State == fielddiff.StateValue {
		return fielddiff.Redacted()
	}
	return v
}
