package security

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type secPlugin struct {
	name    string
	version string
}

func (p *secPlugin) Name() string                                  { return p.name }
func (p *secPlugin) Version() string                               { return p.version }
func (p *secPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (p *secPlugin) Start(_ context.Context) error                 { return nil }
func (p *secPlugin) Stop(_ context.Context) error                  { return nil }
func (p *secPlugin) HealthCheck(_ context.Context) error           { return nil }

func TestNilPluginRegistration_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()

	err := reg.Register(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestEmptyNameRegistration_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	p := &secPlugin{name: "", version: "1.0.0"}

	err := reg.Register(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestDuplicateRegistration_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	reg := registry.New()
	p1 := &secPlugin{name: "dup", version: "1.0.0"}
	p2 := &secPlugin{name: "dup", version: "2.0.0"}

	require.NoError(t, reg.Register(p1))
	err := reg.Register(p2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestMetadataValidation_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	testCases := []struct {
		name     string
		metadata plugin.Metadata
		wantErr  bool
	}{
		{"empty name", plugin.Metadata{Name: "", Version: "1.0.0"}, true},
		{"empty version", plugin.Metadata{Name: "test", Version: ""}, true},
		{"valid", plugin.Metadata{Name: "test", Version: "1.0.0"}, false},
		{"with special chars", plugin.Metadata{Name: "test-plugin_v2", Version: "1.0.0"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.metadata.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNilConfigAccessorSafety_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	var cfg plugin.Config // nil

	assert.Equal(t, "fallback", cfg.GetString("key", "fallback"))
	assert.Equal(t, 42, cfg.GetInt("key", 42))
	assert.InDelta(t, 3.14, cfg.GetFloat64("key", 3.14), 0.001)
	assert.False(t, cfg.GetBool("key", false))
	assert.Nil(t, cfg.GetStringSlice("key"))
	assert.False(t, cfg.Has("key"))
}

func TestSandboxNilPluginHandling_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	ctx := context.Background()
	cfg := sandbox.DefaultConfig()

	// ProcessSandbox
	ps := sandbox.NewProcessSandbox(cfg)
	_, err := ps.Execute(ctx, nil, sandbox.Action{Name: "health"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// InProcessSandbox
	ips := sandbox.NewInProcessSandbox(cfg)
	_, err = ips.Execute(ctx, nil, sandbox.Action{Name: "health"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestMaliciousJSONSchemaValidation_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	validator := structured.NewValidator(true)
	schema := &structured.Schema{
		Type: "object",
		Properties: map[string]*structured.Schema{
			"name": {Type: "string"},
		},
		Required: []string{"name"},
	}

	maliciousInputs := []string{
		`{"name": "` + strings.Repeat("A", 100000) + `"}`,
		`{"name": "\u0000\u0001\u0002"}`,
		`{"name": "test", "__proto__": {"polluted": true}}`,
		`{"name": "<script>alert(1)</script>"}`,
	}

	for _, input := range maliciousInputs {
		result, err := validator.ValidateJSON(input, schema)
		require.NoError(t, err, "validator should not error on input")
		assert.True(t, result.Valid, "input with string name should be valid")
	}
}

func TestInvalidVersionConstraints_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	invalidVersions := []string{
		"",
		"abc",
		"1.2",
		"1.2.3.4",
		"-1.0.0",
		"1..0",
	}

	for _, v := range invalidVersions {
		_, err := registry.CheckVersionConstraint(v, ">=1.0.0")
		assert.Error(t, err, "version %q should be invalid", v)
	}

	invalidConstraints := []string{
		"abc",
		"1.2",
	}

	for _, c := range invalidConstraints {
		_, err := registry.CheckVersionConstraint("1.0.0", c)
		assert.Error(t, err, "constraint %q should be invalid", c)
	}
}

func TestValidatorRepairBrokenJSON_Security(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	validator := structured.NewValidator(true)
	schema := &structured.Schema{
		Type: "object",
		Properties: map[string]*structured.Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	// Trailing comma repair
	repaired, err := validator.Repair(`{"name": "Alice", "age": 30,}`, schema)
	require.NoError(t, err)
	assert.Contains(t, repaired, "Alice")
}
