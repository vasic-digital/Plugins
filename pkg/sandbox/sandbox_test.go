package sandbox

import (
	"context"
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
