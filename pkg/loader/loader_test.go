package loader

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.plugins/pkg/plugin"
)

// --- mock for Go plugin system ---

type mockPluginHandle struct {
	symbols map[string]any
}

func (m *mockPluginHandle) Lookup(name string) (any, error) {
	sym, ok := m.symbols[name]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found", name)
	}
	return sym, nil
}

type testPlugin struct{}

func (t *testPlugin) Name() string                                     { return "test" }
func (t *testPlugin) Version() string                                  { return "1.0.0" }
func (t *testPlugin) Init(_ context.Context, _ plugin.Config) error    { return nil }
func (t *testPlugin) Start(_ context.Context) error                    { return nil }
func (t *testPlugin) Stop(_ context.Context) error                     { return nil }
func (t *testPlugin) HealthCheck(_ context.Context) error              { return nil }

// --- Config tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "./plugins", cfg.PluginDir)
	assert.Equal(t, []string{"*.so"}, cfg.AllowedPatterns)
	assert.True(t, cfg.AutoRegister)
}

// --- SharedObjectLoader tests ---

func TestSharedObjectLoader_Load_FileNotFound(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	_, err := loader.Load("/nonexistent/plugin.so")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSharedObjectLoader_Load_EmptyPath(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	_, err := loader.Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestSharedObjectLoader_Load_PatternMismatch(t *testing.T) {
	// Create a temp file with wrong extension.
	tmp, err := os.CreateTemp("", "plugin-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	loader := NewSharedObjectLoader(&Config{
		AllowedPatterns: []string{"*.so"},
	})
	_, err = loader.Load(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestSharedObjectLoader_Load_WithMock(t *testing.T) {
	// Create a temp .so file (just for path validation).
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	// Mock the openPlugin function.
	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&testPlugin{}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)
	p, err := loader.Load(tmp.Name())
	require.NoError(t, err)
	assert.Equal(t, "test", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
}

func TestSharedObjectLoader_Load_MissingSymbol(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		return &mockPluginHandle{symbols: map[string]any{}}, nil
	}

	loader := NewSharedObjectLoader(nil)
	_, err = loader.Load(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Plugin symbol")
}

func TestSharedObjectLoader_Load_WrongType(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": "not a plugin",
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)
	_, err = loader.Load(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement")
}

func TestSharedObjectLoader_LoadDir_NotDir(t *testing.T) {
	tmp, err := os.CreateTemp("", "not-a-dir")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	loader := NewSharedObjectLoader(nil)
	_, err = loader.LoadDir(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestSharedObjectLoader_LoadDir_NonExistent(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	_, err := loader.LoadDir("/nonexistent/dir")
	require.Error(t, err)
}

func TestSharedObjectLoader_LoadDir_Empty(t *testing.T) {
	dir := t.TempDir()
	loader := NewSharedObjectLoader(nil)
	plugins, err := loader.LoadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestSharedObjectLoader_LoadDir_WithPlugins(t *testing.T) {
	dir := t.TempDir()

	// Create some .so files.
	for _, name := range []string{"a.so", "b.so", "c.txt"} {
		f, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
		f.Close()
	}

	original := openPlugin
	defer func() { openPlugin = original }()

	counter := 0
	openPlugin = func(path string) (pluginHandle, error) {
		counter++
		name := fmt.Sprintf("plugin-%d", counter)
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&namedPlugin{name: name}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)
	plugins, err := loader.LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, plugins, 2) // Only .so files
}

type namedPlugin struct {
	name string
}

func (n *namedPlugin) Name() string                                     { return n.name }
func (n *namedPlugin) Version() string                                  { return "1.0.0" }
func (n *namedPlugin) Init(_ context.Context, _ plugin.Config) error    { return nil }
func (n *namedPlugin) Start(_ context.Context) error                    { return nil }
func (n *namedPlugin) Stop(_ context.Context) error                     { return nil }
func (n *namedPlugin) HealthCheck(_ context.Context) error              { return nil }

// --- matchesPattern tests ---

func TestSharedObjectLoader_MatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		file     string
		expected bool
	}{
		{"match .so", []string{"*.so"}, "myplugin.so", true},
		{"no match", []string{"*.so"}, "myplugin.dll", false},
		{"empty patterns", nil, "anything", true},
		{"multiple patterns", []string{"*.so", "*.dylib"}, "p.dylib", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &SharedObjectLoader{
				config: &Config{AllowedPatterns: tt.patterns},
			}
			assert.Equal(t, tt.expected, l.matchesPattern(tt.file))
		})
	}
}

// --- ProcessLoader tests ---

func TestNewProcessLoader(t *testing.T) {
	l := NewProcessLoader(nil)
	assert.NotNil(t, l)
	assert.NotNil(t, l.processes)
}

func TestProcessLoader_Load_NotFound(t *testing.T) {
	l := NewProcessLoader(nil)
	_, err := l.Load("/nonexistent/binary")
	require.Error(t, err)
}

func TestProcessLoader_LoadDir_NotFound(t *testing.T) {
	l := NewProcessLoader(nil)
	_, err := l.LoadDir("/nonexistent/dir")
	require.Error(t, err)
}

func TestProcessLoader_Load_Directory(t *testing.T) {
	l := NewProcessLoader(nil)
	dir := t.TempDir()
	_, err := l.Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

// --- ReadProcessMetadata tests ---

func TestReadProcessMetadata(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		expName string
	}{
		{
			name:    "valid",
			input:   `{"name":"test","version":"1.0.0"}` + "\n",
			wantErr: false,
			expName: "test",
		},
		{
			name:    "invalid json",
			input:   "not json\n",
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `{"version":"1.0.0"}` + "\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			meta, err := ReadProcessMetadata(r)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expName, meta.Name)
			}
		})
	}
}

// --- Loader interface compliance ---

func TestLoaderInterfaceCompliance(t *testing.T) {
	var _ Loader = &SharedObjectLoader{}
	var _ Loader = &ProcessLoader{}
}
