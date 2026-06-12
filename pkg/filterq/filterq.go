// Package filterq provides a generic, UI-agnostic filter model for
// GitHub-Projects / Linear style filter builders: a FilterSet is an ordered
// list of conditions (field, operator, values) combined with AND between
// conditions and OR between values of a single condition.
//
// The package also ships a deterministic, human-readable URL codec
// (see codec.go) so a FilterSet round-trips through query parameters and
// links stay shareable.
package filterq

// FieldType describes the value domain of a filterable field and determines
// which operators and value encodings are valid for it.
type FieldType string

const (
	// FieldTypeReference is a dictionary-backed field (IDs, codes, enums).
	FieldTypeReference FieldType = "reference"
	// FieldTypeDate is a day-granular date field (values are YYYY-MM-DD or presets).
	FieldTypeDate FieldType = "date"
	// FieldTypeNumber is a numeric field (integers or decimals).
	FieldTypeNumber FieldType = "number"
	// FieldTypeBool is a flag field (values are "true" / "false").
	FieldTypeBool FieldType = "bool"
)

// Operator is a comparison operator applied to a condition.
type Operator string

const (
	OpIs      Operator = "is"
	OpIsNot   Operator = "isnot"
	OpBefore  Operator = "before"
	OpAfter   Operator = "after"
	OpOn      Operator = "on"
	OpBetween Operator = "between"
	OpEq      Operator = "eq"
	OpGt      Operator = "gt"
	OpLt      Operator = "lt"
)

// Arity returns the number of values the operator expects:
// -1 means one-or-more (OR list), otherwise the exact count.
// OpBetween additionally accepts a single symbolic preset value for date
// fields (see DatePreset); the codec handles that special case.
func (o Operator) Arity() int {
	switch o {
	case OpIs, OpIsNot:
		return -1
	case OpBetween:
		return 2
	default:
		return 1
	}
}

// Valid reports whether o is a known operator.
func (o Operator) Valid() bool {
	switch o {
	case OpIs, OpIsNot, OpBefore, OpAfter, OpOn, OpBetween, OpEq, OpGt, OpLt:
		return true
	}
	return false
}

// DefaultOperators returns the canonical operator set for a field type.
func DefaultOperators(t FieldType) []Operator {
	switch t {
	case FieldTypeReference:
		return []Operator{OpIs, OpIsNot}
	case FieldTypeDate:
		return []Operator{OpOn, OpBefore, OpAfter, OpBetween}
	case FieldTypeNumber:
		return []Operator{OpEq, OpGt, OpLt, OpBetween}
	case FieldTypeBool:
		return []Operator{OpIs}
	}
	return nil
}

// Condition is a single filter: <field> <operator> <values>.
// Multiple values are OR-ed (e.g. status is A or B).
type Condition struct {
	Field  string
	Op     Operator
	Values []string
}

// FilterSet is an ordered list of conditions AND-ed together.
type FilterSet []Condition

// Field returns all conditions for the given field key.
func (fs FilterSet) Field(key string) []Condition {
	var out []Condition
	for _, c := range fs {
		if c.Field == key {
			out = append(out, c)
		}
	}
	return out
}

// Has reports whether any condition targets the given field key.
func (fs FilterSet) Has(key string) bool {
	for _, c := range fs {
		if c.Field == key {
			return true
		}
	}
	return false
}

// Without returns a copy of the set with all conditions on the given field removed.
func (fs FilterSet) Without(key string) FilterSet {
	out := make(FilterSet, 0, len(fs))
	for _, c := range fs {
		if c.Field != key {
			out = append(out, c)
		}
	}
	return out
}

// IsZero reports whether the set has no conditions.
func (fs FilterSet) IsZero() bool { return len(fs) == 0 }
