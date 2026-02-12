package codecs_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
)

func TestSchemaMetadataCodec(t *testing.T) {
	t.Parallel()

	codec := codecs.NewSchemaMetadataCodec()

	// Test basic validation
	t.Run("Validate_ValidPayload", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{
					TableName:        "orders",
					TableDescription: "Customer orders",
				},
			},
		}

		err := codec.Validate(payload)
		if err != nil {
			t.Errorf("Expected validation to pass, got error: %v", err)
		}
	})

	t.Run("Validate_ValidSlicePayload", func(t *testing.T) {
		payload := []schema.TableMetadata{
			{
				TableName:        "orders",
				TableDescription: "Customer orders",
			},
		}

		err := codec.Validate(payload)
		if err != nil {
			t.Errorf("Expected validation to pass, got error: %v", err)
		}
	})

	t.Run("Validate_EmptyTables", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{},
		}

		err := codec.Validate(payload)
		if err == nil {
			t.Error("Expected validation to fail for empty tables, got nil")
		}
	})

	t.Run("Validate_MissingTableName", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{
					TableDescription: "Missing table name",
				},
			},
		}

		err := codec.Validate(payload)
		if err == nil {
			t.Error("Expected validation to fail for missing table name, got nil")
		}
	})

	// Test canonicalization
	t.Run("Canonicalize_ValidPayload", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{
					TableName:        "orders",
					TableDescription: "Customer orders",
					UseCases:         []string{"Revenue analysis"},
				},
			},
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical bytes")
		}
	})

	t.Run("Canonicalize_SortsByTableName", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{TableName: "zebra", TableDescription: "Z table"},
				{TableName: "apple", TableDescription: "A table"},
				{TableName: "mango", TableDescription: "M table"},
			},
		}

		canonical1, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		// Same payload in different order should produce same canonical form
		payloadReversed := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{TableName: "mango", TableDescription: "M table"},
				{TableName: "apple", TableDescription: "A table"},
				{TableName: "zebra", TableDescription: "Z table"},
			},
		}

		canonical2, err := codec.Canonicalize(payloadReversed)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if string(canonical1) != string(canonical2) {
			t.Error("Expected same canonical form for differently ordered tables")
		}
	})

	t.Run("Canonicalize_SlicePayload", func(t *testing.T) {
		payload := []schema.TableMetadata{
			{
				TableName:        "orders",
				TableDescription: "Customer orders",
			},
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical bytes")
		}
	})

	t.Run("Canonicalize_WithMetrics", func(t *testing.T) {
		payload := codecs.SchemaMetadataPayload{
			Tables: []schema.TableMetadata{
				{
					TableName:        "orders",
					TableDescription: "Customer orders",
					Metrics: []schema.MetricDef{
						{
							Name:       "Total Revenue",
							Formula:    "SUM(total_amount)",
							Definition: "Sum of all orders",
						},
					},
				},
			},
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical bytes")
		}
	})
}
