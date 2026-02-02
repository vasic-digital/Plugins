package plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- State tests ---

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{Uninitialized, "uninitialized"},
		{Initialized, "initialized"},
		{Running, "running"},
		{Stopped, "stopped"},
		{Failed, "failed"},
		{State(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// --- Metadata tests ---

func TestMetadata_Validate(t *testing.T) {
	tests := []struct {
		name    string
		meta    Metadata
		wantErr bool
	}{
		{
			name:    "valid metadata",
			meta:    Metadata{Name: "test", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "missing name",
			meta:    Metadata{Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "missing version",
			meta:    Metadata{Name: "test"},
			wantErr: true,
		},
		{
			name: "full metadata",
			meta: Metadata{
				Name:         "myplugin",
				Version:      "2.1.0",
				Description:  "A test plugin",
				Author:       "Author",
				Dependencies: []string{"dep1", "dep2"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meta.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- Config tests ---

func TestConfig_GetString(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		fallback string
		expected string
	}{
		{"existing key", Config{"k": "val"}, "k", "fb", "val"},
		{"missing key", Config{"k": "val"}, "other", "fb", "fb"},
		{"wrong type", Config{"k": 123}, "k", "fb", "fb"},
		{"nil config", nil, "k", "fb", "fb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetString(tt.key, tt.fallback))
		})
	}
}

func TestConfig_GetInt(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		fallback int
		expected int
	}{
		{"int value", Config{"k": 42}, "k", 0, 42},
		{"int64 value", Config{"k": int64(99)}, "k", 0, 99},
		{"float64 value", Config{"k": float64(7)}, "k", 0, 7},
		{"missing key", Config{}, "k", 5, 5},
		{"wrong type", Config{"k": "str"}, "k", 5, 5},
		{"nil config", nil, "k", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetInt(tt.key, tt.fallback))
		})
	}
}

func TestConfig_GetFloat64(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		fallback float64
		expected float64
	}{
		{"float64", Config{"k": 3.14}, "k", 0, 3.14},
		{"float32", Config{"k": float32(2.5)}, "k", 0, 2.5},
		{"int", Config{"k": 10}, "k", 0, 10.0},
		{"int64", Config{"k": int64(20)}, "k", 0, 20.0},
		{"missing", Config{}, "k", 1.5, 1.5},
		{"wrong type", Config{"k": "str"}, "k", 1.5, 1.5},
		{"nil config", nil, "k", 1.5, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.expected, tt.config.GetFloat64(tt.key, tt.fallback), 0.01)
		})
	}
}

func TestConfig_GetBool(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		fallback bool
		expected bool
	}{
		{"true", Config{"k": true}, "k", false, true},
		{"false", Config{"k": false}, "k", true, false},
		{"missing", Config{}, "k", true, true},
		{"wrong type", Config{"k": "yes"}, "k", false, false},
		{"nil config", nil, "k", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetBool(tt.key, tt.fallback))
		})
	}
}

func TestConfig_GetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		expected []string
	}{
		{
			"string slice",
			Config{"k": []string{"a", "b"}},
			"k",
			[]string{"a", "b"},
		},
		{
			"any slice with strings",
			Config{"k": []any{"x", "y"}},
			"k",
			[]string{"x", "y"},
		},
		{
			"any slice mixed",
			Config{"k": []any{"x", 123}},
			"k",
			[]string{"x"},
		},
		{"missing", Config{}, "k", nil},
		{"wrong type", Config{"k": 123}, "k", nil},
		{"nil config", nil, "k", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetStringSlice(tt.key)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConfig_Has(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		key      string
		expected bool
	}{
		{"present", Config{"k": "v"}, "k", true},
		{"absent", Config{"k": "v"}, "other", false},
		{"nil config", nil, "k", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.Has(tt.key))
		})
	}
}

// --- StateTracker tests ---

func TestStateTracker_Get(t *testing.T) {
	st := NewStateTracker()
	assert.Equal(t, Uninitialized, st.Get())
}

func TestStateTracker_Set(t *testing.T) {
	st := NewStateTracker()
	st.Set(Running)
	assert.Equal(t, Running, st.Get())
}

func TestStateTracker_Transition(t *testing.T) {
	tests := []struct {
		name     string
		initial  State
		expected State
		next     State
		wantErr  bool
	}{
		{"valid transition", Uninitialized, Uninitialized, Initialized, false},
		{"wrong current state", Uninitialized, Running, Stopped, true},
		{"to failed", Running, Running, Failed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := NewStateTracker()
			st.Set(tt.initial)
			err := st.Transition(tt.expected, tt.next)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.next, st.Get())
			}
		})
	}
}

// --- Mock plugin for interface verification ---

type mockPlugin struct {
	name    string
	version string
	initErr error
}

func (m *mockPlugin) Name() string    { return m.name }
func (m *mockPlugin) Version() string { return m.version }

func (m *mockPlugin) Init(_ context.Context, _ Config) error {
	return m.initErr
}

func (m *mockPlugin) Start(_ context.Context) error {
	return nil
}

func (m *mockPlugin) Stop(_ context.Context) error {
	return nil
}

func (m *mockPlugin) HealthCheck(_ context.Context) error {
	return nil
}

func TestPlugin_InterfaceCompliance(t *testing.T) {
	var p Plugin = &mockPlugin{name: "test", version: "1.0.0"}
	assert.Equal(t, "test", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
	assert.NoError(t, p.Init(context.Background(), nil))
	assert.NoError(t, p.Start(context.Background()))
	assert.NoError(t, p.Stop(context.Background()))
	assert.NoError(t, p.HealthCheck(context.Background()))
}

func TestPlugin_InitError(t *testing.T) {
	p := &mockPlugin{
		name:    "failing",
		version: "0.1.0",
		initErr: fmt.Errorf("init failed"),
	}
	err := p.Init(context.Background(), Config{"key": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}
