package benchmark

import (
	"context"
	"fmt"
	"testing"

	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
)

type benchPlugin struct {
	name    string
	version string
}

func (p *benchPlugin) Name() string                                  { return p.name }
func (p *benchPlugin) Version() string                               { return p.version }
func (p *benchPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (p *benchPlugin) Start(_ context.Context) error                 { return nil }
func (p *benchPlugin) Stop(_ context.Context) error                  { return nil }
func (p *benchPlugin) HealthCheck(_ context.Context) error           { return nil }

func BenchmarkRegistryRegister(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := registry.New()
		p := &benchPlugin{name: fmt.Sprintf("plugin-%d", i), version: "1.0.0"}
		_ = reg.Register(p)
	}
}

func BenchmarkRegistryGet(b *testing.B) {
	reg := registry.New()
	for i := 0; i < 100; i++ {
		p := &benchPlugin{name: fmt.Sprintf("plugin-%d", i), version: "1.0.0"}
		_ = reg.Register(p)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.Get(fmt.Sprintf("plugin-%d", i%100))
	}
}

func BenchmarkRegistryList(b *testing.B) {
	reg := registry.New()
	for i := 0; i < 100; i++ {
		p := &benchPlugin{name: fmt.Sprintf("plugin-%d", i), version: "1.0.0"}
		_ = reg.Register(p)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.List()
	}
}

func BenchmarkConfigGetString(b *testing.B) {
	cfg := plugin.Config{
		"host":  "localhost",
		"port":  8080,
		"debug": true,
		"rate":  0.95,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.GetString("host", "")
	}
}

func BenchmarkConfigGetInt(b *testing.B) {
	cfg := plugin.Config{
		"port": 8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.GetInt("port", 0)
	}
}

func BenchmarkStateTrackerGetSet(b *testing.B) {
	tracker := plugin.NewStateTracker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Set(plugin.Running)
		_ = tracker.Get()
	}
}

func BenchmarkJSONParsing(b *testing.B) {
	parser := structured.NewJSONParser()
	json := `{"name": "Alice", "age": 30, "active": true, "score": 0.95}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(json, nil)
	}
}

func BenchmarkYAMLParsing(b *testing.B) {
	parser := structured.NewYAMLParser()
	yaml := "name: Alice\nage: 30\nactive: true\nscore: 0.95\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(yaml, nil)
	}
}

func BenchmarkSchemaValidation(b *testing.B) {
	validator := structured.NewValidator(true)
	schema := &structured.Schema{
		Type: "object",
		Properties: map[string]*structured.Schema{
			"name":   {Type: "string"},
			"age":    {Type: "integer"},
			"active": {Type: "boolean"},
		},
		Required: []string{"name", "age"},
	}
	json := `{"name": "Alice", "age": 30, "active": true}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateJSON(json, schema)
	}
}

func BenchmarkVersionConstraintCheck(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.CheckVersionConstraint("1.2.3", ">=1.0.0")
	}
}

func BenchmarkSandboxInProcessExecute(b *testing.B) {
	cfg := sandbox.DefaultConfig()
	sb := sandbox.NewInProcessSandbox(cfg)
	ctx := context.Background()
	p := &benchPlugin{name: "bench", version: "1.0.0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sb.Execute(ctx, p, sandbox.Action{Name: "health"})
	}
}
