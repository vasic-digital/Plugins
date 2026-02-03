// Package loader provides dynamic plugin loading from shared objects
// and external processes.
package loader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"digital.vasic.plugins/pkg/plugin"
)

// Loader is the interface for loading plugins from various sources.
type Loader interface {
	// Load loads a single plugin from the given path.
	Load(path string) (plugin.Plugin, error)
	// LoadDir loads all plugins in the given directory.
	LoadDir(dir string) ([]plugin.Plugin, error)
}

// Config holds configuration for plugin loaders.
type Config struct {
	// PluginDir is the default directory to scan for plugins.
	PluginDir string `json:"plugin_dir" yaml:"plugin_dir"`
	// AllowedPatterns restricts which files may be loaded (glob).
	AllowedPatterns []string `json:"allowed_patterns" yaml:"allowed_patterns"`
	// AutoRegister automatically registers loaded plugins.
	AutoRegister bool `json:"auto_register" yaml:"auto_register"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		PluginDir:       "./plugins",
		AllowedPatterns: []string{"*.so"},
		AutoRegister:    true,
	}
}

// SharedObjectLoader loads Go plugins compiled as shared objects (.so).
type SharedObjectLoader struct {
	config *Config
	mu     sync.Mutex
}

// NewSharedObjectLoader creates a loader for .so plugin files.
func NewSharedObjectLoader(cfg *Config) *SharedObjectLoader {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &SharedObjectLoader{config: cfg}
}

// Load opens a .so file at path, looks up the "Plugin" symbol, and
// returns it as a plugin.Plugin.
func (l *SharedObjectLoader) Load(path string) (plugin.Plugin, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.validatePath(path); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Use the Go plugin package. This only works on Linux/macOS with
	// -buildmode=plugin.
	p, err := openPlugin(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf(
			"plugin %s does not export Plugin symbol: %w", path, err,
		)
	}

	plg, ok := sym.(plugin.Plugin)
	if !ok {
		return nil, fmt.Errorf(
			"plugin %s: Plugin symbol does not implement plugin.Plugin", path,
		)
	}

	return plg, nil
}

// LoadDir walks the configured directory and loads all matching plugins.
func (l *SharedObjectLoader) LoadDir(dir string) ([]plugin.Plugin, error) {
	if dir == "" {
		dir = l.config.PluginDir
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot access directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	var plugins []plugin.Plugin
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !l.matchesPattern(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		p, err := l.Load(path)
		if err != nil {
			// Log and continue; do not fail the entire directory.
			continue
		}
		plugins = append(plugins, p)
	}

	return plugins, nil
}

// absPathFunc is a function type for getting absolute paths (for testing).
var absPathFunc = filepath.Abs

func (l *SharedObjectLoader) validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	absPath, err := absPathFunc(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	if !l.matchesPattern(filepath.Base(absPath)) {
		return fmt.Errorf(
			"file %s does not match allowed patterns", filepath.Base(absPath),
		)
	}
	return nil
}

func (l *SharedObjectLoader) matchesPattern(name string) bool {
	if len(l.config.AllowedPatterns) == 0 {
		return true
	}
	for _, pattern := range l.config.AllowedPatterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

// --- Process-based plugin loader ---

// ProcessLoader runs plugins as separate OS processes and communicates
// via stdin/stdout JSON-RPC.
type ProcessLoader struct {
	config    *Config
	processes map[string]*managedProcess
	mu        sync.Mutex
}

type managedProcess struct {
	cmd  *exec.Cmd
	name string
}

// NewProcessLoader creates a loader that runs plugins as external
// processes.
func NewProcessLoader(cfg *Config) *ProcessLoader {
	if cfg == nil {
		cfg = DefaultConfig()
		cfg.AllowedPatterns = []string{"*"}
	}
	return &ProcessLoader{
		config:    cfg,
		processes: make(map[string]*managedProcess),
	}
}

// Load starts the executable at path as a subprocess and wraps it in
// a processPlugin adapter.
func (l *ProcessLoader) Load(path string) (plugin.Plugin, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	absPath, err := absPathFunc(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory, not an executable", absPath)
	}

	// Start the process to get metadata.
	cmd := exec.Command(absPath, "--metadata") // #nosec G204
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata from %s: %w", absPath, err)
	}

	var meta plugin.Metadata
	if err := json.Unmarshal(output, &meta); err != nil {
		return nil, fmt.Errorf("invalid metadata from %s: %w", absPath, err)
	}

	pp := &processPlugin{
		meta: meta,
		path: absPath,
	}

	l.processes[meta.Name] = &managedProcess{
		name: meta.Name,
	}

	return pp, nil
}

// LoadDir loads all executables in the directory.
func (l *ProcessLoader) LoadDir(dir string) ([]plugin.Plugin, error) {
	if dir == "" {
		dir = l.config.PluginDir
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var plugins []plugin.Plugin
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		p, err := l.Load(path)
		if err != nil {
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// processPlugin wraps an external process as a plugin.Plugin.
type processPlugin struct {
	meta plugin.Metadata
	path string
	cmd  *exec.Cmd
	mu   sync.Mutex
}

func (p *processPlugin) Name() string    { return p.meta.Name }
func (p *processPlugin) Version() string { return p.meta.Version }

func (p *processPlugin) Init(_ context.Context, cfg plugin.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	cmd := exec.Command(p.path, "--init") // #nosec G204
	cmd.Stdin = strings.NewReader(string(data))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("init failed: %s: %w", string(output), err)
	}
	return nil
}

func (p *processPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cmd = exec.CommandContext(ctx, p.path, "--run") // #nosec G204
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin process: %w", err)
	}
	return nil
}

func (p *processPlugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return p.cmd.Process.Kill()
	}

	_ = p.cmd.Wait()
	p.cmd = nil
	return nil
}

func (p *processPlugin) HealthCheck(_ context.Context) error {
	cmd := exec.Command(p.path, "--health") // #nosec G204
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "ok" {
		return fmt.Errorf("unhealthy: %s", result)
	}
	return nil
}

// ReadProcessMetadata reads plugin metadata from a process's stdout.
// The process should print JSON metadata on the first line.
func ReadProcessMetadata(r *bufio.Reader) (*plugin.Metadata, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata line: %w", err)
	}

	var meta plugin.Metadata
	if err := json.Unmarshal([]byte(line), &meta); err != nil {
		return nil, fmt.Errorf("invalid metadata JSON: %w", err)
	}

	if err := meta.Validate(); err != nil {
		return nil, fmt.Errorf("metadata validation failed: %w", err)
	}

	return &meta, nil
}
