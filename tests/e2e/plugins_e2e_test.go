package e2e

import (
	"context"
	"testing"

	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type e2ePlugin struct {
	name    string
	version string
	state   plugin.State
}

func (p *e2ePlugin) Name() string    { return p.name }
func (p *e2ePlugin) Version() string { return p.version }
func (p *e2ePlugin) Init(_ context.Context, _ plugin.Config) error {
	p.state = plugin.Initialized
	return nil
}
func (p *e2ePlugin) Start(_ context.Context) error {
	p.state = plugin.Running
	return nil
}
func (p *e2ePlugin) Stop(_ context.Context) error {
	p.state = plugin.Stopped
	return nil
}
func (p *e2ePlugin) HealthCheck(_ context.Context) error { return nil }

func TestFullPluginLifecyclePipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	ctx := context.Background()
	reg := registry.New()

	// Step 1: Create and register plugins
	p1 := &e2ePlugin{name: "database", version: "2.1.0"}
	p2 := &e2ePlugin{name: "api-gateway", version: "1.3.0"}
	p3 := &e2ePlugin{name: "monitoring", version: "1.0.0"}

	require.NoError(t, reg.Register(p1))
	require.NoError(t, reg.Register(p2))
	require.NoError(t, reg.Register(p3))

	// Step 2: Set dependencies
	require.NoError(t, reg.SetDependencies("api-gateway", []string{"database"}))
	require.NoError(t, reg.SetDependencies("monitoring", []string{"database"}))

	// Step 3: Start all
	err := reg.StartAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, plugin.Running, p1.state)
	assert.Equal(t, plugin.Running, p2.state)
	assert.Equal(t, plugin.Running, p3.state)

	// Step 4: Verify version constraints
	ok, err := registry.CheckVersionConstraint(p1.Version(), ">=2.0.0")
	require.NoError(t, err)
	assert.True(t, ok)

	// Step 5: Stop all
	err = reg.StopAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, plugin.Stopped, p1.state)
}

func TestSandboxExecutionPipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	ctx := context.Background()
	cfg := sandbox.DefaultConfig()
	sb := sandbox.NewInProcessSandbox(cfg)
	p := &e2ePlugin{name: "worker", version: "1.0.0"}

	// Init -> Start -> Health -> Stop lifecycle
	actions := []string{"init", "start", "health", "stop"}
	for _, action := range actions {
		result, err := sb.Execute(ctx, p, sandbox.Action{Name: action})
		require.NoError(t, err, "action %s should not error", action)
		assert.NotEmpty(t, result.ID)
		assert.Empty(t, result.Error, "action %s should succeed", action)
		assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
	}
}

func TestStructuredOutputPipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Step 1: Define schema
	schema := &structured.Schema{
		Type: "object",
		Properties: map[string]*structured.Schema{
			"status":  {Type: "string"},
			"code":    {Type: "integer"},
			"message": {Type: "string"},
		},
		Required: []string{"status", "code"},
	}

	// Step 2: Parse JSON output
	jsonParser := structured.NewJSONParser()
	jsonStr := `{"status": "ok", "code": 200, "message": "success"}`
	data, err := jsonParser.Parse(jsonStr, schema)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Step 3: Validate
	validator := structured.NewValidator(true)
	result, err := validator.ValidateJSON(jsonStr, schema)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)

	// Step 4: Validate invalid input
	invalidJSON := `{"status": "error"}`
	result, err = validator.ValidateJSON(invalidJSON, schema)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestMarkdownParsingPipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	mdParser := structured.NewMarkdownParser()
	mdOutput := "- **Name**: Alice\n- **Role**: Engineer\n- **Team**: Backend\n"

	data, err := mdParser.Parse(mdOutput, nil)
	require.NoError(t, err)

	m, ok := data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Alice", m["Name"])
	assert.Equal(t, "Engineer", m["Role"])
	assert.Equal(t, "Backend", m["Team"])
}

func TestConfigAccessorsPipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	cfg := plugin.Config{
		"host":     "localhost",
		"port":     8080,
		"debug":    true,
		"rate":     0.95,
		"features": []string{"cache", "metrics"},
	}

	assert.Equal(t, "localhost", cfg.GetString("host", ""))
	assert.Equal(t, 8080, cfg.GetInt("port", 0))
	assert.True(t, cfg.GetBool("debug", false))
	assert.InDelta(t, 0.95, cfg.GetFloat64("rate", 0), 0.001)
	assert.True(t, cfg.Has("host"))
	assert.False(t, cfg.Has("missing"))
	assert.Equal(t, "default", cfg.GetString("missing", "default"))
}
