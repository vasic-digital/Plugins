package integration

import (
	"context"
	"fmt"
	"testing"

	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlugin is a minimal Plugin implementation for integration tests.
type testPlugin struct {
	name    string
	version string
	started bool
	stopped bool
}

func (p *testPlugin) Name() string                                  { return p.name }
func (p *testPlugin) Version() string                               { return p.version }
func (p *testPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (p *testPlugin) Start(_ context.Context) error                 { p.started = true; return nil }
func (p *testPlugin) Stop(_ context.Context) error                  { p.stopped = true; return nil }
func (p *testPlugin) HealthCheck(_ context.Context) error           { return nil }

func TestRegistryWithPluginLifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	reg := registry.New()
	ctx := context.Background()

	plugins := []*testPlugin{
		{name: "auth", version: "1.0.0"},
		{name: "cache", version: "2.0.0"},
		{name: "logger", version: "1.5.0"},
	}

	for _, p := range plugins {
		err := reg.Register(p)
		require.NoError(t, err)
	}

	// Set dependencies: cache depends on logger
	err := reg.SetDependencies("cache", []string{"logger"})
	require.NoError(t, err)

	// Start all in dependency order
	err = reg.StartAll(ctx)
	require.NoError(t, err)

	for _, p := range plugins {
		assert.True(t, p.started, "plugin %s should be started", p.name)
	}

	// Stop all in reverse order
	err = reg.StopAll(ctx)
	require.NoError(t, err)

	for _, p := range plugins {
		assert.True(t, p.stopped, "plugin %s should be stopped", p.name)
	}
}

func TestRegistryLookupAndRemove_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	reg := registry.New()
	p := &testPlugin{name: "metrics", version: "1.0.0"}

	err := reg.Register(p)
	require.NoError(t, err)

	// Lookup
	found, ok := reg.Get("metrics")
	require.True(t, ok)
	assert.Equal(t, "metrics", found.Name())

	// List
	names := reg.List()
	assert.Contains(t, names, "metrics")

	// Remove
	err = reg.Remove("metrics")
	require.NoError(t, err)

	_, ok = reg.Get("metrics")
	assert.False(t, ok)
}

func TestVersionConstraintChecking_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testCases := []struct {
		version    string
		constraint string
		expected   bool
	}{
		{"1.2.3", ">=1.0.0", true},
		{"1.2.3", "^1.0.0", true},
		{"2.0.0", "^1.0.0", false},
		{"1.2.3", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.0.0", "=1.0.0", true},
		{"1.0.1", "=1.0.0", false},
		{"2.0.0", ">1.0.0", true},
		{"0.9.0", "<1.0.0", true},
		{"1.0.0", "*", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.version, tc.constraint), func(t *testing.T) {
			ok, err := registry.CheckVersionConstraint(tc.version, tc.constraint)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestSandboxWithPluginExecution_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := sandbox.DefaultConfig()
	sb := sandbox.NewInProcessSandbox(cfg)
	ctx := context.Background()

	p := &testPlugin{name: "sandboxed", version: "1.0.0"}

	// Execute health action
	result, err := sb.Execute(ctx, p, sandbox.Action{Name: "health"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Empty(t, result.Error)

	// Execute start action
	result, err = sb.Execute(ctx, p, sandbox.Action{Name: "start"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestStructuredParsingWithValidation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Parse JSON and validate
	jsonParser := structured.NewJSONParser()
	schema := &structured.Schema{
		Type: "object",
		Properties: map[string]*structured.Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	jsonStr := `{"name": "Alice", "age": 30}`
	data, err := jsonParser.Parse(jsonStr, schema)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Validate
	validator := structured.NewValidator(true)
	result, err := validator.ValidateJSON(jsonStr, schema)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestStructuredYAMLParsing_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	yamlParser := structured.NewYAMLParser()
	yamlStr := "name: Alice\nage: 30\n"

	data, err := yamlParser.Parse(yamlStr, nil)
	require.NoError(t, err)
	assert.NotNil(t, data)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Alice", m["name"])
}
