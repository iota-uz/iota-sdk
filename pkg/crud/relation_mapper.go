package crud

import (
	"context"
)

// JoinableMapperAny is a type-erased version of JoinableMapper for use in generic contexts.
// Mappers that want to participate in MapWithRelations should implement this interface.
type JoinableMapperAny interface {
	ToEntitiesWithAllFieldsAny(ctx context.Context, myFields, allFields []FieldValue) ([]any, error)
}

// MapWithRelations maps an entity from field values and handles nested relations.
// This is the core helper for self-contained mappers that handle their own children.
//
// Parameters:
//   - ctx: context for the mapping operation
//   - myFields: field values for this entity (no prefix, already extracted)
//   - allFields: all field values from query (for passing to child mappers)
//   - mapOwnFields: function that maps this entity's own fields to an entity
//   - relations: this entity's declared relations (each has Mapper and SetOnParent)
//
// For each relation:
//  1. Extracts prefixed fields (e.g., "vt__*" for Alias="vt")
//  2. If not all null, calls the relation's Mapper.ToEntitiesWithAllFieldsAny
//  3. Uses SetOnParent to attach the child entity to the parent
func MapWithRelations[T any](
	ctx context.Context,
	myFields, allFields []FieldValue,
	mapOwnFields func([]FieldValue) T,
	relations []Relation,
) (T, error) {
	// Map own fields first
	entity := mapOwnFields(myFields)

	// Handle each relation
	for _, rel := range relations {
		// Skip relations without mapper or SetOnParent
		if rel.Mapper == nil || rel.SetOnParent == nil {
			continue
		}

		// Extract child's prefixed fields
		childFields := ExtractPrefixedFields(allFields, rel.Alias)
		if AllFieldsNull(childFields) {
			continue
		}

		// Type-assert mapper to JoinableMapperAny
		mapper, ok := rel.Mapper.(JoinableMapperAny)
		if !ok {
			continue
		}

		// Call child mapper
		children, err := mapper.ToEntitiesWithAllFieldsAny(ctx, childFields, allFields)
		if err != nil || len(children) == 0 {
			continue
		}

		// Attach child to parent
		entity = rel.SetOnParent(entity, children[0]).(T)
	}

	return entity, nil
}
