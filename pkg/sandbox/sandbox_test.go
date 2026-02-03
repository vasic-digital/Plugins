package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.plugins/pkg/plugin"
)

// --- mock plugin ---

type mockPlugin struct {
	name       string
	version    string
	healthErr  error
	initErr    error
	startErr   error
	stopErr    error
	healthTime time.Duration
}

func (m *mockPlugin) Name() string    { return m.name }
func (m *mockPlugin) Version() string { return m.version }

func (m *mockPlugin) Init(_ context.Context, _ plugin.Config) error {
	return m.initErr
}

func (m *mockPlugin) Start(_ context.Context) error {
	return m.startErr
}

func (m *mockPlugin) Stop(_ context.Context) error {
	return m.stopErr
}

func (m *mockPlugin) HealthCheck(ctx context.Context) error {
	if m.healthTime > 0 {
		select {
		case <-time.After(m.healthTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.healthErr
}

// --- ResourceLimits tests ---

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()
	assert.Equal(t, int64(256*1024*1024), limits.MaxMemory)
	assert.Equal(t, 50, limits.MaxCPU)
	assert.Equal(t, int64(100*1024*1024), limits.MaxDisk)
	assert.Equal(t, 30*time.Second, limits.Timeout)
}

// --- Config tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.False(t, cfg.AllowNetwork)
	assert.Equal(t, DefaultResourceLimits(), cfg.Limits)
}

// --- ProcessSandbox tests ---

func TestProcessSandbox_Execute_Health(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Empty(t, result.Error)
	assert.True(t, result.Duration > 0)
}

func TestProcessSandbox_Execute_HealthError(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{
		name:      "failing",
		version:   "1.0.0",
		healthErr: fmt.Errorf("unhealthy"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", result.Error)
}

func TestProcessSandbox_Execute_Init(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{
		Name:  "init",
		Input: map[string]any{"key": "value"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestProcessSandbox_Execute_Start(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "start"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestProcessSandbox_Execute_Stop(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "stop"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestProcessSandbox_Execute_UnknownAction(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "unknown"})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "unknown action")
}

func TestProcessSandbox_Execute_NilPlugin(t *testing.T) {
	sb := NewProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, Action{Name: "health"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestProcessSandbox_Execute_Timeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.Timeout = 50 * time.Millisecond
	sb := NewProcessSandbox(cfg)

	p := &mockPlugin{
		name:       "slow",
		version:    "1.0.0",
		healthTime: 5 * time.Second,
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error)
}

func TestProcessSandbox_Execute_StartError(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{
		name:     "fail-start",
		version:  "1.0.0",
		startErr: fmt.Errorf("start failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "start"})
	require.NoError(t, err)
	assert.Equal(t, "start failed", result.Error)
}

func TestProcessSandbox_Execute_StopError(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{
		name:    "fail-stop",
		version: "1.0.0",
		stopErr: fmt.Errorf("stop failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "stop"})
	require.NoError(t, err)
	assert.Equal(t, "stop failed", result.Error)
}

// --- InProcessSandbox tests ---

func TestInProcessSandbox_Execute_Health(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.NotEmpty(t, result.ID)
}

func TestInProcessSandbox_Execute_NilPlugin(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	_, err := sb.Execute(context.Background(), nil, Action{Name: "health"})
	require.Error(t, err)
}

func TestInProcessSandbox_Execute_UnknownAction(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "bad"})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "unknown action")
}

func TestInProcessSandbox_Execute_AllActions(t *testing.T) {
	actions := []string{"health", "init", "start", "stop"}
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, err := sb.Execute(
				context.Background(), p, Action{Name: action},
			)
			require.NoError(t, err)
			assert.Empty(t, result.Error)
		})
	}
}

// --- RunCommand tests ---

func TestRunCommand_Echo(t *testing.T) {
	output, err := RunCommand(
		context.Background(), nil, "echo", "hello",
	)
	require.NoError(t, err)
	assert.Equal(t, "hello", output)
}

func TestRunCommand_Failure(t *testing.T) {
	_, err := RunCommand(
		context.Background(), nil, "false",
	)
	require.Error(t, err)
}

func TestRunCommand_NotFound(t *testing.T) {
	_, err := RunCommand(
		context.Background(), nil, "nonexistent-binary-xyz",
	)
	require.Error(t, err)
}

func TestRunCommand_WithWorkDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = "/tmp"
	output, err := RunCommand(
		context.Background(), cfg, "pwd",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "tmp")
}

func TestRunCommand_Timeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.Timeout = 50 * time.Millisecond
	_, err := RunCommand(
		context.Background(), cfg, "sleep", "10",
	)
	require.Error(t, err)
}

// --- Sandbox interface compliance ---

func TestSandboxInterfaceCompliance(t *testing.T) {
	var _ Sandbox = &ProcessSandbox{}
	var _ Sandbox = &InProcessSandbox{}
}

// --- Additional coverage tests ---

func TestProcessSandbox_Execute_InitError(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{
		name:    "fail-init",
		version: "1.0.0",
		initErr: fmt.Errorf("init failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{
		Name:  "init",
		Input: map[string]any{"key": "value"},
	})
	require.NoError(t, err)
	assert.Equal(t, "init failed", result.Error)
}

func TestInProcessSandbox_Execute_InitError(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{
		name:    "fail-init",
		version: "1.0.0",
		initErr: fmt.Errorf("init failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "init"})
	require.NoError(t, err)
	assert.Equal(t, "init failed", result.Error)
}

func TestInProcessSandbox_Execute_StartError(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{
		name:     "fail-start",
		version:  "1.0.0",
		startErr: fmt.Errorf("start failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "start"})
	require.NoError(t, err)
	assert.Equal(t, "start failed", result.Error)
}

func TestInProcessSandbox_Execute_StopError(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{
		name:    "fail-stop",
		version: "1.0.0",
		stopErr: fmt.Errorf("stop failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "stop"})
	require.NoError(t, err)
	assert.Equal(t, "stop failed", result.Error)
}

func TestInProcessSandbox_Execute_HealthError(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{
		name:      "fail-health",
		version:   "1.0.0",
		healthErr: fmt.Errorf("health failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Equal(t, "health failed", result.Error)
}

func TestConfigFromPayload_ValidInput(t *testing.T) {
	action := Action{
		Name:  "init",
		Input: map[string]any{"key": "value"},
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)
	assert.Equal(t, "value", cfg["key"])
}

func TestConfigFromPayload_NilInput(t *testing.T) {
	action := Action{
		Name:  "init",
		Input: nil,
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)
	assert.Empty(t, cfg)
}

func TestConfigFromPayload_InvalidJSON(t *testing.T) {
	_, err := configFromPayload([]byte("invalid json"))
	assert.Error(t, err)
}

func TestConfigFromPayload_NonMapInput(t *testing.T) {
	action := Action{
		Name:  "init",
		Input: "string input",
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)
	// Non-map input returns empty config
	assert.Empty(t, cfg)
}

func TestProcessSandbox_Execute_WithZeroTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: 0, // Zero timeout should default to 30s
		},
	}
	sb := NewProcessSandbox(cfg)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestInProcessSandbox_Execute_WithZeroTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: 0, // Zero timeout should default to 30s
		},
	}
	sb := NewInProcessSandbox(cfg)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestRunCommand_WithZeroTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: 0, // Zero timeout should default to 30s
		},
	}

	output, err := RunCommand(context.Background(), cfg, "echo", "test")
	require.NoError(t, err)
	assert.Equal(t, "test", output)
}

func TestResourceLimits_CustomValues(t *testing.T) {
	limits := ResourceLimits{
		MaxMemory: 512 * 1024 * 1024,
		MaxCPU:    100,
		MaxDisk:   200 * 1024 * 1024,
		Timeout:   60 * time.Second,
	}

	assert.Equal(t, int64(512*1024*1024), limits.MaxMemory)
	assert.Equal(t, 100, limits.MaxCPU)
	assert.Equal(t, int64(200*1024*1024), limits.MaxDisk)
	assert.Equal(t, 60*time.Second, limits.Timeout)
}

func TestConfig_AllFields(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			MaxMemory: 128 * 1024 * 1024,
			MaxCPU:    25,
			MaxDisk:   50 * 1024 * 1024,
			Timeout:   10 * time.Second,
		},
		AllowedSyscalls: []string{"read", "write"},
		AllowNetwork:    true,
		WorkDir:         "/tmp/sandbox",
	}

	assert.Equal(t, int64(128*1024*1024), cfg.Limits.MaxMemory)
	assert.Equal(t, 25, cfg.Limits.MaxCPU)
	assert.True(t, cfg.AllowNetwork)
	assert.Equal(t, "/tmp/sandbox", cfg.WorkDir)
	assert.Len(t, cfg.AllowedSyscalls, 2)
}

func TestAction_Struct(t *testing.T) {
	action := Action{
		Name:  "process",
		Input: map[string]any{"data": "test"},
	}

	assert.Equal(t, "process", action.Name)
	m, ok := action.Input.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test", m["data"])
}

func TestResult_Struct(t *testing.T) {
	result := Result{
		ID:       "123",
		Output:   map[string]any{"result": "ok"},
		Duration: 100 * time.Millisecond,
		Error:    "",
	}

	assert.Equal(t, "123", result.ID)
	assert.NotNil(t, result.Output)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
	assert.Empty(t, result.Error)
}

func TestProcessSandbox_Execute_ContextTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.Timeout = 50 * time.Millisecond
	sb := NewProcessSandbox(cfg)

	p := &mockPlugin{
		name:       "slow",
		version:    "1.0.0",
		healthTime: 10 * time.Second,
	}

	// Use a context that's already close to timing out
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := sb.Execute(ctx, p, Action{Name: "health"})
	require.NoError(t, err)
	// Should timeout
	assert.NotEmpty(t, result.Error)
}

