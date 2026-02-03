// Package structured provides parsers and validators for structured output
// formats including JSON, YAML, and Markdown.
package structured

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputFormat represents a supported output format.
type OutputFormat string

const (
	FormatJSON     OutputFormat = "json"
	FormatYAML     OutputFormat = "yaml"
	FormatMarkdown OutputFormat = "markdown"
)

// Schema defines the expected structure of parsed output.
type Schema struct {
	Type        string             `json:"type" yaml:"type"`
	Properties  map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string           `json:"required,omitempty" yaml:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
	Enum        []any              `json:"enum,omitempty" yaml:"enum,omitempty"`
	Pattern     string             `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	MinLength   *int               `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinItems    *int               `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems    *int               `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
}

// SchemaFromType generates a Schema from a Go struct type using
// reflection and JSON struct tags.
func SchemaFromType(t any) (*Schema, error) {
	val := reflect.TypeOf(t)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return schemaFromReflectType(val, make(map[string]bool))
}

func schemaFromReflectType(
	t reflect.Type, visited map[string]bool,
) (*Schema, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	typeName := t.String()
	if visited[typeName] {
		return &Schema{Type: "object"}, nil
	}
	visited[typeName] = true
	defer delete(visited, typeName)

	s := &Schema{}

	switch t.Kind() {
	case reflect.String:
		s.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		s.Type = "integer"
	case reflect.Float32, reflect.Float64:
		s.Type = "number"
	case reflect.Bool:
		s.Type = "boolean"
	case reflect.Slice, reflect.Array:
		s.Type = "array"
		item, err := schemaFromReflectType(t.Elem(), visited)
		if err != nil {
			return nil, err
		}
		s.Items = item
	case reflect.Map:
		s.Type = "object"
	case reflect.Struct:
		s.Type = "object"
		s.Properties = make(map[string]*Schema)
		s.Required = make([]string, 0)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			jsonTag := field.Tag.Get("json")
			fieldName := field.Name
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] == "-" {
					continue
				}
				if parts[0] != "" {
					fieldName = parts[0]
				}
			}
			prop, err := schemaFromReflectType(field.Type, visited)
			if err != nil {
				return nil, err
			}
			if desc := field.Tag.Get("description"); desc != "" {
				prop.Description = desc
			}
			s.Properties[fieldName] = prop
			if jsonTag != "" && !strings.Contains(jsonTag, "omitempty") {
				s.Required = append(s.Required, fieldName)
			}
		}
	case reflect.Interface:
		s.Type = "object"
	default:
		return nil, fmt.Errorf("unsupported type: %v", t.Kind())
	}

	return s, nil
}

// --- Parser interface and implementations ---

// Parser parses a raw string output into structured data using a schema.
type Parser interface {
	// Parse parses the output string according to the schema.
	Parse(output string, schema *Schema) (any, error)
}

// JSONParser parses JSON output.
type JSONParser struct{}

// NewJSONParser creates a new JSON parser.
func NewJSONParser() *JSONParser { return &JSONParser{} }

// Parse parses a JSON string into a Go value.
func (p *JSONParser) Parse(output string, _ *Schema) (any, error) {
	output = strings.TrimSpace(output)
	output = extractFromCodeBlock(output, "json")

	var data any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return data, nil
}

// YAMLParser parses YAML output.
type YAMLParser struct{}

// NewYAMLParser creates a new YAML parser.
func NewYAMLParser() *YAMLParser { return &YAMLParser{} }

// Parse parses a YAML string into a Go value.
func (p *YAMLParser) Parse(output string, _ *Schema) (any, error) {
	output = strings.TrimSpace(output)
	output = extractFromCodeBlock(output, "yaml")

	var data any
	if err := yaml.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return data, nil
}

// MarkdownParser extracts structured data from markdown content.
type MarkdownParser struct{}

// NewMarkdownParser creates a new Markdown parser.
func NewMarkdownParser() *MarkdownParser { return &MarkdownParser{} }

// Parse extracts key-value pairs from markdown list items.
// It looks for lines like "- **key**: value" or "- key: value".
func (p *MarkdownParser) Parse(
	output string, _ *Schema,
) (any, error) {
	output = strings.TrimSpace(output)
	result := make(map[string]any)
	kvRe := regexp.MustCompile(
		`^[-*]\s+\*{0,2}([^*:]+)\*{0,2}\s*:\s*(.+)$`,
	)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := kvRe.FindStringSubmatch(line)
		if len(matches) == 3 {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			result[key] = value
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no structured data found in markdown")
	}
	return result, nil
}

// --- Validator ---

// ValidationError describes a single validation failure.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationResult contains the outcome of validation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
	Data   any               `json:"data,omitempty"`
}

// jsonMarshalIndenter is a function type for JSON marshal indent (for testing).
type jsonMarshalIndenter func(v any, prefix, indent string) ([]byte, error)

// Validator checks parsed output against a schema.
type Validator struct {
	strictMode    bool
	marshalIndent jsonMarshalIndenter
}

// NewValidator creates a new schema validator.
func NewValidator(strictMode bool) *Validator {
	return &Validator{strictMode: strictMode, marshalIndent: json.MarshalIndent}
}

// Validate validates a raw string against a schema by first parsing it
// as JSON.
func (v *Validator) Validate(
	output string, schema *Schema,
) (*ValidationResult, error) {
	return v.ValidateJSON(output, schema)
}

// ValidateJSON validates a JSON string against a schema.
func (v *Validator) ValidateJSON(
	output string, schema *Schema,
) (*ValidationResult, error) {
	result := &ValidationResult{Valid: true}

	var data any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Path:    "$",
			Message: fmt.Sprintf("invalid JSON: %v", err),
			Value:   truncate(output, 100),
		})
		return result, nil
	}

	result.Data = data
	errs := v.validateValue(data, schema, "$")
	if len(errs) > 0 {
		result.Valid = false
		result.Errors = errs
	}
	return result, nil
}

func (v *Validator) validateValue(
	value any, schema *Schema, path string,
) []ValidationError {
	if schema == nil {
		return nil
	}

	var errs []ValidationError

	switch schema.Type {
	case "string":
		str, ok := value.(string)
		if !ok {
			return append(errs, ValidationError{
				Path: path, Message: "expected string",
				Value: fmt.Sprintf("%T", value),
			})
		}
		if schema.MinLength != nil && len(str) < *schema.MinLength {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string too short (min: %d)", *schema.MinLength),
			})
		}
		if schema.MaxLength != nil && len(str) > *schema.MaxLength {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string too long (max: %d)", *schema.MaxLength),
			})
		}
		if schema.Pattern != "" {
			if matched, _ := regexp.MatchString(schema.Pattern, str); !matched {
				errs = append(errs, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("does not match pattern: %s", schema.Pattern),
				})
			}
		}
		if len(schema.Enum) > 0 {
			found := false
			for _, e := range schema.Enum {
				if e == str {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("value not in enum: %v", schema.Enum),
				})
			}
		}

	case "integer":
		num, ok := value.(float64)
		if !ok {
			return append(errs, ValidationError{
				Path: path, Message: "expected integer",
				Value: fmt.Sprintf("%T", value),
			})
		}
		if num != float64(int64(num)) {
			errs = append(errs, ValidationError{
				Path: path, Message: "expected integer, got float",
			})
		}
		if schema.Minimum != nil && num < *schema.Minimum {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("below minimum (%v)", *schema.Minimum),
			})
		}
		if schema.Maximum != nil && num > *schema.Maximum {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("above maximum (%v)", *schema.Maximum),
			})
		}

	case "number":
		if _, ok := value.(float64); !ok {
			errs = append(errs, ValidationError{
				Path: path, Message: "expected number",
				Value: fmt.Sprintf("%T", value),
			})
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			errs = append(errs, ValidationError{
				Path: path, Message: "expected boolean",
				Value: fmt.Sprintf("%T", value),
			})
		}

	case "array":
		arr, ok := value.([]any)
		if !ok {
			return append(errs, ValidationError{
				Path: path, Message: "expected array",
				Value: fmt.Sprintf("%T", value),
			})
		}
		if schema.MinItems != nil && len(arr) < *schema.MinItems {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("array too short (min: %d)", *schema.MinItems),
			})
		}
		if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("array too long (max: %d)", *schema.MaxItems),
			})
		}
		if schema.Items != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				errs = append(errs, v.validateValue(item, schema.Items, itemPath)...)
			}
		}

	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return append(errs, ValidationError{
				Path: path, Message: "expected object",
				Value: fmt.Sprintf("%T", value),
			})
		}
		for _, req := range schema.Required {
			if _, exists := obj[req]; !exists {
				errs = append(errs, ValidationError{
					Path:    path + "." + req,
					Message: "required property missing",
				})
			}
		}
		for propName, propSchema := range schema.Properties {
			if propValue, exists := obj[propName]; exists {
				propPath := path + "." + propName
				errs = append(errs,
					v.validateValue(propValue, propSchema, propPath)...)
			}
		}
	}

	return errs
}

// Repair attempts to fix common issues in JSON output.
func (v *Validator) Repair(
	output string, schema *Schema,
) (string, error) {
	output = strings.TrimSpace(output)
	output = extractFromCodeBlock(output, "json")

	// Remove trailing commas.
	output = regexp.MustCompile(`,\s*([\]}])`).
		ReplaceAllString(output, "$1")

	// Add missing quotes to keys.
	output = regexp.MustCompile(`(\{|,)\s*(\w+)\s*:`).
		ReplaceAllString(output, `$1"$2":`)

	// ValidateJSON never returns an error; it puts errors in ValidationResult.
	result, _ := v.ValidateJSON(output, schema)
	if !result.Valid {
		return "", fmt.Errorf("could not repair output: %v", result.Errors)
	}

	marshal := v.marshalIndent
	if marshal == nil {
		marshal = json.MarshalIndent
	}
	data, err := marshal(result.Data, "", "  ")
	if err != nil {
		return output, nil
	}
	return string(data), nil
}

// --- helpers ---

func extractFromCodeBlock(output, lang string) string {
	if strings.Contains(output, "```"+lang) {
		re := regexp.MustCompile(
			"```" + lang + `\s*\n([\s\S]*?)\n` + "```",
		)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	} else if strings.Contains(output, "```") {
		re := regexp.MustCompile("```\\s*\\n([\\s\\S]*?)\\n```")
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return output
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
