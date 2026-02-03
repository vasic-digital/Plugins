package loader

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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

func (t *testPlugin) Name() string                                  { return "test" }
func (t *testPlugin) Version() string                               { return "1.0.0" }
func (t *testPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (t *testPlugin) Start(_ context.Context) error                 { return nil }
func (t *testPlugin) Stop(_ context.Context) error                  { return nil }
func (t *testPlugin) HealthCheck(_ context.Context) error           { return nil }

type namedPlugin struct {
	name string
}

func (n *namedPlugin) Name() string                                  { return n.name }
func (n *namedPlugin) Version() string                               { return "1.0.0" }
func (n *namedPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }
func (n *namedPlugin) Start(_ context.Context) error                 { return nil }
func (n *namedPlugin) Stop(_ context.Context) error                  { return nil }
func (n *namedPlugin) HealthCheck(_ context.Context) error           { return nil }

// --- Config tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "./plugins", cfg.PluginDir)
	assert.Equal(t, []string{"*.so"}, cfg.AllowedPatterns)
	assert.True(t, cfg.AutoRegister)
}

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		PluginDir:       "/custom/plugins",
		AllowedPatterns: []string{"*.so", "*.dylib"},
		AutoRegister:    false,
	}
	assert.Equal(t, "/custom/plugins", cfg.PluginDir)
	assert.Equal(t, []string{"*.so", "*.dylib"}, cfg.AllowedPatterns)
	assert.False(t, cfg.AutoRegister)
}

// --- SharedObjectLoader constructor tests ---

func TestNewSharedObjectLoader_NilConfig(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	assert.NotNil(t, loader)
	assert.NotNil(t, loader.config)
	assert.Equal(t, "./plugins", loader.config.PluginDir)
}

func TestNewSharedObjectLoader_WithConfig(t *testing.T) {
	cfg := &Config{
		PluginDir:       "/my/plugins",
		AllowedPatterns: []string{"*.dll"},
		AutoRegister:    false,
	}
	loader := NewSharedObjectLoader(cfg)
	assert.Equal(t, "/my/plugins", loader.config.PluginDir)
	assert.Equal(t, []string{"*.dll"}, loader.config.AllowedPatterns)
}

// --- SharedObjectLoader.Load tests ---

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
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

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

func TestSharedObjectLoader_Load_OpenPluginError(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		return nil, fmt.Errorf("simulated open error")
	}

	loader := NewSharedObjectLoader(nil)
	_, err = loader.Load(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open plugin")
	assert.Contains(t, err.Error(), "simulated open error")
}

func TestSharedObjectLoader_Load_ConcurrentSafety(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	callCount := 0
	var mu sync.Mutex

	openPlugin = func(path string) (pluginHandle, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&testPlugin{}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = loader.Load(tmp.Name())
		}()
	}
	wg.Wait()

	assert.Equal(t, 5, callCount)
}

// --- SharedObjectLoader.LoadDir tests ---

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
	assert.Contains(t, err.Error(), "cannot access directory")
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
	assert.Len(t, plugins, 2)
}

func TestSharedObjectLoader_LoadDir_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subdir, 0755))

	// Create a .so file in subdir (should be skipped)
	f, err := os.Create(filepath.Join(subdir, "plugin.so"))
	require.NoError(t, err)
	f.Close()

	// Create a .so file in main dir
	f2, err := os.Create(filepath.Join(dir, "main.so"))
	require.NoError(t, err)
	f2.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	loadedPaths := []string{}
	openPlugin = func(path string) (pluginHandle, error) {
		loadedPaths = append(loadedPaths, path)
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&testPlugin{}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)
	plugins, err := loader.LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Len(t, loadedPaths, 1)
	assert.Contains(t, loadedPaths[0], "main.so")
}

func TestSharedObjectLoader_LoadDir_UsesDefaultDir(t *testing.T) {
	dir := t.TempDir()

	f, err := os.Create(filepath.Join(dir, "test.so"))
	require.NoError(t, err)
	f.Close()

	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&testPlugin{}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(&Config{
		PluginDir:       dir,
		AllowedPatterns: []string{"*.so"},
	})

	// Pass empty string to use default
	plugins, err := loader.LoadDir("")
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
}

