// Sentinel call-site tests for the structured-output schema-validation
// i18n migration (CONST-046, round-386 §11.4).
//
// These tests pin the migrated ValidationError.Message strings so a
// regression of the translator-seam wiring fails loud rather than
// greenly passing. Each test below proves three invariants per
// migrated key:
//
//  1. NoopTranslator path emits the legacy English fallback verbatim
//     (so historical regression tests + downstream string assertions
//     keep passing — Article XI §11.9 / CONST-035 / CONST-046).
//  2. The recording translator captures the expected i18n key (so a
//     future refactor that drops the SetTranslator wiring or bypasses
//     v.tr() gets caught by the missing-call paired mutation).
//  3. SetTranslator(nil) is a safe no-op (so a regression that drops
//     the nil-guard cannot crash production at wire-up time).
//
// Anti-bluff invariant: the assertions check the FULL message text
// captured from the real Validator.ValidateJSON call path — no mock
// of the validator, no stub, no log-grep. The translator itself is a
// unit-test-only stub (CONST-050(A) — permitted in *_test.go).
package structured

import (
	"testing"

	"digital.vasic.plugins/pkg/i18n"
)

// recordingTranslator is a unit-test-only stub (CONST-050(A)). It
// records every key requested so a test asserts the call-site really
// hit the v.tr() seam, and returns a deterministic non-key string so
// the fallback short-circuit (rendered == key) does NOT fire.
type recordingTranslator struct {
	keys []string
}

func (rt *recordingTranslator) T(key string, _ map[string]any) string {
	rt.keys = append(rt.keys, key)
	return "translated:" + key
}

func (rt *recordingTranslator) seen(key string) bool {
	for _, k := range rt.keys {
		if k == key {
			return true
		}
	}
	return false
}

// strPtr / intPtr build the *int the Schema length/item bounds need.
func intPtr(n int) *int { return &n }

// validateForMessages is the shared driver: it builds a Validator with
// the given translator, validates output against schema, and returns
// every ValidationError.Message produced.
func validateForMessages(v *Validator, output string, schema *Schema) []string {
	res, err := v.ValidateJSON(output, schema)
	if err != nil {
		return nil
	}
	msgs := make([]string, 0, len(res.Errors))
	for _, e := range res.Errors {
		msgs = append(msgs, e.Message)
	}
	return msgs
}

// contains reports whether want appears in msgs.
func contains(msgs []string, want string) bool {
	for _, m := range msgs {
		if m == want {
			return true
		}
	}
	return false
}

// --- 1. NoopTranslator preserves legacy English ---------------------

func TestValidator_NoopTranslator_PreservesEnglishValidationMessages(t *testing.T) {
	v := NewValidator(false)

	cases := []struct {
		name   string
		output string
		schema *Schema
		want   string
	}{
		{"invalid_json", "{not json", &Schema{Type: "object"},
			"invalid JSON: invalid character 'n' looking for beginning of object key string"},
		{"expected_string", `123`, &Schema{Type: "string"},
			"expected string"},
		{"string_too_short", `"hi"`, &Schema{Type: "string", MinLength: intPtr(5)},
			"string too short (min: 5)"},
		{"string_too_long", `"hello"`, &Schema{Type: "string", MaxLength: intPtr(2)},
			"string too long (max: 2)"},
		{"pattern_mismatch", `"abc"`, &Schema{Type: "string", Pattern: "^[0-9]+$"},
			"does not match pattern: ^[0-9]+$"},
		{"value_not_in_enum", `"x"`, &Schema{Type: "string", Enum: []any{"a", "b"}},
			"value not in enum: [a b]"},
		{"expected_integer", `"x"`, &Schema{Type: "integer"},
			"expected integer"},
		{"expected_integer_got_float", `1.5`, &Schema{Type: "integer"},
			"expected integer, got float"},
		{"expected_number", `"x"`, &Schema{Type: "number"},
			"expected number"},
		{"expected_boolean", `"x"`, &Schema{Type: "boolean"},
			"expected boolean"},
		{"expected_array", `"x"`, &Schema{Type: "array"},
			"expected array"},
		{"array_too_short", `[1]`, &Schema{Type: "array", MinItems: intPtr(3)},
			"array too short (min: 3)"},
		{"array_too_long", `[1,2,3]`, &Schema{Type: "array", MaxItems: intPtr(1)},
			"array too long (max: 1)"},
		{"expected_object", `"x"`, &Schema{Type: "object"},
			"expected object"},
		{"required_property_missing", `{}`,
			&Schema{Type: "object", Required: []string{"id"}},
			"required property missing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs := validateForMessages(v, tc.output, tc.schema)
			if !contains(msgs, tc.want) {
				t.Fatalf("NoopTranslator fallback: want %q in %v", tc.want, msgs)
			}
		})
	}
}

