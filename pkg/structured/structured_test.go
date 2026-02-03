package structured

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SchemaFromType tests ---

func TestSchemaFromType_Simple(t *testing.T) {
	type Person struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email,omitempty"`
	}

	schema, err := SchemaFromType(Person{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "name")
	assert.Contains(t, schema.Properties, "age")
	assert.Contains(t, schema.Properties, "email")
	assert.Equal(t, "string", schema.Properties["name"].Type)
	assert.Equal(t, "integer", schema.Properties["age"].Type)
	assert.Contains(t, schema.Required, "name")
	assert.Contains(t, schema.Required, "age")
	assert.NotContains(t, schema.Required, "email")
}

func TestSchemaFromType_Nested(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type Person struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	schema, err := SchemaFromType(Person{})
	require.NoError(t, err)
	assert.Contains(t, schema.Properties, "address")
	assert.Equal(t, "object", schema.Properties["address"].Type)
	assert.Contains(t, schema.Properties["address"].Properties, "street")
}

func TestSchemaFromType_Array(t *testing.T) {
	type Container struct {
		Items []string `json:"items"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "array", schema.Properties["items"].Type)
	assert.NotNil(t, schema.Properties["items"].Items)
	assert.Equal(t, "string", schema.Properties["items"].Items.Type)
}

func TestSchemaFromType_Pointer(t *testing.T) {
	type Item struct {
		Value string `json:"value"`
	}

	schema, err := SchemaFromType(&Item{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "value")
}

func TestSchemaFromType_SkipDash(t *testing.T) {
	type Item struct {
		Public  string `json:"public"`
		Private string `json:"-"`
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	assert.Contains(t, schema.Properties, "public")
	assert.NotContains(t, schema.Properties, "Private")
}

func TestSchemaFromType_AllTypes(t *testing.T) {
	type AllTypes struct {
		S  string  `json:"s"`
		I  int     `json:"i"`
		F  float64 `json:"f"`
		B  bool    `json:"b"`
		Sl []int   `json:"sl"`
	}

	schema, err := SchemaFromType(AllTypes{})
	require.NoError(t, err)
	assert.Equal(t, "string", schema.Properties["s"].Type)
	assert.Equal(t, "integer", schema.Properties["i"].Type)
	assert.Equal(t, "number", schema.Properties["f"].Type)
	assert.Equal(t, "boolean", schema.Properties["b"].Type)
	assert.Equal(t, "array", schema.Properties["sl"].Type)
}

// --- JSONParser tests ---

func TestJSONParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid object", `{"key": "value"}`, false},
		{"valid array", `[1, 2, 3]`, false},
		{"valid string", `"hello"`, false},
		{"from code block", "```json\n{\"key\": \"val\"}\n```", false},
		{"invalid json", `{bad}`, true},
		{"empty string", "", true},
	}
	p := NewJSONParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := p.Parse(tt.input, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

// --- YAMLParser tests ---

func TestYAMLParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid yaml", "name: John\nage: 30", false},
		{"from code block", "```yaml\nname: Jane\n```", false},
		{"valid list", "- a\n- b\n- c", false},
	}
	p := NewYAMLParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := p.Parse(tt.input, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, data)
			}
		})
	}
}

// --- MarkdownParser tests ---

func TestMarkdownParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantKey string
		wantVal string
	}{
		{
			"bold key",
			"- **name**: John\n- **age**: 30",
			false, "name", "John",
		},
		{
			"plain key",
			"- status: active\n- count: 5",
			false, "status", "active",
		},
		{
			"asterisk list",
			"* **key**: value",
			false, "key", "value",
		},
		{
			"no structured data",
			"Just a paragraph of text.",
			true, "", "",
		},
	}
	p := NewMarkdownParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := p.Parse(tt.input, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				m, ok := data.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tt.wantVal, m[tt.wantKey])
			}
		})
	}
}

// --- Validator tests ---