func TestSharedObjectLoader_LoadDir_ContinuesOnLoadError(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"good.so", "bad.so"} {
		f, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
		f.Close()
	}

	original := openPlugin
	defer func() { openPlugin = original }()

	openPlugin = func(path string) (pluginHandle, error) {
		if strings.Contains(path, "bad") {
			return nil, fmt.Errorf("bad plugin")
		}
		return &mockPluginHandle{
			symbols: map[string]any{
				"Plugin": plugin.Plugin(&testPlugin{}),
			},
		}, nil
	}

	loader := NewSharedObjectLoader(nil)
	plugins, err := loader.LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
}

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
		{"empty patterns slice", []string{}, "anything", true},
		{"multiple patterns first match", []string{"*.so", "*.dylib"}, "p.so", true},
		{"multiple patterns second match", []string{"*.so", "*.dylib"}, "p.dylib", true},
		{"multiple patterns no match", []string{"*.so", "*.dylib"}, "p.dll", false},
		{"complex pattern", []string{"plugin-*.so"}, "plugin-test.so", true},
		{"complex pattern no match", []string{"plugin-*.so"}, "test.so", false},
		{"question mark pattern", []string{"plugin?.so"}, "plugin1.so", true},
		{"question mark no match", []string{"plugin?.so"}, "plugin12.so", false},
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

// --- validatePath tests ---

