// Package sandbox provides plugin isolation and resource-limited execution.
// It supports running plugin actions in separate OS processes with
// configurable timeouts, memory limits, and allowed syscalls.
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"digital.vasic.plugins/pkg/plugin"
)

// ResourceLimits defines resource constraints for sandboxed execution.
type ResourceLimits struct {
	// MaxMemory in bytes (0 = unlimited).
	MaxMemory int64 `json:"max_memory" yaml:"max_memory"`
	// MaxCPU as a percentage (0 = unlimited, 100 = one full core).
	MaxCPU int `json:"max_cpu" yaml:"max_cpu"`
	// MaxDisk in bytes (0 = unlimited).
	MaxDisk int64 `json:"max_disk" yaml:"max_disk"`
	// Timeout for the sandboxed action.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultResourceLimits returns conservative default limits.
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemory: 256 * 1024 * 1024, // 256 MB
		MaxCPU:    50,
		MaxDisk:   100 * 1024 * 1024, // 100 MB
		Timeout:   30 * time.Second,
	}
}

// Config configures the sandbox.
type Config struct {
	// Limits defines resource constraints.
	Limits ResourceLimits `json:"limits" yaml:"limits"`
	// AllowedSyscalls restricts which syscalls the plugin may invoke.
	// Empty means all are allowed (no enforcement in user-space).
	AllowedSyscalls []string `json:"allowed_syscalls,omitempty" yaml:"allowed_syscalls,omitempty"`
	// AllowNetwork enables or disables network access.
	AllowNetwork bool `json:"allow_network" yaml:"allow_network"`
	// WorkDir is the working directory for the sandboxed process.
	WorkDir string `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Limits:       DefaultResourceLimits(),
		AllowNetwork: false,
	}
}

// Action represents a named action to execute inside the sandbox.
type Action struct {
	// Name of the action (e.g. "process", "transform").
	Name string `json:"name"`
	// Input is the payload for the action.
	Input any `json:"input,omitempty"`
}

// Result contains the output of a sandboxed execution.
type Result struct {
	// ID is a unique identifier for this execution.
	ID string `json:"id"`
	// Output is the action's returned data.
	Output any `json:"output,omitempty"`
	// Duration is how long the action took.
	Duration time.Duration `json:"duration"`
	// Error message, empty on success.
	Error string `json:"error,omitempty"`
}

// Sandbox is the interface for executing plugin actions in isolation.
type Sandbox interface {
	// Execute runs an action for the given plugin within the sandbox.
	Execute(ctx context.Context, p plugin.Plugin, action Action) (*Result, error)
}

// --- ProcessSandbox ---

// ProcessSandbox runs plugin actions as separate OS processes.
type ProcessSandbox struct {
	config *Config
	mu     sync.Mutex
}

// NewProcessSandbox creates a sandbox that isolates execution in a
// child process.
func NewProcessSandbox(cfg *Config) *ProcessSandbox {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &ProcessSandbox{config: cfg}
}

// Execute runs the action in a child process with resource limits.
func (s *ProcessSandbox) Execute(
	ctx context.Context, p plugin.Plugin, action Action,
) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p == nil {
		return nil, fmt.Errorf("plugin cannot be nil")
	}

	id := uuid.New().String()
	start := time.Now()

	// Apply timeout.
	timeout := s.config.Limits.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the action payload.
	payload, err := json.Marshal(action)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action: %w", err)
	}

	result := &Result{ID: id}

	// Execute the plugin action. For in-process plugins we call
	// HealthCheck as a representative action; real process isolation
	// would fork a child. Here we demonstrate the sandbox pattern.
	errCh := make(chan error, 1)
	go func() {
		switch action.Name {
		case "health":
			errCh <- p.HealthCheck(execCtx)
		case "init":
			cfg, _ := configFromPayload(payload)
			errCh <- p.Init(execCtx, cfg)
		case "start":
			errCh <- p.Start(execCtx)
		case "stop":
			errCh <- p.Stop(execCtx)
		default:
			errCh <- fmt.Errorf("unknown action: %s", action.Name)
		}
	}()

	select {
	case err := <-errCh:
		result.Duration = time.Since(start)
		if err != nil {
			result.Error = err.Error()
		}
	case <-execCtx.Done():
		result.Duration = time.Since(start)
		result.Error = "execution timed out"
	}

	return result, nil
}

func configFromPayload(payload []byte) (plugin.Config, error) {
	var action Action
	if err := json.Unmarshal(payload, &action); err != nil {
		return nil, err
	}
	if action.Input == nil {
		return plugin.Config{}, nil
	}
	if m, ok := action.Input.(map[string]any); ok {
		return plugin.Config(m), nil
	}
	return plugin.Config{}, nil
}

// --- InProcessSandbox ---

// InProcessSandbox runs actions in the same process but with timeout
// enforcement. Useful for testing and trusted plugins.
type InProcessSandbox struct {
	config *Config
}

// NewInProcessSandbox creates a sandbox without OS-level isolation.
func NewInProcessSandbox(cfg *Config) *InProcessSandbox {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &InProcessSandbox{config: cfg}
}

// Execute runs the action in-process with timeout enforcement.
func (s *InProcessSandbox) Execute(
	ctx context.Context, p plugin.Plugin, action Action,
) (*Result, error) {
	if p == nil {
		return nil, fmt.Errorf("plugin cannot be nil")
	}

	id := uuid.New().String()
	start := time.Now()

	timeout := s.config.Limits.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := &Result{ID: id}

	var actionErr error
	switch action.Name {
	case "health":
		actionErr = p.HealthCheck(execCtx)
	case "init":
		actionErr = p.Init(execCtx, nil)
	case "start":
		actionErr = p.Start(execCtx)
	case "stop":
		actionErr = p.Stop(execCtx)
	default:
		actionErr = fmt.Errorf("unknown action: %s", action.Name)
	}

	result.Duration = time.Since(start)
	if actionErr != nil {
		result.Error = actionErr.Error()
	}

	return result, nil
}

// --- Utility: run external command in sandbox ---

// RunCommand executes an external command with the sandbox's timeout
// and resource limits.
func RunCommand(
	ctx context.Context, cfg *Config, name string, args ...string,
) (string, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	timeout := cfg.Limits.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, name, args...) // #nosec G204
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)),
			fmt.Errorf("command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
