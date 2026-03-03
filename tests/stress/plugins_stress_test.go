package stress

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stressPlugin struct {
	name    string
	version string
}

func (p *stressPlugin) Name() string                                  { return p.name }
func (p *stressPlugin) Version() string                               { return p.version }
func (p *stressPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (p *stressPlugin) Start(_ context.Context) error                 { return nil }
func (p *stressPlugin) Stop(_ context.Context) error                  { return nil }
func (p *stressPlugin) HealthCheck(_ context.Context) error           { return nil }

func TestRegistryConcurrentRegisterAndGet_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	reg := registry.New()
	const goroutines = 100
	var wg sync.WaitGroup

	// Register plugins concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := &stressPlugin{
				name:    fmt.Sprintf("plugin-%d", idx),
				version: "1.0.0",
			}
			_ = reg.Register(p)
		}(i)
	}
	wg.Wait()

	// Verify all registered
	names := reg.List()
	assert.Equal(t, goroutines, len(names))

	// Concurrent Get operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p, ok := reg.Get(fmt.Sprintf("plugin-%d", idx))
			assert.True(t, ok)
			assert.NotNil(t, p)
		}(i)
	}
	wg.Wait()
}

func TestRegistryConcurrentListAndRemove_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	reg := registry.New()
	const total = 50

	for i := 0; i < total; i++ {
		p := &stressPlugin{name: fmt.Sprintf("p-%d", i), version: "1.0.0"}
		require.NoError(t, reg.Register(p))
	}

	var wg sync.WaitGroup
	// Concurrent List + Remove
	for i := 0; i < total; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = reg.List()
		}()
		go func(idx int) {
			defer wg.Done()
			_ = reg.Remove(fmt.Sprintf("p-%d", idx))
		}(i)
	}
	wg.Wait()

	// After removal, list should have fewer items
	remaining := reg.List()
	assert.LessOrEqual(t, len(remaining), total)
}

func TestStateTrackerConcurrentTransitions_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 100
	var wg sync.WaitGroup

	tracker := plugin.NewStateTracker()

	// Concurrent Set operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			states := []plugin.State{
				plugin.Uninitialized,
				plugin.Initialized,
				plugin.Running,
				plugin.Stopped,
			}
			tracker.Set(states[idx%len(states)])
		}(i)
	}
	wg.Wait()

	// State should be one of the valid states
	state := tracker.Get()
	assert.True(t, state >= plugin.Uninitialized && state <= plugin.Failed)
}

func TestSandboxConcurrentExecutions_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := sandbox.DefaultConfig()
	sb := sandbox.NewInProcessSandbox(cfg)
	ctx := context.Background()
	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p := &stressPlugin{
				name:    fmt.Sprintf("worker-%d", idx),
				version: "1.0.0",
			}
			result, err := sb.Execute(ctx, p, sandbox.Action{Name: "health"})
			assert.NoError(t, err)
			assert.NotEmpty(t, result.ID)
			assert.Empty(t, result.Error)
		}(i)
	}
	wg.Wait()
}

func TestJSONParserConcurrentParsing_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	parser := structured.NewJSONParser()
	const goroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			json := fmt.Sprintf(`{"id": %d, "name": "item-%d"}`, idx, idx)
			data, err := parser.Parse(json, nil)
			assert.NoError(t, err)
			assert.NotNil(t, data)
		}(i)
	}
	wg.Wait()
}

func TestVersionConstraintConcurrentChecks_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 100
	var wg sync.WaitGroup

	versions := []string{"1.0.0", "1.2.3", "2.0.0", "0.9.5", "3.1.4"}
	constraints := []string{">=1.0.0", "^1.0.0", "~1.2.0", "<2.0.0", "*"}

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v := versions[idx%len(versions)]
			c := constraints[idx%len(constraints)]
			_, err := registry.CheckVersionConstraint(v, c)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
}