func TestValidator_NoopTranslator_BelowMinimum_AboveMaximum(t *testing.T) {
	one := 1.0
	ten := 10.0
	v := NewValidator(false)

	below := validateForMessages(v, `0`, &Schema{Type: "integer", Minimum: &one})
	if !contains(below, "below minimum (1)") {
		t.Fatalf("below-minimum fallback: %v", below)
	}
	above := validateForMessages(v, `99`, &Schema{Type: "integer", Maximum: &ten})
	if !contains(above, "above maximum (10)") {
		t.Fatalf("above-maximum fallback: %v", above)
	}
}

// --- 2. SetTranslator routes every key through the seam --------------

func TestValidator_SetTranslator_RoutesEveryValidationKey(t *testing.T) {
	rt := &recordingTranslator{}
	v := NewValidator(false)
	v.SetTranslator(rt)

	// Drive every key once. Each row's schema/output forces exactly
	// one branch so the recordingTranslator key set is deterministic.
	cases := []struct {
		key    string
		output string
		schema *Schema
	}{
		{"plugins_validation_invalid_json", "{not json", &Schema{Type: "object"}},
		{"plugins_validation_expected_string", `1`, &Schema{Type: "string"}},
		{"plugins_validation_string_too_short", `"x"`,
			&Schema{Type: "string", MinLength: intPtr(9)}},
		{"plugins_validation_string_too_long", `"xxxx"`,
			&Schema{Type: "string", MaxLength: intPtr(1)}},
		{"plugins_validation_pattern_mismatch", `"x"`,
			&Schema{Type: "string", Pattern: "^[0-9]$"}},
		{"plugins_validation_value_not_in_enum", `"x"`,
			&Schema{Type: "string", Enum: []any{"a"}}},
		{"plugins_validation_expected_integer", `"x"`, &Schema{Type: "integer"}},
		{"plugins_validation_expected_integer_got_float", `2.5`,
			&Schema{Type: "integer"}},
		{"plugins_validation_expected_number", `"x"`, &Schema{Type: "number"}},
		{"plugins_validation_expected_boolean", `"x"`, &Schema{Type: "boolean"}},
		{"plugins_validation_expected_array", `"x"`, &Schema{Type: "array"}},
		{"plugins_validation_array_too_short", `[1]`,
			&Schema{Type: "array", MinItems: intPtr(4)}},
		{"plugins_validation_array_too_long", `[1,2]`,
			&Schema{Type: "array", MaxItems: intPtr(1)}},
		{"plugins_validation_expected_object", `"x"`, &Schema{Type: "object"}},
		{"plugins_validation_required_property_missing", `{}`,
			&Schema{Type: "object", Required: []string{"id"}}},
	}

	for _, tc := range cases {
		msgs := validateForMessages(v, tc.output, tc.schema)
		want := "translated:" + tc.key
		if !contains(msgs, want) {
			t.Fatalf("key %s: seam not used, messages=%v", tc.key, msgs)
		}
		if !rt.seen(tc.key) {
			t.Fatalf("key %s: recordingTranslator never saw it; keys=%v",
				tc.key, rt.keys)
		}
	}

	// below-minimum / above-maximum need *float64 bounds.
	one := 1.0
	ten := 10.0
	belowMsgs := validateForMessages(v, `0`, &Schema{Type: "integer", Minimum: &one})
	if !contains(belowMsgs, "translated:plugins_validation_below_minimum") {
		t.Fatalf("below-minimum seam: %v", belowMsgs)
	}
	aboveMsgs := validateForMessages(v, `99`, &Schema{Type: "integer", Maximum: &ten})
	if !contains(aboveMsgs, "translated:plugins_validation_above_maximum") {
		t.Fatalf("above-maximum seam: %v", aboveMsgs)
	}
}

// --- 3. SetTranslator(nil) is a safe no-op --------------------------

func TestValidator_SetTranslator_NilIsNoop(t *testing.T) {
	v := NewValidator(false)
	v.SetTranslator(nil) // MUST NOT panic, MUST NOT overwrite default

	msgs := validateForMessages(v, `1`, &Schema{Type: "string"})
	if !contains(msgs, "expected string") {
		t.Fatalf("after nil SetTranslator: want legacy English, got %v", msgs)
	}
}

// TestValidator_StructLiteral_DefaultsToNoop pins the activeTranslator
// nil-guard: a Validator built via a struct literal (no NewValidator)
// has a nil translator field and MUST still render the English
// fallback rather than panic.
func TestValidator_StructLiteral_DefaultsToNoop(t *testing.T) {
	v := &Validator{} // translator field is nil
	msgs := validateForMessages(v, `1`, &Schema{Type: "string"})
	if !contains(msgs, "expected string") {
		t.Fatalf("struct-literal Validator: want English fallback, got %v", msgs)
	}
}

// Compile-time assertion: NoopTranslator satisfies i18n.Translator —
// the paired-mutation pair for a translator-import regression.
var _ i18n.Translator = i18n.NoopTranslator{}
