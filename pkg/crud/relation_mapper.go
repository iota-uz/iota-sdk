package crud

import (
	"context"
)

// RelationMapper wraps a Mapper[T] and handles nested relations automatically.
// It extracts child fields, calls child mappers, and attaches results to the parent.
//
// Usage:
//
//	// VehicleGroup - leaf, no relations
//	vgMapper := NewRelationMapper(baseVgMapper, nil, vgMapOwn)
//
//	// VehicleType - has VehicleGroup relation
//	vtMapper := NewRelationMapper(baseVtMapper, []Relation{
//	    {Alias: "vg", Mapper: vgMapper, SetOnParent: withGroup},
//	}, vtMapOwn)
//
//	// Vehicle - has VehicleType relation (doesn't know about VehicleGroup)
//	vMapper := NewRelationMapper(baseVMapper, []Relation{
//	    {Alias: "vt", Mapper: vtMapper, SetOnParent: withVehicleType},
//	}, vMapOwn)
type RelationMapper[T any] struct {
	inner     Mapper[T]
	relations []Relation
	mapOwn    func([]FieldValue) T
}

// NewRelationMapper creates a RelationMapper that wraps an existing mapper.
//
// Parameters:
//   - inner: the base mapper (can be nil if mapOwn handles everything)
//   - relations: child relations with Alias, Mapper (must be *RelationMapper), and SetOnParent
//   - mapOwn: function to map this entity's own fields (non-prefixed) to an entity
func NewRelationMapper[T any](
	inner Mapper[T],
	relations []Relation,
	mapOwn func([]FieldValue) T,
) *RelationMapper[T] {
	return &RelationMapper[T]{
		inner:     inner,
		relations: relations,
		mapOwn:    mapOwn,
	}
}

// ToEntity maps a single row of field values to an entity, handling nested relations.
// This is the primary method for mapping joined query results.
func (rm *RelationMapper[T]) ToEntity(ctx context.Context, allFields []FieldValue) (T, error) {
	// Extract this entity's own fields (non-prefixed)
	myFields := ExtractNonPrefixedFields(allFields)

	// Map own fields
	entity := rm.mapOwn(myFields)

	// Handle each relation
	for _, rel := range rm.relations {
		if rel.Mapper == nil || rel.SetOnParent == nil {
			continue
		}

		// Extract child's prefixed fields (includes nested prefixes for cascading)
		childFields := ExtractPrefixedFields(allFields, rel.Alias)
		if AllFieldsNull(childFields) {
			continue
		}

		// Type-assert to relationMapperAny interface
		childMapper, ok := rel.Mapper.(relationMapperAny)
		if !ok {
			continue
		}

		// Call child mapper - childFields becomes its "allFields"
		child, err := childMapper.toEntityAny(ctx, childFields)
		if err != nil {
			continue
		}

		// Attach child to parent
		entity = rel.SetOnParent(entity, child).(T)
	}

	return entity, nil
}

// ToEntities maps multiple rows, each becoming a separate entity.
// Implements Mapper[T] interface.
func (rm *RelationMapper[T]) ToEntities(ctx context.Context, rows ...[]FieldValue) ([]T, error) {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		entity, err := rm.ToEntity(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

// ToFieldValuesList delegates to the inner mapper.
// Implements Mapper[T] interface.
func (rm *RelationMapper[T]) ToFieldValuesList(ctx context.Context, entities ...T) ([][]FieldValue, error) {
	if rm.inner == nil {
		return nil, nil
	}
	return rm.inner.ToFieldValuesList(ctx, entities...)
}

// toEntityAny is the type-erased version for use in nested relation mapping.
// This allows RelationMapper[VehicleType] to be stored in Relation.Mapper
// and called from RelationMapper[Vehicle].
func (rm *RelationMapper[T]) toEntityAny(ctx context.Context, allFields []FieldValue) (any, error) {
	return rm.ToEntity(ctx, allFields)
}

// relationMapperAny is the internal interface for type-erased relation mapping.
// RelationMapper[T] implements this to enable nested cascading.
type relationMapperAny interface {
	toEntityAny(ctx context.Context, allFields []FieldValue) (any, error)
}
