package crud

import (
	"testing"
)

// newTestRelMapper creates a *RelationMapper[any] for testing
func newTestRelMapper() *RelationMapper[any] {
	return NewRelationMapper[any](nil, nil, func(fvs []FieldValue) any { return nil })
}

func TestNewSchemaWithRelations(t *testing.T) {
	fields := NewFields([]Field{
		NewUUIDField("id", WithKey()),
		NewStringField("name"),
	})

	mapper := newTestRelMapper()

	relatedSchema := newMockSchemaForBuilder("related_table")
	relations := NewRelationBuilder().
		BelongsTo("r", relatedSchema).
		LocalKey("related_id").
		EntityField("related_entity").
		Build()

	schema := NewSchemaWithRelations(
		"test_table",
		fields,
		mapper,
		relations,
	)

	// Check relations are accessible
	rels := schema.Relations()
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}

	if rels[0].GetAlias() != "r" {
		t.Errorf("expected alias 'r', got %q", rels[0].GetAlias())
	}

	// Check it still implements Schema
	if schema.Name() != "test_table" {
		t.Errorf("expected name 'test_table', got %q", schema.Name())
	}
}

func TestSchemaWithRelations_EmptyRelations(t *testing.T) {
	fields := NewFields([]Field{
		NewUUIDField("id", WithKey()),
	})
	mapper := newTestRelMapper()

	schema := NewSchemaWithRelations(
		"simple_table",
		fields,
		mapper,
		[]RelationDescriptor{}, // empty relations
	)

	if len(schema.Relations()) != 0 {
		t.Errorf("expected 0 relations, got %d", len(schema.Relations()))
	}
}

func TestSchemaWithRelations_MultipleRelations(t *testing.T) {
	fields := NewFields([]Field{
		NewUUIDField("id", WithKey()),
		NewStringField("name"),
	})
	mapper := newTestRelMapper()

	relatedSchema1 := newMockSchemaForBuilder("types")
	relatedSchema2 := newMockSchemaForBuilder("categories")

	relations := NewRelationBuilder().
		BelongsTo("t", relatedSchema1).
		LocalKey("type_id").
		EntityField("type_entity").
		BelongsTo("c", relatedSchema2).
		LocalKey("category_id").
		EntityField("category_entity").
		Build()

	schema := NewSchemaWithRelations(
		"items",
		fields,
		mapper,
		relations,
	)

	rels := schema.Relations()

	if len(rels) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(rels))
	}

	if rels[0].GetAlias() != "t" {
		t.Errorf("expected first alias 't', got %q", rels[0].GetAlias())
	}
	if rels[1].GetAlias() != "c" {
		t.Errorf("expected second alias 'c', got %q", rels[1].GetAlias())
	}
}

func TestSchemaWithRelations_SchemaMethodsWork(t *testing.T) {
	fields := NewFields([]Field{
		NewUUIDField("id", WithKey()),
		NewStringField("name"),
	})
	mapper := newTestRelMapper()

	schema := NewSchemaWithRelations(
		"test_table",
		fields,
		mapper,
		[]RelationDescriptor{},
	)

	// Verify all Schema interface methods work
	if schema.Name() != "test_table" {
		t.Errorf("Name() = %q, want %q", schema.Name(), "test_table")
	}

	if schema.Fields() == nil {
		t.Error("Fields() returned nil")
	}

	if schema.Mapper() == nil {
		t.Error("Mapper() returned nil")
	}

	if schema.Validators() == nil {
		t.Error("Validators() returned nil")
	}

	if schema.Hooks() == nil {
		t.Error("Hooks() returned nil")
	}
}

func TestSchemaWithRelations_WithOptions(t *testing.T) {
	fields := NewFields([]Field{
		NewUUIDField("id", WithKey()),
	})
	mapper := newTestRelMapper()

	validatorCalled := false
	validator := func(entity any) error {
		validatorCalled = true
		return nil
	}

	schema := NewSchemaWithRelations(
		"test_table",
		fields,
		mapper,
		[]RelationDescriptor{},
		WithValidator(validator),
	)

	validators := schema.Validators()
	if len(validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(validators))
	}

	// Call the validator to ensure it was properly set
	_ = validators[0](nil)
	if !validatorCalled {
		t.Error("validator was not called")
	}
}

// Note: newMockSchemaForBuilder is defined in relation_builder_test.go and is shared across tests