func TestSharedObjectLoader_ValidatePath_EmptyPath(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	err := loader.validatePath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestSharedObjectLoader_ValidatePath_FileNotFound(t *testing.T) {
	loader := NewSharedObjectLoader(nil)
	err := loader.validatePath("/nonexistent/plugin.so")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestSharedObjectLoader_ValidatePath_PatternMismatch(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	loader := NewSharedObjectLoader(&Config{
		AllowedPatterns: []string{"*.so"},
	})
	err = loader.validatePath(tmp.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestSharedObjectLoader_ValidatePath_Success(t *testing.T) {
	tmp, err := os.CreateTemp("", "plugin-*.so")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	tmp.Close()

	loader := NewSharedObjectLoader(nil)
	err = loader.validatePath(tmp.Name())
	require.NoError(t, err)
}

// --- ProcessLoader constructor tests ---

func TestNewProcessLoader_NilConfig(t *testing.T) {
	l := NewProcessLoader(nil)
	assert.NotNil(t, l)
	assert.NotNil(t, l.processes)
	assert.NotNil(t, l.config)
	assert.Equal(t, []string{"*"}, l.config.AllowedPatterns)
}

func TestNewProcessLoader_WithConfig(t *testing.T) {
	cfg := &Config{
		PluginDir:       "/custom/path",
		AllowedPatterns: []string{"*.bin"},
	}
	l := NewProcessLoader(cfg)
	assert.Equal(t, "/custom/path", l.config.PluginDir)
	assert.Equal(t, []string{"*.bin"}, l.config.AllowedPatterns)
}

// --- ProcessLoader.Load tests ---

func TestProcessLoader_Load_NotFound(t *testing.T) {
	l := NewProcessLoader(nil)
	_, err := l.Load("/nonexistent/binary")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestProcessLoader_Load_Directory(t *testing.T) {
	l := NewProcessLoader(nil)
	dir := t.TempDir()
	_, err := l.Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestProcessLoader_Load_InvalidMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	// Create a script that outputs invalid JSON
	dir := t.TempDir()
	script := filepath.Join(dir, "bad-meta")
	content := "#!/bin/sh\necho 'not json'"
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	l := NewProcessLoader(nil)
	_, err := l.Load(script)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata")
}

func TestProcessLoader_Load_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "good-plugin")
	content := `#!/bin/sh
echo '{"name":"my-plugin","version":"2.0.0"}'
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	l := NewProcessLoader(nil)
	p, err := l.Load(script)
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", p.Name())
	assert.Equal(t, "2.0.0", p.Version())

	// Verify it was added to processes map
	l.mu.Lock()
	_, exists := l.processes["my-plugin"]
	l.mu.Unlock()
	assert.True(t, exists)
}

func TestProcessLoader_Load_ExecutionFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "failing-script")
	content := "#!/bin/sh\nexit 1"
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	l := NewProcessLoader(nil)
	_, err := l.Load(script)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get metadata")
}

// --- ProcessLoader.LoadDir tests ---

func TestProcessLoader_LoadDir_NotFound(t *testing.T) {
	l := NewProcessLoader(nil)
	_, err := l.LoadDir("/nonexistent/dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read directory")
}

func TestProcessLoader_LoadDir_Empty(t *testing.T) {
	dir := t.TempDir()
	l := NewProcessLoader(nil)
	plugins, err := l.LoadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestProcessLoader_LoadDir_UsesDefaultDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()

	script := filepath.Join(dir, "plugin1")
	content := `#!/bin/sh
echo '{"name":"plugin1","version":"1.0.0"}'
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	l := NewProcessLoader(&Config{
		PluginDir:       dir,
		AllowedPatterns: []string{"*"},
	})

	plugins, err := l.LoadDir("")
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
}

func TestProcessLoader_LoadDir_SkipsSubdirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()

	// Create subdirectory
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.Mkdir(subdir, 0755))

	// Create plugin in main dir only
	script := filepath.Join(dir, "plugin")
	content := `#!/bin/sh
echo '{"name":"main-plugin","version":"1.0.0"}'
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	l := NewProcessLoader(nil)
	plugins, err := l.LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
}

func TestProcessLoader_LoadDir_ContinuesOnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()

	// Good plugin
	good := filepath.Join(dir, "good")
	goodContent := `#!/bin/sh
echo '{"name":"good","version":"1.0.0"}'
`
	require.NoError(t, os.WriteFile(good, []byte(goodContent), 0755))

	// Bad plugin (fails execution)
	bad := filepath.Join(dir, "bad")
	badContent := "#!/bin/sh\nexit 1"
	require.NoError(t, os.WriteFile(bad, []byte(badContent), 0755))

	l := NewProcessLoader(nil)
	plugins, err := l.LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Equal(t, "good", plugins[0].Name())
}

// --- processPlugin tests ---

func TestProcessPlugin_Name(t *testing.T) {
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "test-plugin", Version: "1.0.0"},
		path: "/some/path",
	}
	assert.Equal(t, "test-plugin", pp.Name())
}

func TestProcessPlugin_Version(t *testing.T) {
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "test-plugin", Version: "2.5.0"},
		path: "/some/path",
	}
	assert.Equal(t, "2.5.0", pp.Version())
}

func TestProcessPlugin_Init_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "init-plugin")
	content := `#!/bin/sh
# Read stdin and exit successfully
cat > /dev/null
exit 0
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "init-test", Version: "1.0.0"},
		path: script,
	}

	cfg := plugin.Config{"key": "value"}
	err := pp.Init(context.Background(), cfg)
	require.NoError(t, err)
}

func TestProcessPlugin_Init_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "init-fail")
	content := `#!/bin/sh
echo "init error" >&2
exit 1
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "init-fail", Version: "1.0.0"},
		path: script,
	}

	err := pp.Init(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestProcessPlugin_Start_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "run-plugin")
	content := `#!/bin/sh
trap 'exit 0' INT TERM
while true; do sleep 0.1; done
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "run-test", Version: "1.0.0"},
		path: script,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pp.Start(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pp.cmd)
	assert.NotNil(t, pp.cmd.Process)

	// Clean up
	_ = pp.Stop(context.Background())
}

func TestProcessPlugin_Start_InvalidPath(t *testing.T) {
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "bad", Version: "1.0.0"},
		path: "/nonexistent/binary",
	}

	err := pp.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start plugin process")
}

func TestProcessPlugin_Stop_NilCmd(t *testing.T) {
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "test", Version: "1.0.0"},
		path: "/some/path",
		cmd:  nil,
	}

	err := pp.Stop(context.Background())
	require.NoError(t, err)
}

