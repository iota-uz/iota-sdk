package context_test

import (
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
)

func TestSystemCodec(t *testing.T) {
	t.Parallel()

	codec := codecs.NewSystemRulesCodec()

	t.Run("ID and Version", func(t *testing.T) {
		if codec.ID() != "system-rules" {
			t.Errorf("Expected ID 'system-rules', got '%s'", codec.ID())
		}
		if codec.Version() == "" {
			t.Error("Expected non-empty version")
		}
	})

	t.Run("Validate valid payload", func(t *testing.T) {
		tests := []struct {
			name    string
			payload interface{}
		}{
			{
				name:    "string payload",
				payload: "You are a helpful assistant",
			},
			{
				name: "struct payload",
				payload: codecs.SystemRulesPayload{
					Text: "System rules here",
				},
			},
			{
				name: "map payload",
				payload: map[string]interface{}{
					"text": "Rules from map",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := codec.Validate(tt.payload)
				if err != nil {
					t.Errorf("Expected no error for valid payload, got: %v", err)
				}
			})
		}
	})

	t.Run("Validate invalid payload", func(t *testing.T) {
		tests := []struct {
			name    string
			payload interface{}
		}{
			{
				name:    "empty string",
				payload: "",
			},
			{
				name: "empty struct",
				payload: codecs.SystemRulesPayload{
					Text: "",
				},
			},
			{
				name: "empty map",
				payload: map[string]interface{}{
					"text": "",
				},
			},
			{
				name:    "invalid type",
				payload: 123,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := codec.Validate(tt.payload)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})

	t.Run("Canonicalize", func(t *testing.T) {
		payload := "  System   rules  with  extra   spaces  "
		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		// Canonical form should be valid JSON
		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical form")
		}

		// Should be deterministic
		canonical2, _ := codec.Canonicalize(payload)
		if string(canonical) != string(canonical2) {
			t.Error("Canonicalize should be deterministic")
		}
	})
}

func TestHistoryCodec(t *testing.T) {
	t.Parallel()

	codec := codecs.NewConversationHistoryCodec()

	t.Run("ID and Version", func(t *testing.T) {
		if codec.ID() != "conversation-history" {
			t.Errorf("Expected ID 'conversation-history', got '%s'", codec.ID())
		}
		if codec.Version() == "" {
			t.Error("Expected non-empty version")
		}
	})

	t.Run("Validate valid payload", func(t *testing.T) {
		validPayload := codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			Summary: "A friendly greeting",
		}

		err := codec.Validate(validPayload)
		if err != nil {
			t.Errorf("Expected no error for valid payload, got: %v", err)
		}
	})

	t.Run("Validate invalid payloads", func(t *testing.T) {
		tests := []struct {
			name    string
			payload interface{}
		}{
			{
				name: "empty messages",
				payload: codecs.ConversationHistoryPayload{
					Messages: []codecs.ConversationMessage{},
				},
			},
			{
				name: "message missing role",
				payload: codecs.ConversationHistoryPayload{
					Messages: []codecs.ConversationMessage{
						{Role: "", Content: "Hello"},
					},
				},
			},
			{
				name: "message missing content",
				payload: codecs.ConversationHistoryPayload{
					Messages: []codecs.ConversationMessage{
						{Role: "user", Content: ""},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := codec.Validate(tt.payload)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})

	t.Run("Canonicalize", func(t *testing.T) {
		payload := codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "  Hello  world  "},
				{Role: "assistant", Content: "Hi!"},
			},
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical form")
		}

		// Should be deterministic
		canonical2, _ := codec.Canonicalize(payload)
		if string(canonical) != string(canonical2) {
			t.Error("Canonicalize should be deterministic")
		}
	})
}

func TestSchemaCodec(t *testing.T) {
	t.Parallel()

	codec := codecs.NewDatabaseSchemaCodec()

	t.Run("ID and Version", func(t *testing.T) {
		if codec.ID() != "database-schema" {
			t.Errorf("Expected ID 'database-schema', got '%s'", codec.ID())
		}
		if codec.Version() == "" {
			t.Error("Expected non-empty version")
		}
	})

	t.Run("Validate valid payload", func(t *testing.T) {
		validPayload := codecs.DatabaseSchemaPayload{
			SchemaName: "public",
			Tables: []codecs.TableSchema{
				{
					Name:        "users",
					Description: "User accounts",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "integer", Nullable: false},
						{Name: "name", Type: "text", Nullable: true},
						{Name: "email", Type: "text", Nullable: false},
					},
				},
				{
					Name: "posts",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "integer", Nullable: false},
						{Name: "user_id", Type: "integer", Nullable: false},
						{Name: "title", Type: "text", Nullable: false},
					},
				},
			},
		}

		err := codec.Validate(validPayload)
		if err != nil {
			t.Errorf("Expected no error for valid payload, got: %v", err)
		}
	})

	t.Run("Validate invalid payloads", func(t *testing.T) {
		tests := []struct {
			name    string
			payload interface{}
		}{
			{
				name: "empty schema name",
				payload: codecs.DatabaseSchemaPayload{
					SchemaName: "",
					Tables: []codecs.TableSchema{
						{Name: "test", Columns: []codecs.TableColumn{{Name: "id", Type: "int"}}},
					},
				},
			},
			{
				name: "no tables",
				payload: codecs.DatabaseSchemaPayload{
					SchemaName: "public",
					Tables:     []codecs.TableSchema{},
				},
			},
			{
				name: "table missing name",
				payload: codecs.DatabaseSchemaPayload{
					SchemaName: "public",
					Tables: []codecs.TableSchema{
						{Name: "", Columns: []codecs.TableColumn{{Name: "id", Type: "int"}}},
					},
				},
			},
			{
				name: "table missing columns",
				payload: codecs.DatabaseSchemaPayload{
					SchemaName: "public",
					Tables: []codecs.TableSchema{
						{Name: "users", Columns: []codecs.TableColumn{}},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := codec.Validate(tt.payload)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})

	t.Run("Canonicalize", func(t *testing.T) {
		payload := codecs.DatabaseSchemaPayload{
			SchemaName: "public",
			Tables: []codecs.TableSchema{
				{
					Name:        "users",
					Description: "  User  table  ",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "integer", Nullable: false},
					},
				},
			},
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical form")
		}

		// Should be deterministic
		canonical2, _ := codec.Canonicalize(payload)
		if string(canonical) != string(canonical2) {
			t.Error("Canonicalize should be deterministic")
		}
	})
}

func TestQueryCodec(t *testing.T) {
	t.Parallel()

	t.Run("Default max rows", func(t *testing.T) {
		codec := codecs.NewQueryResultCodec()

		if codec.ID() != "query-result" {
			t.Errorf("Expected ID 'query-result', got '%s'", codec.ID())
		}
	})

	t.Run("Custom max rows", func(t *testing.T) {
		codec := codecs.NewQueryResultCodec(codecs.WithMaxRows(50))

		// Create payload with more rows than max
		payload := codecs.QueryResultPayload{
			Query:   "SELECT * FROM users",
			Columns: []string{"id", "name"},
			Rows:    make([][]interface{}, 100), // 100 rows
		}

		// Initialize rows
		for i := 0; i < 100; i++ {
			payload.Rows[i] = []interface{}{i, "user" + string(rune(i))}
		}

		// Canonicalize should truncate
		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		if len(canonical) == 0 {
			t.Error("Expected non-empty canonical form")
		}
	})

	t.Run("Validate valid payload", func(t *testing.T) {
		codec := codecs.NewQueryResultCodec()

		validPayload := codecs.QueryResultPayload{
			Query:      "SELECT id, name FROM users WHERE active = true",
			Columns:    []string{"id", "name"},
			Rows:       [][]interface{}{{1, "Alice"}, {2, "Bob"}},
			RowCount:   2,
			ExecutedAt: "2024-01-01T00:00:00Z",
		}

		err := codec.Validate(validPayload)
		if err != nil {
			t.Errorf("Expected no error for valid payload, got: %v", err)
		}
	})

	t.Run("Validate invalid payloads", func(t *testing.T) {
		codec := codecs.NewQueryResultCodec()

		tests := []struct {
			name    string
			payload interface{}
		}{
			{
				name: "empty query",
				payload: codecs.QueryResultPayload{
					Query:   "",
					Columns: []string{"id"},
					Rows:    [][]interface{}{},
				},
			},
			{
				name: "no columns",
				payload: codecs.QueryResultPayload{
					Query:   "SELECT * FROM users",
					Columns: []string{},
					Rows:    [][]interface{}{},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := codec.Validate(tt.payload)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})

	t.Run("Truncation with max rows", func(t *testing.T) {
		codec := codecs.NewQueryResultCodec(codecs.WithMaxRows(10))

		// Create payload with 50 rows
		rows := make([][]interface{}, 50)
		for i := 0; i < 50; i++ {
			rows[i] = []interface{}{i, "data"}
		}

		payload := codecs.QueryResultPayload{
			Query:   "SELECT * FROM large_table",
			Columns: []string{"id", "data"},
			Rows:    rows,
		}

		canonical, err := codec.Canonicalize(payload)
		if err != nil {
			t.Fatalf("Canonicalize failed: %v", err)
		}

		// Check that truncation is indicated in canonical form
		canonicalStr := string(canonical)
		if !strings.Contains(canonicalStr, "truncated") {
			t.Error("Expected 'truncated' field in canonical form")
		}
		if !strings.Contains(canonicalStr, "max_rows") {
			t.Error("Expected 'max_rows' field in canonical form")
		}
	})
}

func TestCodec_Deterministic(t *testing.T) {
	t.Parallel()

	// Test that canonicalization is deterministic for all codecs
	tests := []struct {
		name  string
		codec interface {
			Canonicalize(interface{}) ([]byte, error)
		}
		payload interface{}
	}{
		{
			name:    "SystemRulesCodec",
			codec:   codecs.NewSystemRulesCodec(),
			payload: "System rules",
		},
		{
			name:  "ConversationHistoryCodec",
			codec: codecs.NewConversationHistoryCodec(),
			payload: codecs.ConversationHistoryPayload{
				Messages: []codecs.ConversationMessage{
					{Role: "user", Content: "Hello"},
				},
			},
		},
		{
			name:  "DatabaseSchemaCodec",
			codec: codecs.NewDatabaseSchemaCodec(),
			payload: codecs.DatabaseSchemaPayload{
				SchemaName: "public",
				Tables: []codecs.TableSchema{
					{Name: "test", Columns: []codecs.TableColumn{{Name: "id", Type: "int"}}},
				},
			},
		},
		{
			name:  "QueryResultCodec",
			codec: codecs.NewQueryResultCodec(),
			payload: codecs.QueryResultPayload{
				Query:   "SELECT 1",
				Columns: []string{"result"},
				Rows:    [][]interface{}{{1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Canonicalize 10 times
			var results []string
			for i := 0; i < 10; i++ {
				canonical, err := tt.codec.Canonicalize(tt.payload)
				if err != nil {
					t.Fatalf("Canonicalize failed: %v", err)
				}
				results = append(results, string(canonical))
			}

			// All results should be identical
			for i := 1; i < len(results); i++ {
				if results[i] != results[0] {
					t.Errorf("Canonicalize not deterministic: result %d differs from result 0", i)
				}
			}
		})
	}
}
