package structured

import (
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