func TestValidator_ValidateJSON(t *testing.T) {
	v := NewValidator(false)

	tests := []struct {
		name      string
		input     string
		schema    *Schema
		wantValid bool
	}{
		{
			"valid object",
			`{"name": "John", "age": 30}`,
			&Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"name": {Type: "string"},
					"age":  {Type: "integer"},
				},
				Required: []string{"name"},
			},
			true,
		},
		{
			"missing required",
			`{"age": 30}`,
			&Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"name": {Type: "string"},
					"age":  {Type: "integer"},
				},
				Required: []string{"name"},
			},
			false,
		},
		{
			"wrong type",
			`{"age": "thirty"}`,
			&Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"age": {Type: "integer"},
				},
			},
			false,
		},
		{
			"valid array",
			`["a", "b"]`,
			&Schema{
				Type:  "array",
				Items: &Schema{Type: "string"},
			},
			true,
		},
		{
			"string enum valid",
			`"red"`,
			&Schema{
				Type: "string",
				Enum: []any{"red", "green", "blue"},
			},
			true,
		},
		{
			"string enum invalid",
			`"yellow"`,
			&Schema{
				Type: "string",
				Enum: []any{"red", "green", "blue"},
			},
			false,
		},
		{
			"invalid json",
			`{bad`,
			&Schema{Type: "object"},
			false,
		},
		{
			"boolean valid",
			`true`,
			&Schema{Type: "boolean"},
			true,
		},
		{
			"boolean invalid",
			`"true"`,
			&Schema{Type: "boolean"},
			false,
		},
		{
			"number valid",
			`3.14`,
			&Schema{Type: "number"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, tt.schema)
			require.NoError(t, err)
			assert.Equal(t, tt.wantValid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_StringConstraints(t *testing.T) {
	v := NewValidator(false)
	minLen := 2
	maxLen := 5
	schema := &Schema{
		Type:      "string",
		MinLength: &minLen,
		MaxLength: &maxLen,
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"within range", `"abc"`, true},
		{"too short", `"a"`, false},
		{"too long", `"toolong"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_NumberRange(t *testing.T) {
	v := NewValidator(false)
	min := 0.0
	max := 100.0
	schema := &Schema{
		Type:    "integer",
		Minimum: &min,
		Maximum: &max,
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"in range", `50`, true},
		{"below min", `-1`, false},
		{"above max", `101`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_ArrayConstraints(t *testing.T) {
	v := NewValidator(false)
	minItems := 2
	maxItems := 4
	schema := &Schema{
		Type:     "array",
		Items:    &Schema{Type: "string"},
		MinItems: &minItems,
		MaxItems: &maxItems,
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid", `["a", "b", "c"]`, true},
		{"too few", `["a"]`, false},
		{"too many", `["a","b","c","d","e"]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_Pattern(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type:    "string",
		Pattern: `^\d{3}-\d{4}$`,
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"matches", `"123-4567"`, true},
		{"no match", `"abc"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_NilSchema(t *testing.T) {
	v := NewValidator(false)
	result, err := v.ValidateJSON(`{"a":1}`, nil)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

// --- Repair tests ---

func TestValidator_Repair(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"from markdown code block",
			"```json\n{\"name\": \"John\"}\n```",
			false,
		},
		{
			"trailing comma",
			`{"name": "John",}`,
			false,
		},
		{
			"unquoted key",
			`{name: "John"}`,
			false,
		},
		{
			"irreparable",
			`completely invalid [[[ data`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repaired, err := v.Repair(tt.input, schema)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, repaired, "John")
			}
		})
	}
}

// --- extractFromCodeBlock tests ---

func TestExtractFromCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		lang     string
		expected string
	}{
		{
			"json block",
			"text\n```json\n{\"a\":1}\n```\nmore",
			"json",
			`{"a":1}`,
		},
		{
			"generic block",
			"text\n```\nhello\n```",
			"json",
			"hello",
		},
		{
			"no block",
			"just text",
			"json",
			"just text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFromCodeBlock(tt.input, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- truncate tests ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		expect string
	}{
		{"short", "hi", 10, "hi"},
		{"exact", "hello", 5, "hello"},
		{"long", "hello world", 5, "hello..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, truncate(tt.input, tt.maxLen))
		})
	}
}

// --- Parser interface compliance ---

func TestParserInterfaceCompliance(t *testing.T) {
	var _ Parser = &JSONParser{}
	var _ Parser = &YAMLParser{}
	var _ Parser = &MarkdownParser{}
}