func TestInProcessSandbox_Execute_Timeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.Timeout = 50 * time.Millisecond
	sb := NewInProcessSandbox(cfg)

	p := &mockPlugin{
		name:       "slow",
		version:    "1.0.0",
		healthTime: 5 * time.Second,
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	// Should complete (context deadline exceeded passed to plugin)
	assert.Contains(t, result.Error, "context deadline exceeded")
}

func TestProcessSandbox_Execute_InitWithComplexPayload(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	result, err := sb.Execute(context.Background(), p, Action{
		Name: "init",
		Input: map[string]any{
			"string_key":  "value",
			"int_key":     42,
			"bool_key":    true,
			"nested_key":  map[string]any{"inner": "value"},
			"array_key":   []any{1, 2, 3},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestRunCommand_WithComplexArgs(t *testing.T) {
	output, err := RunCommand(
		context.Background(), nil,
		"echo", "-n", "hello world",
	)
	require.NoError(t, err)
	assert.Contains(t, output, "hello")
}

func TestProcessSandbox_Concurrency(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := sb.Execute(context.Background(), p, Action{Name: "health"})
			assert.NoError(t, err)
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestProcessSandbox_Execute_ErrorFromActionChannel(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{
		name:      "fail-health",
		version:   "1.0.0",
		healthErr: fmt.Errorf("health check failed"),
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Equal(t, "health check failed", result.Error)
	assert.True(t, result.Duration > 0)
}

func TestProcessSandbox_Execute_TimeoutPath(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: 10 * time.Millisecond,
		},
	}
	sb := NewProcessSandbox(cfg)

	// Plugin that takes longer than timeout
	p := &mockPlugin{
		name:       "slow-plugin",
		version:    "1.0.0",
		healthTime: 5 * time.Second,
	}

	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "timed out")
}

func TestConfigFromPayload_ComplexMap(t *testing.T) {
	action := Action{
		Name: "init",
		Input: map[string]any{
			"string_key":  "value",
			"int_key":     float64(42), // JSON numbers are float64
			"bool_key":    true,
			"nested_key":  map[string]any{"inner": "value"},
			"array_key":   []any{float64(1), float64(2), float64(3)},
		},
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)

	assert.Equal(t, "value", cfg["string_key"])
	assert.Equal(t, float64(42), cfg["int_key"])
	assert.Equal(t, true, cfg["bool_key"])
}

func TestConfigFromPayload_ArrayInput(t *testing.T) {
	action := Action{
		Name:  "init",
		Input: []any{"a", "b", "c"}, // Array input (not map)
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)
	// Non-map input returns empty config
	assert.Empty(t, cfg)
}

func TestConfigFromPayload_IntInput(t *testing.T) {
	action := Action{
		Name:  "init",
		Input: 42, // Int input (not map)
	}
	payload, err := json.Marshal(action)
	require.NoError(t, err)

	cfg, err := configFromPayload(payload)
	require.NoError(t, err)
	// Non-map input returns empty config
	assert.Empty(t, cfg)
}

func TestRunCommand_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := RunCommand(ctx, nil, "echo", "test")
	assert.Error(t, err)
}

func TestInProcessSandbox_Execute_WithNegativeTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: -1 * time.Second, // Negative timeout
		},
	}
	sb := NewInProcessSandbox(cfg)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	// Should default to 30 seconds
	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestProcessSandbox_Execute_WithNegativeTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: -1 * time.Second, // Negative timeout
		},
	}
	sb := NewProcessSandbox(cfg)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	// Should default to 30 seconds
	result, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
}

