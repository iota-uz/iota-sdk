package js

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToJS(t *testing.T) {
	type simpleStruct struct {
		Name string
		Age  int
	}

	type nestedStruct struct {
		Person      simpleStruct
		IsEmployed  bool
		PhoneNumber *string
	}

	phoneNumber := "555-1234"
	person := simpleStruct{Name: "John", Age: 30}
	nestedWithPhone := nestedStruct{
		Person:      person,
		IsEmployed:  true,
		PhoneNumber: &phoneNumber,
	}
	nestedWithoutPhone := nestedStruct{
		Person:      person,
		IsEmployed:  false,
		PhoneNumber: nil,
	}

	jsExprInstance := templ.JSExpression("myRawJS()")
	jsExprForStructField := templ.JSExpression("function(value) { return value.toFixed(2) + '%'; }")
	type jsExpressionStruct struct {
		Name      string
		Formatter templ.JSExpression
	}
	exprStructInstance := jsExpressionStruct{
		Name:      "Test",
		Formatter: jsExprForStructField,
	}

	jsExprForPtr := templ.JSExpression("pointedTo()")
	type structWithPtrToJSExpr struct {
		PtrRaw *templ.JSExpression
		Name   string
	}

	type TaggedStruct struct {
		RegularField        string
		TaggedField         string         `json:"tagged_field_renamed"`
		OmittedField        string         `json:"-"`
		OmitEmptyStr        string         `json:"omit_empty_str,omitempty"`
		OmitEmptyStrNon     string         `json:"omit_empty_str_non,omitempty"`
		OmitEmptyIntZero    int            `json:"omit_empty_int_zero,omitempty"`
		OmitEmptyIntNon     int            `json:"omit_empty_int_non,omitempty"`
		OmitEmptyBoolFalse  bool           `json:"omit_empty_bool_false,omitempty"`
		OmitEmptyBoolTrue   bool           `json:"omit_empty_bool_true,omitempty"`
		OmitEmptySliceNil   []int          `json:"omit_empty_slice_nil,omitempty"`
		OmitEmptySliceEmpty []int          `json:"omit_empty_slice_empty,omitempty"`
		OmitEmptySliceNon   []int          `json:"omit_empty_slice_non,omitempty"`
		OmitEmptyMapNil     map[string]int `json:"omit_empty_map_nil,omitempty"`
		OmitEmptyMapEmpty   map[string]int `json:"omit_empty_map_empty,omitempty"`
		OmitEmptyMapNon     map[string]int `json:"omit_empty_map_non,omitempty"` // Test with single entry
		OmitEmptyPtrNil     *string        `json:"omit_empty_ptr_nil,omitempty"`
		OmitEmptyPtrNon     *string        `json:"omit_empty_ptr_non,omitempty"`
	}
	nonEmptyStrValForTag := "value"

	type structForSliceWithJSExpr struct {
		ID   int
		Code templ.JSExpression
	}
	jsExprInSlice1 := templ.JSExpression("sliceCode1()")
	jsExprInSlice2 := templ.JSExpression("sliceCode2()")

	type StructWithOmitEmptyJSExpr struct {
		Name     string             `json:"name"`
		Callback templ.JSExpression `json:"callback,omitempty"`
		Another  string             `json:"another_field"`
	}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{name: "nil_input", input: nil, expected: "null"},
		{name: "bool_true", input: true, expected: "true"},
		{name: "bool_false", input: false, expected: "false"},
		{name: "int_number", input: 42, expected: "42"},
		{name: "float_number", input: 3.14, expected: "3.14"},
		{name: "negative_int_number", input: -5, expected: "-5"},
		{name: "negative_float_number", input: -2.71, expected: "-2.71"},
		{name: "zero_int", input: 0, expected: "0"},
		{name: "zero_float", input: 0.0, expected: "0"},
		{name: "string_simple", input: "hello", expected: "'hello'"},
		{name: "string_empty", input: "", expected: "''"},
		{name: "string_with_single_quotes", input: "hello 'world'", expected: "'hello \\'world\\''"},
		{name: "string_with_double_quotes", input: "hello \"world\"", expected: "'hello \"world\"'"},
		{name: "string_with_backticks", input: "hello `world`", expected: "'hello `world`'"},
		{name: "string_with_newlines", input: "hello\nworld", expected: "'hello\\nworld'"},
		{name: "string_with_carriage_return", input: "hello\rworld", expected: "'hello\\rworld'"},
		{name: "string_with_tab", input: "hello\tworld", expected: "'hello\\tworld'"},
		{name: "string_with_backslash", input: "hello\\world", expected: "'hello\\\\world'"},
		{name: "slice_empty_int", input: []int{}, expected: "[]"},
		{name: "slice_of_ints", input: []int{1, 2, 3}, expected: "[1,2,3]"},
		{name: "slice_of_strings", input: []string{"a", "b", "c"}, expected: "['a','b','c']"},
		{name: "slice_of_bools", input: []bool{true, false, true}, expected: "[true,false,true]"},
		{name: "slice_of_mixed_primitives_interface_with_nil", input: []interface{}{1, "two", true, 3.14, nil}, expected: "[1,\"two\",true,3.14,null]"},
		{name: "slice_of_simple_structs", input: []simpleStruct{person, {Name: "Jane", Age: 25}}, expected: "[{'Name': 'John','Age': 30},{'Name': 'Jane','Age': 25}]"},
		{name: "slice_of_pointers_to_structs_with_nil", input: []*simpleStruct{&person, nil, {Name: "Doe", Age: 99}}, expected: "[{'Name': 'John','Age': 30},null,{'Name': 'Doe','Age': 99}]"},
		{name: "slice_of_JSExpression", input: []templ.JSExpression{templ.JSExpression("fn1()"), templ.JSExpression("fn2()")}, expected: "[fn1(),fn2()]"},
		{
			name: "slice_of_structs_with_JSExpression_field",
			input: []structForSliceWithJSExpr{
				{ID: 1, Code: jsExprInSlice1},
				{ID: 2, Code: jsExprInSlice2},
			},
			expected: "[{'ID': 1,'Code': sliceCode1()},{'ID': 2,'Code': sliceCode2()}]",
		},
		{name: "map_empty", input: map[string]int{}, expected: "{}"},
		{name: "map_single_entry_string_to_int", input: map[string]int{"key1": 100}, expected: "{'key1': 100}"},
		{name: "map_single_entry_string_to_string", input: map[string]string{"message": "hello"}, expected: "{'message': 'hello'}"},
		{name: "map_single_entry_string_to_JSExpression", input: map[string]templ.JSExpression{"callback": templ.JSExpression("doAction()")}, expected: "{'callback': doAction()}"},
		{name: "map_single_entry_string_to_nil", input: map[string]interface{}{"nothing": nil}, expected: "{'nothing': null}"},
		{name: "struct_simple", input: person, expected: "{'Name': 'John','Age': 30}"},
		{name: "struct_nested_with_pointer_field_non_nil", input: nestedWithPhone, expected: "{'Person': {'Name': 'John','Age': 30},'IsEmployed': true,'PhoneNumber': '555-1234'}"},
		{name: "struct_nested_with_pointer_field_nil", input: nestedWithoutPhone, expected: "{'Person': {'Name': 'John','Age': 30},'IsEmployed': false,'PhoneNumber': null}"},
		{name: "struct_with_JSExpression_field", input: exprStructInstance, expected: "{'Name': 'Test','Formatter': function(value) { return value.toFixed(2) + '%'; }}"},
		{name: "struct_with_pointer_to_JSExpression_non_nil", input: structWithPtrToJSExpr{PtrRaw: &jsExprForPtr, Name: "TestPtr"}, expected: "{'PtrRaw': pointedTo(),'Name': 'TestPtr'}"},
		{name: "struct_with_pointer_to_JSExpression_nil", input: structWithPtrToJSExpr{PtrRaw: nil, Name: "TestNilPtr"}, expected: "{'PtrRaw': null,'Name': 'TestNilPtr'}"},
		{name: "pointer_to_nil_typed_int", input: (*int)(nil), expected: "null"},
		{name: "pointer_to_int", input: func() *int { v := 42; return &v }(), expected: "42"},
		{name: "pointer_to_string", input: func() *string { v := "hello"; return &v }(), expected: "'hello'"},
		{name: "pointer_to_bool_true", input: func() *bool { v := true; return &v }(), expected: "true"},
		{name: "pointer_to_struct_val", input: &person, expected: "{'Name': 'John','Age': 30}"},
		{name: "pointer_to_nil_struct", input: (*simpleStruct)(nil), expected: "null"},
		{name: "pointer_to_JSExpression_val", input: &jsExprInstance, expected: "myRawJS()"},
		{name: "pointer_to_nil_JSExpression", input: (*templ.JSExpression)(nil), expected: "null"},
		{name: "function_type_reference", input: func(a, b int) int { return a + b }, expected: "/* function reference: func(int, int) int */"},
		{name: "JSExpression_direct_usage", input: jsExprInstance, expected: "myRawJS()"},
		{
			name: "json_tags_all_behaviors",
			input: TaggedStruct{
				RegularField:        "val1",
				TaggedField:         "val2",
				OmittedField:        "omit_me",
				OmitEmptyStr:        "",
				OmitEmptyStrNon:     "has_val",
				OmitEmptyIntZero:    0,
				OmitEmptyIntNon:     123,
				OmitEmptyBoolFalse:  false,
				OmitEmptyBoolTrue:   true,
				OmitEmptySliceNil:   nil,
				OmitEmptySliceEmpty: []int{},
				OmitEmptySliceNon:   []int{7},
				OmitEmptyMapNil:     nil,
				OmitEmptyMapEmpty:   map[string]int{},
				OmitEmptyMapNon:     map[string]int{"singleKey": 8}, // Changed to single entry
				OmitEmptyPtrNil:     nil,
				OmitEmptyPtrNon:     &nonEmptyStrValForTag,
			},
			expected: "{'RegularField': 'val1'," +
				"'tagged_field_renamed': 'val2'," +
				"'omit_empty_str_non': 'has_val'," +
				"'omit_empty_int_non': 123," +
				"'omit_empty_bool_true': true," +
				"'omit_empty_slice_non': [7]," +
				"'omit_empty_map_non': {'singleKey': 8}," +
				"'omit_empty_ptr_non': 'value'}",
		},
		{
			name: "json_tag_hyphen_omits_field_completely",
			input: struct {
				KeepMe           string
				HideMeAbsolutely string `json:"-"`
			}{KeepMe: "visible", HideMeAbsolutely: "hidden"},
			expected: "{'KeepMe': 'visible'}",
		},
		{
			name: "json_tag_rename_field_only",
			input: struct {
				OldName string `json:"newName"`
			}{OldName: "value"},
			expected: "{'newName': 'value'}",
		},
		{
			name: "struct_with_omitempty_jsexpr_empty",
			input: StructWithOmitEmptyJSExpr{
				Name:     "Test1",
				Callback: templ.JSExpression(""),
				Another:  "Present",
			},
			expected: "{'name': 'Test1','another_field': 'Present'}", // Field names from json tags
		},
		{
			name: "struct_with_omitempty_jsexpr_non_empty",
			input: StructWithOmitEmptyJSExpr{
				Name:     "Test2",
				Callback: templ.JSExpression("myFunc()"),
				Another:  "AlsoPresent",
			},
			expected: "{'name': 'Test2','callback': myFunc(),'another_field': 'AlsoPresent'}", // Field names from json tags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToJS(tt.input)
			require.NoError(t, err, "ToJS failed for test: %s", tt.name)

			assert.Equal(t, tt.expected, result, "Output mismatch for test: %s", tt.name)
		})
	}
}