// --- Additional coverage tests ---

func TestSchemaFromType_Interface(t *testing.T) {
	type Container struct {
		Value any `json:"value"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Properties["value"].Type)
}

func TestSchemaFromType_UnsupportedType(t *testing.T) {
	// Channels are not supported
	_, err := SchemaFromType(make(chan int))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestSchemaFromType_RecursiveType(t *testing.T) {
	type Node struct {
		Value    string `json:"value"`
		Children []Node `json:"children"`
	}

	schema, err := SchemaFromType(Node{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "children")
}

func TestSchemaFromType_PointerField(t *testing.T) {
	type Item struct {
		Name *string `json:"name"`
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	assert.Equal(t, "string", schema.Properties["name"].Type)
}

func TestSchemaFromType_MapType(t *testing.T) {
	type Container struct {
		Data map[string]int `json:"data"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Properties["data"].Type)
}

func TestSchemaFromType_UnexportedFields(t *testing.T) {
	type Item struct {
		Public  string `json:"public"`
		private string //nolint:unused
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	assert.Contains(t, schema.Properties, "public")
	assert.NotContains(t, schema.Properties, "private")
}

func TestSchemaFromType_NoJSONTag(t *testing.T) {
	type Item struct {
		FieldName string
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	// Without json tag, uses field name
	assert.Contains(t, schema.Properties, "FieldName")
}

func TestSchemaFromType_Description(t *testing.T) {
	type Item struct {
		Name string `json:"name" description:"The item name"`
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	assert.Equal(t, "The item name", schema.Properties["name"].Description)
}

func TestSchemaFromType_AllIntTypes(t *testing.T) {
	type IntTypes struct {
		I   int   `json:"i"`
		I8  int8  `json:"i8"`
		I16 int16 `json:"i16"`
		I32 int32 `json:"i32"`
		I64 int64 `json:"i64"`
		U   uint  `json:"u"`
		U8  uint8 `json:"u8"`
		U16 uint16 `json:"u16"`
		U32 uint32 `json:"u32"`
		U64 uint64 `json:"u64"`
	}

	schema, err := SchemaFromType(IntTypes{})
	require.NoError(t, err)
	for _, name := range []string{"i", "i8", "i16", "i32", "i64", "u", "u8", "u16", "u32", "u64"} {
		assert.Equal(t, "integer", schema.Properties[name].Type, "type for %s", name)
	}
}

func TestSchemaFromType_Float32(t *testing.T) {
	type Floats struct {
		F32 float32 `json:"f32"`
		F64 float64 `json:"f64"`
	}

	schema, err := SchemaFromType(Floats{})
	require.NoError(t, err)
	assert.Equal(t, "number", schema.Properties["f32"].Type)
	assert.Equal(t, "number", schema.Properties["f64"].Type)
}

func TestSchemaFromType_ArrayType(t *testing.T) {
	type Container struct {
		FixedArray [5]int `json:"fixed_array"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "array", schema.Properties["fixed_array"].Type)
	assert.Equal(t, "integer", schema.Properties["fixed_array"].Items.Type)
}

func TestValidator_Validate(t *testing.T) {
	// Validate delegates to ValidateJSON
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	result, err := v.Validate(`{"name": "test"}`, schema)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestValidator_ValidateJSON_IntegerAsFloat(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "integer"}

	// Float value for integer type
	result, err := v.ValidateJSON(`1.5`, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "expected integer, got float")
}

func TestValidator_ValidateJSON_NumberType(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "number"}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"integer as number", `42`, true},
		{"float as number", `3.14`, true},
		{"string not number", `"42"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_NestedObject(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"person": {
				Type: "object",
				Properties: map[string]*Schema{
					"name": {Type: "string"},
					"age":  {Type: "integer"},
				},
				Required: []string{"name"},
			},
		},
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid nested", `{"person": {"name": "John", "age": 30}}`, true},
		{"missing required nested", `{"person": {"age": 30}}`, false},
		{"wrong type nested", `{"person": {"name": 123}}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_ArrayItemValidation(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type:  "array",
		Items: &Schema{Type: "string"},
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid string array", `["a", "b", "c"]`, true},
		{"invalid item type", `["a", 1, "c"]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestValidator_ValidateJSON_ExpectedArray(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "array"}

	result, err := v.ValidateJSON(`"not an array"`, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "expected array")
}

func TestValidator_ValidateJSON_ExpectedObject(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "object"}

	result, err := v.ValidateJSON(`"not an object"`, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "expected object")
}

func TestValidator_Repair_GenericCodeBlock(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"key": {Type: "string"},
		},
	}

	input := "```\n{\"key\": \"value\"}\n```"
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "value")
}

func TestYAMLParser_InvalidYAML(t *testing.T) {
	p := NewYAMLParser()

	// YAML with tab indentation issues (invalid)
	input := "name:\n\t- invalid"
	_, err := p.Parse(input, nil)
	// YAML library may accept this, let's use truly invalid YAML
	input2 := "key: [unclosed"
	_, err = p.Parse(input2, nil)
	assert.Error(t, err)
}

func TestMarkdownParser_EmptyLines(t *testing.T) {
	p := NewMarkdownParser()

	input := `
- key1: value1

- key2: value2
`
	data, err := p.Parse(input, nil)
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value1", m["key1"])
	assert.Equal(t, "value2", m["key2"])
}

func TestExtractFromCodeBlock_NoMatch(t *testing.T) {
	// Code block without content that matches
	input := "```json\n\n```"
	result := extractFromCodeBlock(input, "json")
	// Empty content after trim
	assert.Equal(t, "", result)
}

func TestValidator_Repair_MarshalIndentError(t *testing.T) {
	// This is hard to trigger as json.MarshalIndent rarely fails
	// but we test the path where validation succeeds and marshal works
	v := NewValidator(false)
	schema := &Schema{Type: "object"}

	input := `{"key":"value"}`
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "key")
}