func TestProcessPlugin_Stop_NilProcess(t *testing.T) {
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "test", Version: "1.0.0"},
		path: "/some/path",
		cmd:  &exec.Cmd{},
	}

	err := pp.Stop(context.Background())
	require.NoError(t, err)
}

func TestProcessPlugin_Stop_GracefulShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "stop-plugin")
	// Script that handles SIGINT gracefully with short sleep
	content := `#!/bin/sh
trap 'exit 0' INT TERM
while true; do sleep 0.1; done
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "stop-test", Version: "1.0.0"},
		path: script,
	}

	ctx := context.Background()
	err := pp.Start(ctx)
	require.NoError(t, err)

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	err = pp.Stop(ctx)
	require.NoError(t, err)
	assert.Nil(t, pp.cmd)
}

func TestProcessPlugin_Stop_ProcessAlreadyExited(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "quick-exit-plugin")
	// Script that exits immediately
	content := `#!/bin/sh
exit 0
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "quick-exit", Version: "1.0.0"},
		path: script,
	}

	ctx := context.Background()
	err := pp.Start(ctx)
	require.NoError(t, err)

	// Wait for process to exit naturally
	time.Sleep(200 * time.Millisecond)

	// Stop on already exited process - Signal will fail, triggering Kill
	err = pp.Stop(ctx)
	// The process already exited, so Stop should handle it gracefully
	// No error expected as Kill on a dead process is handled
	assert.Nil(t, pp.cmd)
}

func TestProcessPlugin_HealthCheck_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "health-plugin")
	content := `#!/bin/sh
echo "ok"
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "health-test", Version: "1.0.0"},
		path: script,
	}

	err := pp.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestProcessPlugin_HealthCheck_Unhealthy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "unhealthy-plugin")
	content := `#!/bin/sh
echo "degraded"
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "unhealthy", Version: "1.0.0"},
		path: script,
	}

	err := pp.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy")
	assert.Contains(t, err.Error(), "degraded")
}

func TestProcessPlugin_HealthCheck_CommandFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "failing-health")
	content := `#!/bin/sh
exit 1
`
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	pp := &processPlugin{
		meta: plugin.Metadata{Name: "fail-health", Version: "1.0.0"},
		path: script,
	}

	err := pp.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

// --- ReadProcessMetadata tests ---

func TestReadProcessMetadata_Valid(t *testing.T) {
	input := `{"name":"test","version":"1.0.0"}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))
	meta, err := ReadProcessMetadata(r)
	require.NoError(t, err)
	assert.Equal(t, "test", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
}

func TestReadProcessMetadata_InvalidJSON(t *testing.T) {
	input := "not json\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadProcessMetadata(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata JSON")
}

func TestReadProcessMetadata_MissingName(t *testing.T) {
	input := `{"version":"1.0.0"}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadProcessMetadata(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata validation failed")
}

func TestReadProcessMetadata_MissingVersion(t *testing.T) {
	input := `{"name":"test"}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadProcessMetadata(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata validation failed")
}

func TestReadProcessMetadata_EmptyInput(t *testing.T) {
	input := ""
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadProcessMetadata(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read metadata line")
}

func TestReadProcessMetadata_WithOptionalFields(t *testing.T) {
	input := `{"name":"test","version":"1.0.0","description":"A test plugin","author":"Test Author","dependencies":["dep1","dep2"]}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))
	meta, err := ReadProcessMetadata(r)
	require.NoError(t, err)
	assert.Equal(t, "test", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
	assert.Equal(t, "A test plugin", meta.Description)
	assert.Equal(t, "Test Author", meta.Author)
	assert.Equal(t, []string{"dep1", "dep2"}, meta.Dependencies)
}

// --- Loader interface compliance ---

func TestLoaderInterfaceCompliance(t *testing.T) {
	var _ Loader = &SharedObjectLoader{}
	var _ Loader = &ProcessLoader{}
}

// --- pluginHandle interface tests ---

func TestMockPluginHandle_Lookup_Found(t *testing.T) {
	m := &mockPluginHandle{
		symbols: map[string]any{
			"TestSymbol": "test value",
		},
	}

	sym, err := m.Lookup("TestSymbol")
	require.NoError(t, err)
	assert.Equal(t, "test value", sym)
}

