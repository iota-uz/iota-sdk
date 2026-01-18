package crud

import (
	"fmt"
	"strings"
)

// RelationSchema is the interface used to extract schema metadata from type-erased
// Schema[T] values stored in Relation.Schema. All Schema[T] types satisfy this
// interface since they implement Name() and Fields().
type RelationSchema interface {
	Name() string
	Fields() Fields
}

// prefixedField wraps a Field with a different name (prefix stripped).
// This is used when extracting prefixed fields to rename them.
type prefixedField struct {
	Field
	name string
}

func (pf *prefixedField) Name() string {
	return pf.name
}

func (pf *prefixedField) Value(value any) FieldValue {
	return &fieldValue{
		field: pf,
		value: value,
	}
}

// ExtractPrefixedFields filters FieldValues where field name starts with prefix + "__"
// and returns new FieldValues with the prefix stripped from field names.
//
// Example: prefix="vt", field "vt__id" returns FieldValue with field named "id"
func ExtractPrefixedFields(fvs []FieldValue, prefix string) []FieldValue {
	if len(fvs) == 0 {
		return nil
	}

	fullPrefix := prefix + "__"
	result := make([]FieldValue, 0)

	for _, fv := range fvs {
		fieldName := fv.Field().Name()

		// Check if field starts with the prefix
		if !strings.HasPrefix(fieldName, fullPrefix) {
			continue
		}

		// Get the part after the prefix
		remainder := strings.TrimPrefix(fieldName, fullPrefix)

		// For nested prefixes like "vt__vg__id" when extracting with prefix "vt",
		// we should only get fields that have exactly one level after the prefix.
		// "vt__id" -> "id" (no more __)
		// "vt__vg__id" -> "vg__id" (has more __, so skip for "vt" prefix)
		if strings.Contains(remainder, "__") {
			continue
		}

		// Create a wrapper field with the stripped name
		wrappedField := &prefixedField{
			Field: fv.Field(),
			name:  remainder,
		}

		// Create a new FieldValue with the wrapped field
		result = append(result, &fieldValue{
			field: wrappedField,
			value: fv.Value(),
		})
	}

	return result
}

// ExtractNonPrefixedFields returns FieldValues where field name contains no "__" separator.
// These are the parent entity's own fields (not from joined relations).
func ExtractNonPrefixedFields(fvs []FieldValue) []FieldValue {
	if len(fvs) == 0 {
		return nil
	}

	result := make([]FieldValue, 0)

	for _, fv := range fvs {
		fieldName := fv.Field().Name()

		// If the field name contains "__", it's a prefixed field from a join
		if strings.Contains(fieldName, "__") {
			continue
		}

		result = append(result, fv)
	}

	return result
}

// AllFieldsNull returns true if all FieldValue.Value() return nil or IsZero().
// Used to detect NULL relations (LEFT JOIN with no match).
func AllFieldsNull(fvs []FieldValue) bool {
	if len(fvs) == 0 {
		return true
	}

	for _, fv := range fvs {
		if !fv.IsZero() {
			return false
		}
	}

	return true
}

// TopologicalSortRelations sorts relations so dependencies (Through) come before dependents.
// Relations with no Through come first.
// Returns sorted slice for bottom-up processing.
func TopologicalSortRelations(relations []Relation) []Relation {
	if len(relations) == 0 {
		return nil
	}

	// Build a map from alias to relation for quick lookup
	aliasToRelation := make(map[string]Relation)
	for _, rel := range relations {
		aliasToRelation[rel.Alias] = rel
	}

	// Track visited and result
	visited := make(map[string]bool)
	result := make([]Relation, 0, len(relations))

	// Recursive function to add relation and its dependencies
	var visit func(alias string)
	visit = func(alias string) {
		if visited[alias] {
			return
		}

		rel, exists := aliasToRelation[alias]
		if !exists {
			return
		}

		// If this relation depends on another, visit that first
		if rel.Through != "" {
			visit(rel.Through)
		}

		visited[alias] = true
		result = append(result, rel)
	}

	// Visit all relations in their original order
	for _, rel := range relations {
		visit(rel.Alias)
	}

	return result
}

// BuildRelationSelectColumns generates SELECT column specifications for all relations.
// Each relation's schema fields are prefixed with the relation alias.
//
// Example:
//
//	relations := []Relation{{Alias: "vt", Schema: vehicleTypeSchema}}
//	columns := BuildRelationSelectColumns(relations)
//	// Returns: ["vt.id AS vt__id", "vt.name AS vt__name"]
//
// For nested relations (with Through), the alias is used directly (not nested):
//
//	relations := []Relation{
//	    {Alias: "vt", Schema: vtSchema},
//	    {Alias: "vg", Through: "vt", Schema: vgSchema},
//	}
//	// Returns: ["vt.id AS vt__id", ..., "vg.id AS vg__id", ...]
func BuildRelationSelectColumns(relations []Relation) []string {
	if len(relations) == 0 {
		return nil
	}

	var columns []string

	for _, rel := range relations {
		// Skip relations with nil schema
		if rel.Schema == nil {
			continue
		}

		// Type-assert to RelationSchema to access metadata
		schema, ok := rel.Schema.(RelationSchema)
		if !ok {
			continue
		}

		// Get all field names from the schema
		for _, field := range schema.Fields().Fields() {
			fieldName := field.Name()
			// Format: alias.field AS alias__field
			column := fmt.Sprintf("%s.%s AS %s__%s", rel.Alias, fieldName, rel.Alias, fieldName)
			columns = append(columns, column)
		}
	}

	return columns
}

// BuildRelationJoinClauses converts relations to JoinClause slice for use with the Join system.
//
// For relations without Through, the join is from the main table:
//
//	{Alias: "vt", LocalKey: "vehicle_type_id"} ->
//	JOIN vehicle_types vt ON mainTable.vehicle_type_id = vt.id
//
// For relations with Through, the join is from the Through table's alias:
//
//	{Alias: "vg", LocalKey: "group_id", Through: "vt"} ->
//	JOIN vehicle_groups vg ON vt.group_id = vg.id
func BuildRelationJoinClauses(mainTable string, relations []Relation) []JoinClause {
	if len(relations) == 0 {
		return nil
	}

	var clauses []JoinClause

	for _, rel := range relations {
		// Skip relations with nil schema
		if rel.Schema == nil {
			continue
		}

		// Type-assert to RelationSchema to get table name
		schema, ok := rel.Schema.(RelationSchema)
		if !ok {
			continue
		}

		// Determine the left side of the join
		var leftTable string
		if rel.Through != "" {
			// Join from the Through table's alias
			leftTable = rel.Through
		} else {
			// Join from the main table
			leftTable = mainTable
		}

		// Default RemoteKey to "id" if not specified
		remoteKey := rel.RemoteKey
		if remoteKey == "" {
			remoteKey = "id"
		}

		clause := JoinClause{
			Type:        rel.JoinType,
			Table:       schema.Name(),
			TableAlias:  rel.Alias,
			LeftColumn:  fmt.Sprintf("%s.%s", leftTable, rel.LocalKey),
			RightColumn: fmt.Sprintf("%s.%s", rel.Alias, remoteKey),
		}

		clauses = append(clauses, clause)
	}

	return clauses
}
