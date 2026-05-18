// Sentinel call-site tests for the plugin package i18n migration
// (CONST-046, round-125).
//
// These tests pin the migrated error messages so a regression of the
// translator-seam wiring (e.g. accidentally hardcoding the literal
// back in source, or breaking the SetTranslator override) fails loud
// rather than greenly passing.
//
// Anti-bluff invariant: the assertions check the FULL message text
// captured from the real Metadata.Validate call path — no mock, no
// stub, no log-grep. Real call site is exercised end-to-end with real
// inputs.
package plugin_test

import (
	"strings"
	"testing"

	"digital.vasic.plugins/pkg/i18n"
	"digital.vasic.plugins/pkg/plugin"
)

// fakeTranslator is a unit-test-only stub (CONST-050(A): mocks
// allowed only in unit tests). It records the most-recent key
// requested so the test asserts the call-site really hit the seam.
type fakeTranslator struct {
	lastKey string
	prefix  string
}

func (f *fakeTranslator) T(key string, _ map[string]any) string {
	f.lastKey = key
	return f.prefix + key
}

func TestMetadata_Validate_MissingName_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{prefix: "X:"}
	plugin.SetTranslator(ft)
	defer plugin.SetTranslator(i18n.NoopTranslator{})

	m := &plugin.Metadata{Version: "1.0.0"}
	err := m.Validate()
	if err == nil {
		t.Fatalf("expected error from missing name; got nil")
	}
	if ft.lastKey != "plugins_metadata_name_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	if !strings.Contains(err.Error(), "plugins_metadata_name_required") {
		t.Fatalf("error %q does not carry seam payload", err.Error())
	}
}

func TestMetadata_Validate_MissingVersion_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	plugin.SetTranslator(ft)
	defer plugin.SetTranslator(i18n.NoopTranslator{})

	m := &plugin.Metadata{Name: "demo"}
	err := m.Validate()
	if err == nil {
		t.Fatalf("expected error from missing version; got nil")
	}
	if ft.lastKey != "plugins_metadata_version_required" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

// TestMetadata_Validate_NoopDefault_PreservesLegacyText asserts that
// production callers who never invoke SetTranslator still see the
// original (now-key-shaped) error payload — keeps downstream string
// assertions actionable.
func TestMetadata_Validate_NoopDefault_PreservesLegacyText(t *testing.T) {
	plugin.SetTranslator(i18n.NoopTranslator{})

	m := &plugin.Metadata{Version: "1.0.0"}
	err := m.Validate()
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if err.Error() != "plugins_metadata_name_required" {
		t.Fatalf("Noop default error = %q; want exact key", err.Error())
	}
}

// TestSetTranslator_NilFallsBackToNoop verifies the nil-guard so
// callers can't accidentally crash production by passing nil.
func TestSetTranslator_NilFallsBackToNoop(t *testing.T) {
	plugin.SetTranslator(nil) // must not panic
	defer plugin.SetTranslator(i18n.NoopTranslator{})

	m := &plugin.Metadata{Version: "1.0.0"}
	err := m.Validate()
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if err.Error() != "plugins_metadata_name_required" {
		t.Fatalf("nil-guard fallback failed; err=%q", err.Error())
	}
}
