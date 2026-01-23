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

// collectHasManyAliases returns a set of aliases that are HasMany relations.
// Used to filter out relations nested under HasMany (they're handled via subqueries).
func collectHasManyAliases(relations []RelationDescriptor) map[string]bool {
	aliases := make(map[string]bool)
	for _, rel := range relations {
		if rel.GetType() == HasMany {
			aliases[rel.GetAlias()] = true
		}
	}
	return aliases
}

// buildThroughMap creates a map of alias -> through for walking ancestor chains.
func buildThroughMap(relations []RelationDescriptor) map[string]string {
	throughMap := make(map[string]string)
	for _, rel := range relations {
		if through := rel.GetThrough(); through != "" {
			throughMap[rel.GetAlias()] = through
		}
	}
	return throughMap
}

// buildFullPrefix computes the complete ancestor chain prefix for a relation.
// For deeply nested relations (e.g., Vehicle -> Person -> District -> Region),
// returns the full chain: "p__d__dr" instead of just "d__dr".
func buildFullPrefix(alias string, throughMap map[string]string) string {
	// Build the chain from alias up to root
	chain := []string{alias}
	current := throughMap[alias]

	visited := make(map[string]bool)
	for current != "" {
		if visited[current] {
			break // Prevent infinite loops
		}
		visited[current] = true
		chain = append([]string{current}, chain...) // Prepend to maintain order
		current = throughMap[current]
	}

	return strings.Join(chain, "__")
}

// hasHasManyAncestor walks the ancestor chain via Through links and returns true
// if any ancestor alias is present in hasManyAliases.
func hasHasManyAncestor(rel RelationDescriptor, hasManyAliases map[string]bool, throughMap map[string]string) bool {
	// Start with the direct through
	current := rel.GetThrough()

	// Walk up the chain
	visited := make(map[string]bool)
	for current != "" {
		// Prevent infinite loops
		if visited[current] {
			break
		}
		visited[current] = true

		// Check if this ancestor is a HasMany
		if hasManyAliases[current] {
			return true
		}

		// Move to the parent's through
		current = throughMap[current]
	}

	return false
}

// buildHasManySubquery generates a JSON_AGG subquery for a single HasMany relation.
// parentAlias is the alias of the parent table to correlate with.
func buildHasManySubquery(parentAlias string, rel RelationDescriptor) string {
	schemaAny := rel.GetSchema()
	if schemaAny == nil {
		return ""
	}

	schema, ok := schemaAny.(RelationSchema)
	if !ok {
		return ""
	}

	// Determine parent alias: use Through if set, else use the provided parentAlias
	through := rel.GetThrough()
	if through != "" {
		parentAlias = through
	}

	alias := rel.GetAlias()
	tableName := rel.TableName()
	localKey := rel.GetLocalKey()
	remoteKey := rel.GetRemoteKey()
	if remoteKey == "" {
		remoteKey = "id"
	}

	// Build fields and joins recursively (handles nested BelongsTo and HasMany)
	jsonFields, joins := buildJSONFields(schema, alias)

	joinClause := ""
	if len(joins) > 0 {
		joinClause = " " + strings.Join(joins, " ")
	}

	// Build output alias with through prefix (matches BelongsTo convention)
	var outputAlias string
	if through != "" {
		outputAlias = through + "__" + alias
	} else {
		outputAlias = alias
	}

	return fmt.Sprintf(
		"(SELECT COALESCE(JSON_AGG(json_build_object(%s)), '[]'::json) FROM %s %s%s WHERE %s.%s = %s.%s) AS %s__json",
		strings.Join(jsonFields, ", "),
		tableName,
		alias,
		joinClause,
		alias, remoteKey,
		parentAlias, localKey,
		outputAlias,
	)
}

