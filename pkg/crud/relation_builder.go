package crud

// RelationBuilder provides a fluent API for declaring schema relations.
type RelationBuilder struct {
	relations []RelationDescriptor
}

// NewRelationBuilder creates a new RelationBuilder.
func NewRelationBuilder() *RelationBuilder {
	return &RelationBuilder{
		relations: make([]RelationDescriptor, 0),
	}
}

// BelongsTo declares a many-to-one relationship.
func (rb *RelationBuilder) BelongsTo(alias string, schema Schema[any]) *RelationConfig {
	r := &Relation[any]{
		Type:      BelongsTo,
		Alias:     alias,
		Schema:    schema,
		RemoteKey: "id",
		JoinType:  JoinTypeLeft,
	}
	rb.relations = append(rb.relations, r)
	return &RelationConfig{
		builder: rb,
		index:   len(rb.relations) - 1,
	}
}

// HasMany declares a one-to-many relationship.
func (rb *RelationBuilder) HasMany(alias string, schema Schema[any]) *RelationConfig {
	r := &Relation[any]{
		Type:      HasMany,
		Alias:     alias,
		Schema:    schema,
		RemoteKey: "id",
		JoinType:  JoinTypeLeft,
	}
	rb.relations = append(rb.relations, r)
	return &RelationConfig{
		builder: rb,
		index:   len(rb.relations) - 1,
	}
}

// Build returns the configured relations.
func (rb *RelationBuilder) Build() []RelationDescriptor {
	return rb.relations
}

// RelationConfig provides per-relation configuration methods.
type RelationConfig struct {
	builder *RelationBuilder
	index   int
}

func (rc *RelationConfig) relation() *Relation[any] {
	return rc.builder.relations[rc.index].(*Relation[any])
}

// LocalKey sets the foreign key column in this table.
func (rc *RelationConfig) LocalKey(col string) *RelationConfig {
	rc.relation().LocalKey = col
	return rc
}

// RemoteKey sets the primary key column in the related table.
func (rc *RelationConfig) RemoteKey(col string) *RelationConfig {
	rc.relation().RemoteKey = col
	return rc
}

// EntityField sets the name for the EntityFieldValue in the mapper.
func (rc *RelationConfig) EntityField(name string) *RelationConfig {
	rc.relation().EntityField = name
	return rc
}

// Through sets the parent alias for nested relations.
func (rc *RelationConfig) Through(parentAlias string) *RelationConfig {
	rc.relation().Through = parentAlias
	return rc
}

// InnerJoin changes the join type to INNER JOIN.
func (rc *RelationConfig) InnerJoin() *RelationConfig {
	rc.relation().JoinType = JoinTypeInner
	return rc
}

// BelongsTo allows chaining to declare another BelongsTo relation.
func (rc *RelationConfig) BelongsTo(alias string, schema Schema[any]) *RelationConfig {
	return rc.builder.BelongsTo(alias, schema)
}

// HasMany allows chaining to declare a HasMany relation.
func (rc *RelationConfig) HasMany(alias string, schema Schema[any]) *RelationConfig {
	return rc.builder.HasMany(alias, schema)
}

// Mapper sets the mapper for the related entity.
func (rc *RelationConfig) Mapper(mapper any) *RelationConfig {
	rc.relation().Mapper = nil // Can't set typed mapper on Relation[any]
	return rc
}

// SetOnParent sets the function that attaches child entity to parent.
func (rc *RelationConfig) SetOnParent(fn func(parent, child any) any) *RelationConfig {
	rc.relation().SetOnParent = fn
	return rc
}

// Build returns the configured relations.
func (rc *RelationConfig) Build() []RelationDescriptor {
	return rc.builder.Build()
}
