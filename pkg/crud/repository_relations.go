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

// RelationsProvider is implemented by schemas that declare their own relations.
// Used by BuildRelationsRecursive to discover nested relations.
type RelationsProvider interface {
	Relations() []RelationDescriptor
}

// BuildRelationsRecursive discovers all nested relations by walking the schema tree.
// For each relation, if its Schema implements RelationsProvider, those nested relations
// are also included with their Through field set to the parent alias.
func BuildRelationsRecursive(relations []RelationDescriptor) []RelationDescriptor {
	if len(relations) == 0 {
		return nil
	}

	var result []RelationDescriptor
	visited := make(map[string]bool)

	var discover func(parentAlias string, rels []RelationDescriptor)
	discover = func(parentAlias string, rels []RelationDescriptor) {
		for _, rel := range rels {
			alias := rel.GetAlias()
			key := parentAlias + "." + alias
			if visited[key] {
				continue
			}
			visited[key] = true

			// For nested relations, we need to create a copy with Through set
			// Since RelationDescriptor is an interface, we create a wrapper
			if parentAlias != "" {
				result = append(result, &relationWithThrough{rel, parentAlias})
			} else {
				result = append(result, rel)
			}

			// Recursively discover from child schema if it provides relations
			schema := rel.GetSchema()
			if schema != nil {
				if provider, ok := schema.(RelationsProvider); ok {
					discover(alias, provider.Relations())
				}
			}
		}
	}

	discover("", relations)
	return result
}

// relationWithThrough wraps a RelationDescriptor to override Through.
type relationWithThrough struct {
	RelationDescriptor
	through string
}

func (r *relationWithThrough) GetThrough() string { return r.through }

// prefixedField wraps a Field with a different name (prefix stripped).
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
func ExtractPrefixedFields(fvs []FieldValue, prefix string) []FieldValue {
	if len(fvs) == 0 {
		return nil
	}

	fullPrefix := prefix + "__"
	result := make([]FieldValue, 0)

	for _, fv := range fvs {
		fieldName := fv.Field().Name()

		if !strings.HasPrefix(fieldName, fullPrefix) {
			continue
		}

		remainder := strings.TrimPrefix(fieldName, fullPrefix)

		wrappedField := &prefixedField{
			Field: fv.Field(),
			name:  remainder,
		}

		result = append(result, &fieldValue{
			field: wrappedField,
			value: fv.Value(),
		})
	}

	return result
}

// ExtractNonPrefixedFields returns FieldValues where field name contains no "__" separator.
func ExtractNonPrefixedFields(fvs []FieldValue) []FieldValue {
	if len(fvs) == 0 {
		return nil
	}

	result := make([]FieldValue, 0)

	for _, fv := range fvs {
		fieldName := fv.Field().Name()

		if strings.Contains(fieldName, "__") {
			continue
		}

		result = append(result, fv)
	}

	return result
}

// AllFieldsNull returns true if all FieldValue.Value() return nil or IsZero().
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
func TopologicalSortRelations(relations []RelationDescriptor) []RelationDescriptor {
	if len(relations) == 0 {
		return nil
	}

	aliasToRelation := make(map[string]RelationDescriptor)
	for _, rel := range relations {
		aliasToRelation[rel.GetAlias()] = rel
	}

	visited := make(map[string]bool)
	result := make([]RelationDescriptor, 0, len(relations))

	var visit func(alias string)
	visit = func(alias string) {
		if visited[alias] {
			return
		}
		visited[alias] = true

		rel, exists := aliasToRelation[alias]
		if !exists {
			return
		}

		through := rel.GetThrough()
		if through != "" {
			visit(through)
		}

		result = append(result, rel)
	}

	for _, rel := range relations {
		visit(rel.GetAlias())
	}

	return result
}

// BuildRelationSelectColumns generates SELECT column specifications for all relations.
func BuildRelationSelectColumns(relations []RelationDescriptor) []string {
	if len(relations) == 0 {
		return nil
	}

	var columns []string

	for _, rel := range relations {
		// Handle manual relations first
		manual := rel.GetManual()
		if manual != nil && len(manual.Columns) > 0 {
			alias := rel.GetAlias()
			through := rel.GetThrough()

			var columnPrefix string
			if through != "" {
				columnPrefix = through + "__" + alias
			} else {
				columnPrefix = alias
			}

			for _, col := range manual.Columns {
				if strings.Contains(col, " AS ") || strings.Contains(col, " as ") {
					columns = append(columns, col)
				} else {
					columns = append(columns, fmt.Sprintf("%s.%s AS %s__%s", alias, col, columnPrefix, col))
				}
			}
			continue
		}

		// Schema-based relations
		schemaAny := rel.GetSchema()
		if schemaAny == nil {
			continue
		}

		schema, ok := schemaAny.(RelationSchema)
		if !ok {
			continue
		}

		alias := rel.GetAlias()
		through := rel.GetThrough()

		var columnPrefix string
		if through != "" {
			columnPrefix = through + "__" + alias
		} else {
			columnPrefix = alias
		}

		for _, field := range schema.Fields().Fields() {
			fieldName := field.Name()
			column := fmt.Sprintf("%s.%s AS %s__%s", alias, fieldName, columnPrefix, fieldName)
			columns = append(columns, column)
		}
	}

	return columns
}

// BuildRelationJoinClauses converts relations to JoinClause slice.
func BuildRelationJoinClauses(mainTable string, relations []RelationDescriptor) []JoinClause {
	if len(relations) == 0 {
		return nil
	}

	clauses := make([]JoinClause, 0, len(relations))

	for _, rel := range relations {
		tableName := rel.TableName()
		if tableName == "" {
			continue
		}

		through := rel.GetThrough()
		var leftTable string
		if through != "" {
			leftTable = through
		} else {
			leftTable = mainTable
		}

		remoteKey := rel.GetRemoteKey()
		if remoteKey == "" {
			remoteKey = "id"
		}

		clause := JoinClause{
			Type:        rel.GetJoinType(),
			Table:       tableName,
			TableAlias:  rel.GetAlias(),
			LeftColumn:  fmt.Sprintf("%s.%s", leftTable, rel.GetLocalKey()),
			RightColumn: fmt.Sprintf("%s.%s", rel.GetAlias(), remoteKey),
		}

		clauses = append(clauses, clause)
	}

	return clauses
}