// BuildRelationSelectColumns generates SELECT column specifications for all relations.
// mainAlias is the alias of the main table (used for top-level HasMany subqueries).
// HasMany relations generate JSON subqueries. Relations nested under HasMany are skipped
// (they're included in the parent HasMany's subquery via buildJSONFields).
func BuildRelationSelectColumns(mainAlias string, relations []RelationDescriptor) []string {
	if len(relations) == 0 {
		return nil
	}

	hasManyAliases := collectHasManyAliases(relations)
	throughMap := buildThroughMap(relations)
	var columns []string

	for _, rel := range relations {
		// Skip relations nested under HasMany at any depth (they're handled inside the HasMany subquery)
		if hasHasManyAncestor(rel, hasManyAliases, throughMap) {
			continue
		}

		// Handle HasMany relations - generate JSON subquery
		if rel.GetType() == HasMany {
			subquery := buildHasManySubquery(mainAlias, rel)
			if subquery != "" {
				columns = append(columns, subquery)
			}
			continue
		}

		// Handle manual relations first
		manual := rel.GetManual()
		if manual != nil && len(manual.Columns) > 0 {
			alias := rel.GetAlias()
			// Use full ancestor chain for prefix to support deeply nested relations
			columnPrefix := buildFullPrefix(alias, throughMap)

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
		// Use full ancestor chain for prefix to support deeply nested relations
		columnPrefix := buildFullPrefix(alias, throughMap)

		for _, field := range schema.Fields().Fields() {
			fieldName := field.Name()
			column := fmt.Sprintf("%s.%s AS %s__%s", alias, fieldName, columnPrefix, fieldName)
			columns = append(columns, column)
		}
	}

	return columns
}

// buildJSONFields recursively builds json_build_object fields for a schema.
// Returns (jsonFields, joins) where joins are needed for nested BelongsTo relations.
func buildJSONFields(schema RelationSchema, alias string) ([]string, []string) {
	fields := schema.Fields().Fields()
	jsonFields := make([]string, 0, len(fields))
	var joins []string

	// Add own fields
	for _, field := range fields {
		fieldName := field.Name()
		jsonFields = append(jsonFields, fmt.Sprintf("'%s', %s.%s", fieldName, alias, fieldName))
	}

	// Process nested relations if schema implements RelationsProvider
	if provider, ok := schema.(RelationsProvider); ok {
		for _, nested := range provider.Relations() {
			nestedAlias := alias + "_" + nested.GetAlias()

			switch nested.GetType() {
			case BelongsTo:
				nestedSchema, ok := nested.GetSchema().(RelationSchema)
				if !ok {
					continue
				}

				// Add JOIN for this BelongsTo
				remoteKey := nested.GetRemoteKey()
				if remoteKey == "" {
					remoteKey = "id"
				}
				join := fmt.Sprintf("LEFT JOIN %s %s ON %s.%s = %s.%s",
					nested.TableName(),
					nestedAlias,
					alias, nested.GetLocalKey(),
					nestedAlias, remoteKey,
				)
				joins = append(joins, join)

				// Recursively build nested fields
				nestedFields, nestedJoins := buildJSONFields(nestedSchema, nestedAlias)
				joins = append(joins, nestedJoins...)

				// Add nested json_build_object
				jsonFields = append(jsonFields,
					fmt.Sprintf("'%s', json_build_object(%s)", nested.GetAlias(), strings.Join(nestedFields, ", ")))

			case HasMany:
				// Nested HasMany - add subquery
				nestedSchema, ok := nested.GetSchema().(RelationSchema)
				if !ok {
					continue
				}
				nestedFields, nestedJoins := buildJSONFields(nestedSchema, nestedAlias)

				nestedRemoteKey := nested.GetRemoteKey()
				if nestedRemoteKey == "" {
					nestedRemoteKey = "id"
				}

				nestedSubquery := fmt.Sprintf(
					"SELECT COALESCE(JSON_AGG(json_build_object(%s)), '[]'::json) FROM %s %s %s WHERE %s.%s = %s.%s",
					strings.Join(nestedFields, ", "),
					nested.TableName(),
					nestedAlias,
					strings.Join(nestedJoins, " "),
					nestedAlias, nestedRemoteKey,
					alias, nested.GetLocalKey(),
				)
				jsonFields = append(jsonFields, fmt.Sprintf("'%s', (%s)", nested.GetAlias(), nestedSubquery))
			}
		}
	}

	return jsonFields, joins
}

// BuildHasManySubqueries generates JSON_AGG subquery SELECT columns for HasMany relations.
// mainTable is the parent table name (e.g., "insurance.persons").
// mainAlias is the parent table alias used in the query (e.g., "p").
func BuildHasManySubqueries(mainTable, mainAlias string, relations []RelationDescriptor) []string {
	if mainTable == "" || mainAlias == "" {
		return nil
	}
	if len(relations) == 0 {
		return nil
	}

	var subqueries []string //nolint:prealloc // Size unknown due to type filtering

	for _, rel := range relations {
		if rel.GetType() != HasMany {
			continue
		}

		schemaAny := rel.GetSchema()
		if schemaAny == nil {
			continue
		}

		schema, ok := schemaAny.(RelationSchema)
		if !ok {
			continue
		}

		alias := rel.GetAlias()
		tableName := rel.TableName()
		remoteKey := rel.GetRemoteKey()
		if remoteKey == "" {
			remoteKey = "id"
		}
		localKey := rel.GetLocalKey()

		// Build fields and joins recursively
		jsonFields, joins := buildJSONFields(schema, alias)

		joinClause := ""
		if len(joins) > 0 {
			joinClause = " " + strings.Join(joins, " ")
		}

		subquery := fmt.Sprintf(
			"(SELECT COALESCE(JSON_AGG(json_build_object(%s)), '[]'::json) FROM %s %s%s WHERE %s.%s = %s.%s) AS %s__json",
			strings.Join(jsonFields, ", "),
			tableName,
			alias,
			joinClause,
			alias, remoteKey,
			mainAlias, localKey,
			alias,
		)

		subqueries = append(subqueries, subquery)
	}

	return subqueries
}

// BuildRelationJoinClauses converts relations to JoinClause slice.
// HasMany relations and relations nested under HasMany are skipped - they're handled via subqueries.
func BuildRelationJoinClauses(mainTable string, relations []RelationDescriptor) []JoinClause {
	if len(relations) == 0 {
		return nil
	}

	hasManyAliases := collectHasManyAliases(relations)
	throughMap := buildThroughMap(relations)
	clauses := make([]JoinClause, 0, len(relations))

	for _, rel := range relations {
		// Skip HasMany relations - they're handled via subqueries
		if rel.GetType() == HasMany {
			continue
		}

		// Skip relations nested under HasMany at any depth (they're handled inside the HasMany subquery)
		if hasHasManyAncestor(rel, hasManyAliases, throughMap) {
			continue
		}

		through := rel.GetThrough()

		tableName := rel.TableName()
		if tableName == "" {
			continue
		}

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
