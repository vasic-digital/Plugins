// Package plugin provides core interfaces and types for the plugin system.
// It defines the Plugin interface, Metadata, State, and Config types that
// form the foundation of the plugin lifecycle.
package plugin

import (
	"context"
	"fmt"
	"sync"
)

// State represents the current state of a plugin.
type State int

const (
	// Uninitialized is the default state before Init is called.
	Uninitialized State = iota
	// Initialized means Init completed successfully.
	Initialized
	// Running means Start completed successfully.
	Running
	// Stopped means Stop completed successfully.
	Stopped
	// Failed means the plugin encountered a fatal error.
	Failed
)

// String returns a human-readable name for the state.
func (s State) String() string {
	switch s {
	case Uninitialized:
		return "uninitialized"
	case Initialized:
		return "initialized"
	case Running:
		return "running"
	case Stopped:
		return "stopped"
	case Failed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Plugin defines the interface that all plugins must implement.
type Plugin interface {
	// Name returns the unique name of the plugin.
	Name() string
	// Version returns the semantic version of the plugin.
	Version() string
	// Init initializes the plugin with the given configuration.
	Init(ctx context.Context, config Config) error
	// Start starts the plugin.
	Start(ctx context.Context) error
	// Stop gracefully stops the plugin.
	Stop(ctx context.Context) error
	// HealthCheck returns nil if the plugin is healthy.
	HealthCheck(ctx context.Context) error
}

// Metadata contains descriptive information about a plugin.
type Metadata struct {
	Name         string   `json:"name" yaml:"name"`
	Version      string   `json:"version" yaml:"version"`
	Description  string   `json:"description" yaml:"description"`
	Author       string   `json:"author" yaml:"author"`
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// Validate checks that required metadata fields are present.
func (m *Metadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("metadata name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("metadata version is required")
	}
	return nil
}

// Config is a map-based configuration with typed accessor methods.
type Config map[string]any

// GetString returns the string value for key, or the fallback if missing
// or not a string.
func (c Config) GetString(key, fallback string) string {
	if c == nil {
		return fallback
	}
	v, ok := c[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

// GetInt returns the int value for key, or the fallback if missing or
// not convertible to int.
func (c Config) GetInt(key string, fallback int) int {
	if c == nil {
		return fallback
	}
	v, ok := c[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}

// GetFloat64 returns the float64 value for key, or the fallback if
// missing or not convertible.
func (c Config) GetFloat64(key string, fallback float64) float64 {
	if c == nil {
		return fallback
	}
	v, ok := c[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return fallback
	}
}

// GetBool returns the bool value for key, or the fallback if missing
// or not a bool.
func (c Config) GetBool(key string, fallback bool) bool {
	if c == nil {
		return fallback
	}
	v, ok := c[key]
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

// GetStringSlice returns a []string for key, or nil if missing or not
// convertible.
func (c Config) GetStringSlice(key string) []string {
	if c == nil {
		return nil
	}
	v, ok := c[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

// Has reports whether the config contains the given key.
func (c Config) Has(key string) bool {
	if c == nil {
		return false
	}
	_, ok := c[key]
	return ok
}

// StateTracker tracks plugin state transitions in a thread-safe manner.
type StateTracker struct {
	mu    sync.RWMutex
	state State
}

// NewStateTracker creates a new tracker starting in Uninitialized.
func NewStateTracker() *StateTracker {
	return &StateTracker{state: Uninitialized}
}

// Get returns the current state.
func (t *StateTracker) Get() State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// Set sets the state to the given value.
func (t *StateTracker) Set(s State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = s
}

// Transition attempts to move from expected to next. Returns an error
// if the current state does not match expected.
func (t *StateTracker) Transition(expected, next State) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != expected {
		return fmt.Errorf(
			"invalid state transition: expected %s, got %s",
			expected, t.state,
		)
	}
	t.state = next
	return nil
}