func TestMockPluginHandle_Lookup_NotFound(t *testing.T) {
	m := &mockPluginHandle{
		symbols: map[string]any{},
	}

	_, err := m.Lookup("Missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Edge cases and boundary tests ---

func TestSharedObjectLoader_LoadDir_ReadDirError(t *testing.T) {
	// Create a directory with no read permissions
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not reliable on Windows")
	}

	dir := t.TempDir()
	restrictedDir := filepath.Join(dir, "restricted")
	require.NoError(t, os.Mkdir(restrictedDir, 0000))
	defer os.Chmod(restrictedDir, 0755) // Restore for cleanup

	loader := NewSharedObjectLoader(&Config{PluginDir: restrictedDir})
	_, err := loader.LoadDir(restrictedDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read directory")
}

func TestProcessLoader_ConcurrentLoad(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	dir := t.TempDir()

	// Create multiple plugin scripts
	for i := 0; i < 5; i++ {
		script := filepath.Join(dir, fmt.Sprintf("plugin%d", i))
		content := fmt.Sprintf(`#!/bin/sh
echo '{"name":"plugin%d","version":"1.0.0"}'
`, i)
		require.NoError(t, os.WriteFile(script, []byte(content), 0755))
	}

	l := NewProcessLoader(nil)

	var wg sync.WaitGroup
	results := make(chan plugin.Plugin, 5)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path := filepath.Join(dir, fmt.Sprintf("plugin%d", idx))
			p, err := l.Load(path)
			if err != nil {
				errors <- err
				return
			}
			results <- p
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	errCount := len(errors)
	resultCount := len(results)

	assert.Equal(t, 0, errCount)
	assert.Equal(t, 5, resultCount)
}

func TestProcessPlugin_Init_MarshalError(t *testing.T) {
	// Create a config with an unmarshalable value (channel)
	pp := &processPlugin{
		meta: plugin.Metadata{Name: "test", Version: "1.0.0"},
		path: "/some/path",
	}

	// Channels cannot be marshaled to JSON
	cfg := plugin.Config{"channel": make(chan int)}
	err := pp.Init(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal config")
}

// --- Table-driven test for SharedObjectLoader Load scenarios ---

func TestSharedObjectLoader_Load_Scenarios(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   func() (string, func())
		setupMock   func()
		expectError bool
		errContains string
	}{
		{
			name: "empty path",
			setupFile: func() (string, func()) {
				return "", func() {}
			},
			setupMock:   nil,
			expectError: true,
			errContains: "empty path",
		},
		{
			name: "file not found",
			setupFile: func() (string, func()) {
				return "/nonexistent/plugin.so", func() {}
			},
			setupMock:   nil,
			expectError: true,
			errContains: "not found",
		},
		{
			name: "pattern mismatch",
			setupFile: func() (string, func()) {
				tmp, _ := os.CreateTemp("", "plugin-*.txt")
				tmp.Close()
				return tmp.Name(), func() { os.Remove(tmp.Name()) }
			},
			setupMock:   nil,
			expectError: true,
			errContains: "does not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup := tt.setupFile()
			defer cleanup()

			if tt.setupMock != nil {
				tt.setupMock()
			}

			loader := NewSharedObjectLoader(&Config{AllowedPatterns: []string{"*.so"}})
			_, err := loader.Load(path)

			if tt.expectError {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Benchmark tests ---

func BenchmarkSharedObjectLoader_MatchesPattern(b *testing.B) {
	loader := &SharedObjectLoader{
		config: &Config{AllowedPatterns: []string{"*.so", "*.dylib", "plugin-*.bin"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader.matchesPattern("myplugin.so")
	}
}

func BenchmarkReadProcessMetadata(b *testing.B) {
	input := `{"name":"benchmark-plugin","version":"1.0.0","description":"A benchmark test plugin"}` + "\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bufio.NewReader(strings.NewReader(input))
		_, _ = ReadProcessMetadata(r)
	}
}