func TestRunCommand_WithNegativeTimeout(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			Timeout: -1 * time.Second, // Negative timeout
		},
	}

	// Should default to 30 seconds
	output, err := RunCommand(context.Background(), cfg, "echo", "test")
	require.NoError(t, err)
	assert.Equal(t, "test", output)
}

func TestProcessSandbox_Execute_AllActionsSuccess(t *testing.T) {
	sb := NewProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	actions := []Action{
		{Name: "health"},
		{Name: "init", Input: map[string]any{"key": "value"}},
		{Name: "start"},
		{Name: "stop"},
	}

	for _, action := range actions {
		t.Run(action.Name, func(t *testing.T) {
			result, err := sb.Execute(context.Background(), p, action)
			require.NoError(t, err)
			assert.NotEmpty(t, result.ID)
			assert.Empty(t, result.Error)
		})
	}
}

func TestInProcessSandbox_Concurrency(t *testing.T) {
	sb := NewInProcessSandbox(nil)
	p := &mockPlugin{name: "test", version: "1.0.0"}

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := sb.Execute(context.Background(), p, Action{Name: "health"})
			assert.NoError(t, err)
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestNewProcessSandbox_WithConfig(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			MaxMemory: 100 * 1024 * 1024,
			MaxCPU:    25,
			MaxDisk:   50 * 1024 * 1024,
			Timeout:   10 * time.Second,
		},
		AllowNetwork:    true,
		AllowedSyscalls: []string{"read", "write"},
		WorkDir:         "/tmp",
	}

	sb := NewProcessSandbox(cfg)
	assert.NotNil(t, sb)
	assert.Equal(t, cfg, sb.config)
}

func TestNewInProcessSandbox_WithConfig(t *testing.T) {
	cfg := &Config{
		Limits: ResourceLimits{
			MaxMemory: 100 * 1024 * 1024,
			Timeout:   10 * time.Second,
		},
	}

	sb := NewInProcessSandbox(cfg)
	assert.NotNil(t, sb)
	assert.Equal(t, cfg, sb.config)
}

// --- Tests for dependency injection error paths ---

func TestProcessSandbox_Execute_MarshalError_DI(t *testing.T) {
	sb := NewProcessSandbox(nil)
	// Inject a failing JSON marshaler.
	sb.marshalJSON = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("simulated marshal error")
	}

	p := &mockPlugin{name: "test", version: "1.0.0"}

	_, err := sb.Execute(context.Background(), p, Action{Name: "health"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal action")
	assert.Contains(t, err.Error(), "simulated marshal error")
}