func TestSchemaFromType_EmptyJSONTagPart(t *testing.T) {
	type Item struct {
		Field string `json:",omitempty"`
	}

	schema, err := SchemaFromType(Item{})
	require.NoError(t, err)
	// With empty first part, should use field name
	assert.Contains(t, schema.Properties, "Field")
	// omitempty means not required
	assert.NotContains(t, schema.Required, "Field")
}

func TestValidator_StrictMode(t *testing.T) {
	v := NewValidator(true) // strict mode
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	result, err := v.Validate(`{"name": "test"}`, schema)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}

func TestSchemaFromType_NestedPointer(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Inner *Inner `json:"inner"`
	}

	schema, err := SchemaFromType(Outer{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "inner")
	assert.Equal(t, "object", schema.Properties["inner"].Type)
}

func TestSchemaFromType_SliceOfPointers(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}
	type Container struct {
		Items []*Item `json:"items"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "array", schema.Properties["items"].Type)
	assert.Equal(t, "object", schema.Properties["items"].Items.Type)
}

func TestValidator_Repair_InvalidAfterRepair(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"count": {Type: "integer"},
		},
		Required: []string{"count"},
	}

	// Input that can be parsed but doesn't match schema requirements
	input := `{"name": "test"}`
	_, err := v.Repair(input, schema)
	// Should error because required field is missing
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not repair output")
}

func TestValidator_ValidateJSON_NestedArrayObject(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"items": {
				Type: "array",
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"name": {Type: "string"},
					},
					Required: []string{"name"},
				},
			},
		},
	}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid nested array", `{"items": [{"name": "a"}, {"name": "b"}]}`, true},
		{"invalid item", `{"items": [{"name": "a"}, {"other": "b"}]}`, false},
		{"wrong item type", `{"items": ["a", "b"]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON(tt.input, schema)
			require.NoError(t, err)
			assert.Equal(t, tt.valid, result.Valid)
		})
	}
}

func TestExtractFromCodeBlock_MultipleBlocks(t *testing.T) {
	// Only first block is extracted
	input := "```json\n{\"first\": 1}\n```\n\ntext\n\n```json\n{\"second\": 2}\n```"
	result := extractFromCodeBlock(input, "json")
	assert.Contains(t, result, "first")
}

