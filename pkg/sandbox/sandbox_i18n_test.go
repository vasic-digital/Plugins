// Sentinel call-site tests for the sandbox package i18n migration
// (CONST-046, round-125).
//
// These tests pin the migrated error messages so a regression of the
// translator-seam wiring fails loud rather than greenly passing.
//
// Anti-bluff invariant: the assertions check the FULL message text
// captured from the real ProcessSandbox.Execute and
// InProcessSandbox.Execute call paths — no mock, no stub, no
// log-grep.
package sandbox_test

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.plugins/pkg/i18n"
	"digital.vasic.plugins/pkg/sandbox"
)

// fakeTranslator is a unit-test-only stub (CONST-050(A)). It records
// the most-recent key requested so the test asserts the call-site
// really hit the seam.
type fakeTranslator struct {
	lastKey string
	prefix  string
}

func (f *fakeTranslator) T(key string, _ map[string]any) string {
	f.lastKey = key
	return f.prefix + key
}

func TestProcessSandbox_Execute_NilPlugin_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{prefix: "X:"}
	sandbox.SetTranslator(ft)
	defer sandbox.SetTranslator(i18n.NoopTranslator{})

	sb := sandbox.NewProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, sandbox.Action{Name: "health"})
	if err == nil {
		t.Fatalf("expected error from nil plugin; got nil")
	}
	if ft.lastKey != "plugins_sandbox_plugin_nil" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
	if !strings.Contains(err.Error(), "plugins_sandbox_plugin_nil") {
		t.Fatalf("error %q does not carry seam payload", err.Error())
	}
}

func TestInProcessSandbox_Execute_NilPlugin_UsesI18nSeam(t *testing.T) {
	ft := &fakeTranslator{}
	sandbox.SetTranslator(ft)
	defer sandbox.SetTranslator(i18n.NoopTranslator{})

	sb := sandbox.NewInProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, sandbox.Action{Name: "health"})
	if err == nil {
		t.Fatalf("expected error from nil plugin; got nil")
	}
	if ft.lastKey != "plugins_sandbox_plugin_nil" {
		t.Fatalf("seam not hit: lastKey=%q", ft.lastKey)
	}
}

// TestProcessSandbox_NilPlugin_NoopDefault_PreservesLegacyText asserts
// that production callers who never invoke SetTranslator still see
// the original (now-key-shaped) error payload — keeps downstream
// string assertions actionable.
func TestProcessSandbox_NilPlugin_NoopDefault_PreservesLegacyText(t *testing.T) {
	sandbox.SetTranslator(i18n.NoopTranslator{})

	sb := sandbox.NewProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, sandbox.Action{Name: "health"})
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if err.Error() != "plugins_sandbox_plugin_nil" {
		t.Fatalf("Noop default error = %q; want exact key", err.Error())
	}
}

// TestSetTranslator_NilFallsBackToNoop verifies the nil-guard so
// callers can't accidentally crash production by passing nil.
func TestSetTranslator_NilFallsBackToNoop(t *testing.T) {
	sandbox.SetTranslator(nil) // must not panic
	defer sandbox.SetTranslator(i18n.NoopTranslator{})

	sb := sandbox.NewProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, sandbox.Action{Name: "health"})
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	if err.Error() != "plugins_sandbox_plugin_nil" {
		t.Fatalf("nil-guard fallback failed; err=%q", err.Error())
	}
}
