package schema_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/schema"
)

// TestMemoryProvider demonstrates basic usage of the in-memory provider.
func TestMemoryProvider(t *testing.T) {
	t.Parallel()

	metadata := []schema.TableMetadata{
		{
			TableName:        "orders",
			TableDescription: "Customer orders and transactions",
			UseCases: []string{
				"Calculate total revenue",
				"Track order trends over time",
				"Identify top customers by order value",
			},
			DataQualityNotes: []string{
				"Order dates are in UTC timezone",
				"Cancelled orders have status='cancelled'",
			},
			ColumnNotes: map[string]string{
				"total_amount": "Total order value including tax and shipping",
				"status":       "One of: pending, confirmed, shipped, delivered, cancelled",
			},
			Metrics: []schema.MetricDef{
				{
					Name:       "Average Order Value",
					Formula:    "SUM(total_amount) / COUNT(DISTINCT order_id)",
					Definition: "Average revenue per order",
				},
			},
		},
		{
			TableName:        "customers",
			TableDescription: "Customer information and contact details",
			UseCases: []string{
				"Customer segmentation analysis",
				"Track customer lifetime value",
			},
		},
	}

	provider := schema.NewMemoryMetadataProvider(metadata)

	// Test GetTableMetadata
	ctx := context.Background()
	orderMeta, err := provider.GetTableMetadata(ctx, "orders")
	if err != nil {
		t.Fatalf("GetTableMetadata failed: %v", err)
	}
	if orderMeta == nil {
		t.Fatal("Expected order metadata, got nil")
	}
	if orderMeta.TableName != "orders" {
		t.Errorf("Expected table name 'orders', got '%s'", orderMeta.TableName)
	}
	if len(orderMeta.UseCases) != 3 {
		t.Errorf("Expected 3 use cases, got %d", len(orderMeta.UseCases))
	}

	// Test ListMetadata
	allMeta, err := provider.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata failed: %v", err)
	}
	if len(allMeta) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(allMeta))
	}

	// Test non-existent table
	nonExistent, err := provider.GetTableMetadata(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetTableMetadata failed: %v", err)
	}
	if nonExistent != nil {
		t.Error("Expected nil for non-existent table, got metadata")
	}
}

// TestFileProvider demonstrates file-based metadata loading.
func TestFileProvider(t *testing.T) {
	t.Parallel()

	// Create temporary directory with test metadata files
	tmpDir := t.TempDir()

	// Create orders.json
	ordersMetadata := schema.TableMetadata{
		TableName:        "orders",
		TableDescription: "Customer orders",
		UseCases:         []string{"Revenue analysis", "Order tracking"},
		Metrics: []schema.MetricDef{
			{
				Name:       "Total Revenue",
				Formula:    "SUM(total_amount)",
				Definition: "Sum of all order amounts",
			},
		},
	}
	ordersJSON, _ := json.MarshalIndent(ordersMetadata, "", "  ")
	err := os.WriteFile(filepath.Join(tmpDir, "orders.json"), ordersJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create customers.json
	customersMetadata := schema.TableMetadata{
		TableName:        "customers",
		TableDescription: "Customer information",
	}
	customersJSON, _ := json.MarshalIndent(customersMetadata, "", "  ")
	err = os.WriteFile(filepath.Join(tmpDir, "customers.json"), customersJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create provider
	provider, err := schema.NewFileMetadataProvider(tmpDir)
	if err != nil {
		t.Fatalf("NewFileMetadataProvider failed: %v", err)
	}

	// Test GetTableMetadata
	ctx := context.Background()
	orderMeta, err := provider.GetTableMetadata(ctx, "orders")
	if err != nil {
		t.Fatalf("GetTableMetadata failed: %v", err)
	}
	if orderMeta == nil {
		t.Fatal("Expected order metadata, got nil")
	}
	if orderMeta.TableDescription != "Customer orders" {
		t.Errorf("Expected 'Customer orders', got '%s'", orderMeta.TableDescription)
	}

	// Test ListMetadata
	allMeta, err := provider.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata failed: %v", err)
	}
	if len(allMeta) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(allMeta))
	}
}

// TestFileProvider_InvalidDirectory tests error handling for invalid directory.
func TestFileProvider_InvalidDirectory(t *testing.T) {
	t.Parallel()

	// Test non-existent directory
	_, err := schema.NewFileMetadataProvider("/nonexistent/directory")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}

	// Test file instead of directory
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	err = os.WriteFile(tmpFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = schema.NewFileMetadataProvider(tmpFile)
	if err == nil {
		t.Error("Expected error for file instead of directory, got nil")
	}
}