func TestYAMLParser_GenericCodeBlock(t *testing.T) {
	p := NewYAMLParser()

	// YAML in generic code block
	input := "```\nname: John\nage: 30\n```"
	data, err := p.Parse(input, nil)
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "John", m["name"])
}

func TestJSONParser_GenericCodeBlock(t *testing.T) {
	p := NewJSONParser()

	// JSON in generic code block
	input := "```\n{\"key\": \"value\"}\n```"
	data, err := p.Parse(input, nil)
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", m["key"])
}

func TestValidator_ValidateJSON_StringExpected(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "string"}

	result, err := v.ValidateJSON(`42`, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "expected string")
}

func TestValidator_ValidateJSON_IntegerExpected(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "integer"}

	result, err := v.ValidateJSON(`"string"`, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0].Message, "expected integer")
}

func TestValidator_Repair_UnquotedKeys(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
	}

	input := `{name: "John", age: 30}`
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "John")
	assert.Contains(t, repaired, "30")
}

func TestValidator_Repair_NestedTrailingComma(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{Type: "object"}

	input := `{"a": [1, 2, 3,], "b": {"c": "d",}}`
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.NotContains(t, repaired, ",]")
	assert.NotContains(t, repaired, ",}")
}

func TestSchemaFromType_EmptyStruct(t *testing.T) {
	type Empty struct{}

	schema, err := SchemaFromType(Empty{})
	require.NoError(t, err)
	assert.Equal(t, "object", schema.Type)
	assert.Empty(t, schema.Properties)
	assert.Empty(t, schema.Required)
}

func TestSchemaFromType_NestedSlice(t *testing.T) {
	type Container struct {
		Matrix [][]int `json:"matrix"`
	}

	schema, err := SchemaFromType(Container{})
	require.NoError(t, err)
	assert.Equal(t, "array", schema.Properties["matrix"].Type)
	assert.Equal(t, "array", schema.Properties["matrix"].Items.Type)
	assert.Equal(t, "integer", schema.Properties["matrix"].Items.Items.Type)
}

func TestValidator_ValidateJSON_EmptyEnum(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "string",
		Enum: []any{}, // Empty enum
	}

	result, err := v.ValidateJSON(`"any_value"`, schema)
	require.NoError(t, err)
	// Empty enum check is skipped (len(schema.Enum) > 0 is false)
	// So any string value passes for empty enum
	assert.True(t, result.Valid)
}

func TestValidator_ValidateJSON_ArrayNoItemsSchema(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type:  "array",
		Items: nil, // No items schema
	}

	result, err := v.ValidateJSON(`[1, "a", true]`, schema)
	require.NoError(t, err)
	// Without items schema, any array items are valid
	assert.True(t, result.Valid)
}

func TestSchemaFromType_ErrorInSliceElem(t *testing.T) {
	// Test error propagation from slice element type
	// Since channels are unsupported, a slice of channels should fail
	_, err := SchemaFromType([]chan int{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestSchemaFromType_ErrorInNestedStruct(t *testing.T) {
	type Outer struct {
		Inner chan int `json:"inner"` // Unsupported type in struct
	}

	_, err := SchemaFromType(Outer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestValidator_Repair_ValidJSONNoChange(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	// Already valid JSON
	input := `{"name": "test"}`
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "test")
}

// Tests for Repair marshalIndent error path using dependency injection.

func TestValidator_Repair_MarshalIndentError_DI(t *testing.T) {
	v := NewValidator(false)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	// Inject a failing marshal indent function
	v.marshalIndent = func(v any, prefix, indent string) ([]byte, error) {
		return nil, fmt.Errorf("simulated marshal indent error")
	}

	input := `{"name": "test"}`
	repaired, err := v.Repair(input, schema)
	// When MarshalIndent fails, Repair returns the original output
	require.NoError(t, err)
	assert.Equal(t, input, repaired)
}

// Test that nil marshalIndent defaults to json.MarshalIndent.

func TestValidator_Repair_NilMarshalIndentDefault(t *testing.T) {
	v := &Validator{
		strictMode:    false,
		marshalIndent: nil, // Explicitly nil
	}
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	input := `{"name": "test"}`
	repaired, err := v.Repair(input, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "test")
}
