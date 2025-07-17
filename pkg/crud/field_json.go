package crud

import (
	"encoding/json"
	"fmt"
)

// JsonFieldData is the base interface for all JSON field data types
type JsonFieldData interface {
	ToJSON() (string, error)
	FromJSON(jsonStr string) error
	ValidateData() error
}

// JsonSchemaTypeInterface defines the interface for specific JSON schema implementations
type JsonSchemaTypeInterface interface {
	// CreateData creates a new instance of the schema-specific data type
	CreateData() JsonFieldData

	// FormatJSON formats data according to the schema's requirements
	FormatJSON(data interface{}) (string, error)

	// ParseJSON parses JSON string into the schema-specific format
	ParseJSON(input string) (interface{}, error)

	// ValidateJSON validates JSON according to the schema's rules
	ValidateJSON(input string) error
}

type JsonField interface {
	Field

	// Basic JSON methods
	SchemaType() string
	CreateData() JsonFieldData
	FormatJSON(data interface{}) (string, error)
	ParseJSON(input string) (interface{}, error)
	ValidateJSON(input string) error
}

// Global registry for JSON schema types
var jsonSchemaRegistry = make(map[string]JsonSchemaTypeInterface)

// RegisterJsonSchemaType registers a new JSON schema type
func RegisterJsonSchemaType(name string, schemaType JsonSchemaTypeInterface) {
	jsonSchemaRegistry[name] = schemaType
}

// GetJsonSchemaType retrieves a registered JSON schema type
func GetJsonSchemaType(name string) (JsonSchemaTypeInterface, bool) {
	schemaType, exists := jsonSchemaRegistry[name]
	return schemaType, exists
}

func NewJsonField(
	name string,
	opts ...FieldOption,
) JsonField {
	f := newField(
		name,
		JsonFieldType,
		opts...,
	).(*field)

	return &jsonField{field: f}
}

type jsonField struct {
	*field
}

func (j *jsonField) SchemaType() string {
	if val, ok := j.attrs[JsonSchemaType].(string); ok {
		return val
	}
	return ""
}

func (j *jsonField) AsJsonField() (JsonField, error) {
	return j, nil
}

func (j *jsonField) Value(value any) FieldValue {
	if !isValidType(j.Type(), value) {
		panic(fmt.Sprintf(
			"invalid type for field %q: expected %s, got %T",
			j.name, j.Type(), value,
		))
	}
	return &fieldValue{
		field: j, // Use the jsonField, not the embedded field
		value: value,
	}
}

// FormatJSON Helper functions for JSON handling
func (j *jsonField) FormatJSON(data interface{}) (string, error) {
	schemaType := j.SchemaType()
	if schemaType != "" {
		if schema, exists := GetJsonSchemaType(schemaType); exists {
			return schema.FormatJSON(data)
		}
	}

	// Fallback to generic JSON formatting
	return j.compactFormat(data)
}

func (j *jsonField) compactFormat(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

func (j *jsonField) ParseJSON(input string) (interface{}, error) {
	schemaType := j.SchemaType()
	if schemaType != "" {
		if schema, exists := GetJsonSchemaType(schemaType); exists {
			return schema.ParseJSON(input)
		}
	}

	// Fallback to generic JSON parsing
	var result interface{}
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

func (j *jsonField) CreateData() JsonFieldData {
	schemaType := j.SchemaType()
	if schemaType != "" {
		if schema, exists := GetJsonSchemaType(schemaType); exists {
			return schema.CreateData()
		}
	}

	return nil
}

func (j *jsonField) ValidateJSON(input string) error {
	schemaType := j.SchemaType()
	if schemaType != "" {
		if schema, exists := GetJsonSchemaType(schemaType); exists {
			return schema.ValidateJSON(input)
		}
	}

	// Fallback to generic JSON validation
	var result interface{}
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
