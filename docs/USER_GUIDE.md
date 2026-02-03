# User Guide

## Overview

The `digital.vasic.plugins` module provides a complete plugin system for Go applications. It covers five core concerns:

1. **Plugin Interface** -- A standard contract for plugin lifecycle (init, start, stop, health check)
2. **Registry** -- Thread-safe plugin management with dependency-ordered startup and shutdown
3. **Loader** -- Dynamic loading from shared objects (`.so`) or external processes
4. **Sandbox** -- Resource-limited, timeout-enforced isolated execution
5. **Structured Output** -- Parsing and validating JSON, YAML, and Markdown output with schema support

## Installation

```bash
go get digital.vasic.plugins@latest
```

Requires Go 1.24 or later.

## Package Imports

```go
import (
    "digital.vasic.plugins/pkg/plugin"
    "digital.vasic.plugins/pkg/registry"
    "digital.vasic.plugins/pkg/loader"
    "digital.vasic.plugins/pkg/sandbox"
    "digital.vasic.plugins/pkg/structured"
)
```

---

## 1. Plugin Interface

The `plugin.Plugin` interface is the core contract that all plugins must implement:

```go
type Plugin interface {
    Name() string
    Version() string
    Init(ctx context.Context, config Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    HealthCheck(ctx context.Context) error
}
```

### Implementing a Plugin

```go
package myplugin

import (
    "context"
    "fmt"

    "digital.vasic.plugins/pkg/plugin"
)

type MyPlugin struct {
    state  *plugin.StateTracker
    config plugin.Config
}

func New() *MyPlugin {
    return &MyPlugin{
        state: plugin.NewStateTracker(),
    }
}

func (p *MyPlugin) Name() string    { return "my-plugin" }
func (p *MyPlugin) Version() string { return "1.0.0" }

func (p *MyPlugin) Init(ctx context.Context, cfg plugin.Config) error {
    if err := p.state.Transition(plugin.Uninitialized, plugin.Initialized); err != nil {
        return fmt.Errorf("init failed: %w", err)
    }
    p.config = cfg
    return nil
}

func (p *MyPlugin) Start(ctx context.Context) error {
    if err := p.state.Transition(plugin.Initialized, plugin.Running); err != nil {
        return fmt.Errorf("start failed: %w", err)
    }
    // Start plugin work...
    return nil
}

func (p *MyPlugin) Stop(ctx context.Context) error {
    if err := p.state.Transition(plugin.Running, plugin.Stopped); err != nil {
        return fmt.Errorf("stop failed: %w", err)
    }
    // Clean up resources...
    return nil
}

func (p *MyPlugin) HealthCheck(ctx context.Context) error {
    if p.state.Get() != plugin.Running {
        return fmt.Errorf("plugin is not running: %s", p.state.Get())
    }
    return nil
}
```

### Plugin States

Plugins transition through a defined lifecycle:

```
Uninitialized --> Initialized --> Running --> Stopped
                                    |
                                    v
                                  Failed
```

The `StateTracker` enforces valid transitions:

```go
tracker := plugin.NewStateTracker()
tracker.Get()                                              // Uninitialized
tracker.Transition(plugin.Uninitialized, plugin.Initialized) // OK
tracker.Transition(plugin.Running, plugin.Stopped)           // Error: expected Running, got Initialized
```

### Plugin Configuration

`Config` is a `map[string]any` with typed accessor methods:

```go
cfg := plugin.Config{
    "host":       "localhost",
    "port":       8080,
    "debug":      true,
    "timeout":    3.5,
    "allowed":    []string{"read", "write"},
}

host := cfg.GetString("host", "127.0.0.1")   // "localhost"
port := cfg.GetInt("port", 3000)              // 8080
debug := cfg.GetBool("debug", false)          // true
timeout := cfg.GetFloat64("timeout", 1.0)     // 3.5
allowed := cfg.GetStringSlice("allowed")      // ["read", "write"]
exists := cfg.Has("host")                     // true
```

### Plugin Metadata

Metadata provides descriptive information for discovery and dependency declaration:

```go
meta := plugin.Metadata{
    Name:         "my-plugin",
    Version:      "1.0.0",
    Description:  "A sample plugin",
    Author:       "Author Name",
    Dependencies: []string{"database-plugin", "cache-plugin"},
}

if err := meta.Validate(); err != nil {
    // Name and Version are required
}
```

---

## 2. Registry

The registry manages plugin registration, dependency resolution, and ordered lifecycle.

### Basic Usage

```go
ctx := context.Background()

reg := registry.New()

// Register plugins
reg.Register(databasePlugin)
reg.Register(cachePlugin)
reg.Register(appPlugin)

// Declare dependencies (appPlugin depends on database and cache)
reg.SetDependencies("app-plugin", []string{"database-plugin", "cache-plugin"})

// Start all in dependency order:
// database-plugin, cache-plugin start first, then app-plugin
if err := reg.StartAll(ctx); err != nil {
    log.Fatalf("startup failed: %v", err)
}

// Stop all in reverse dependency order:
// app-plugin stops first, then cache-plugin, database-plugin
defer reg.StopAll(ctx)
```

### Querying the Registry

```go
// List all registered plugin names
names := reg.List()

// Get a specific plugin
p, found := reg.Get("database-plugin")
if found {
    fmt.Println(p.Version())
}

// Remove a plugin
err := reg.Remove("old-plugin")
```

### Dependency Resolution

The registry uses Kahn's topological sort algorithm to resolve startup order. If a circular dependency is detected, `StartAll` returns an error:

```go
reg.SetDependencies("A", []string{"B"})
reg.SetDependencies("B", []string{"A"})

err := reg.StartAll(ctx)
// Error: dependency resolution failed: circular dependency detected
```

### Version Constraint Checking

Check whether a plugin version satisfies a semver constraint:

```go
ok, err := registry.CheckVersionConstraint("1.2.3", ">=1.0.0")
// ok = true

ok, err = registry.CheckVersionConstraint("2.0.0", "^1.2.3")
// ok = false (^1.2.3 means >=1.2.3, <2.0.0)

ok, err = registry.CheckVersionConstraint("1.3.0", "~1.2.3")
// ok = false (for patch, ~1.2.3 means >=1.2.3, <1.3.0)
```

Supported operators: `=`, `>=`, `<=`, `>`, `<`, `^` (compatible), `~` (approximate), `*` (any).

---

## 3. Dynamic Loading

### Shared Object Loader

Load Go plugins compiled with `-buildmode=plugin` (Linux/macOS):

```go
cfg := &loader.Config{
    PluginDir:       "./plugins",
    AllowedPatterns: []string{"*.so"},
    AutoRegister:    true,
}

soLoader := loader.NewSharedObjectLoader(cfg)

// Load a single plugin
p, err := soLoader.Load("./plugins/myplugin.so")
if err != nil {
    log.Fatal(err)
}
fmt.Println(p.Name(), p.Version())

// Load all plugins from a directory
plugins, err := soLoader.LoadDir("./plugins")
for _, p := range plugins {
    fmt.Println(p.Name())
}
```

The `.so` file must export a package-level variable named `Plugin` that implements `plugin.Plugin`:

```go
// myplugin.go - build with: go build -buildmode=plugin -o myplugin.so
package main

var Plugin = &MyPlugin{}
```

### Process Loader

Load plugins as separate OS processes. The executable must support `--metadata`, `--init`, `--run`, and `--health` flags:

```go
cfg := &loader.Config{
    PluginDir:       "./plugins",
    AllowedPatterns: []string{"*"},
}

procLoader := loader.NewProcessLoader(cfg)

// Load a plugin executable
p, err := procLoader.Load("./plugins/my-plugin-binary")
if err != nil {
    log.Fatal(err)
}

// The returned plugin implements the Plugin interface
// and communicates with the child process via JSON
p.Init(ctx, plugin.Config{"key": "value"})
p.Start(ctx)
defer p.Stop(ctx)
```

**Process plugin protocol:**

| Flag | Behavior |
|------|----------|
| `--metadata` | Print JSON `Metadata` to stdout and exit |
| `--init` | Read JSON config from stdin, initialize, exit |
| `--run` | Start the plugin process (long-running) |
| `--health` | Print `ok` to stdout if healthy, exit |

### Reading Process Metadata

For custom process-based loaders, use `ReadProcessMetadata`:

```go
reader := bufio.NewReader(stdout)
meta, err := loader.ReadProcessMetadata(reader)
if err != nil {
    log.Fatal(err)
}
fmt.Println(meta.Name, meta.Version)
```

---

## 4. Sandboxing

### Process Sandbox

Execute plugin actions in a child process with resource limits:

```go
cfg := &sandbox.Config{
    Limits: sandbox.ResourceLimits{
        MaxMemory: 256 * 1024 * 1024, // 256 MB
        MaxCPU:    50,                 // 50% of one core
        MaxDisk:   100 * 1024 * 1024,  // 100 MB
        Timeout:   30 * time.Second,
    },
    AllowNetwork: false,
    WorkDir:      "/tmp/sandbox",
}

sb := sandbox.NewProcessSandbox(cfg)

result, err := sb.Execute(ctx, myPlugin, sandbox.Action{
    Name:  "health",
    Input: nil,
})

fmt.Printf("ID: %s, Duration: %v, Error: %s\n",
    result.ID, result.Duration, result.Error)
```

### In-Process Sandbox

For trusted plugins, use `InProcessSandbox` which provides timeout enforcement without OS-level isolation:

```go
sb := sandbox.NewInProcessSandbox(nil) // nil uses defaults

result, err := sb.Execute(ctx, myPlugin, sandbox.Action{
    Name:  "start",
    Input: map[string]any{"mode": "debug"},
})
```

### Supported Actions

Both sandbox implementations support these built-in action names:

| Action | Plugin Method Called |
|--------|-------------------|
| `"health"` | `HealthCheck(ctx)` |
| `"init"` | `Init(ctx, config)` |
| `"start"` | `Start(ctx)` |
| `"stop"` | `Stop(ctx)` |

### Running External Commands

The `RunCommand` utility executes arbitrary commands with sandbox timeout and working directory:

```go
output, err := sandbox.RunCommand(ctx, cfg, "ls", "-la", "/tmp")
if err != nil {
    log.Fatal(err)
}
fmt.Println(output)
```

---

## 5. Structured Output Parsing

### JSON Parsing

```go
parser := structured.NewJSONParser()

data, err := parser.Parse(`{"name": "test", "value": 42}`, nil)
// data = map[string]any{"name": "test", "value": 42}

// Automatically extracts from code blocks
data, err = parser.Parse("```json\n{\"key\": \"value\"}\n```", nil)
```

### YAML Parsing

```go
parser := structured.NewYAMLParser()

data, err := parser.Parse("name: test\nvalue: 42", nil)
```

### Markdown Parsing

Extracts key-value pairs from markdown list items:

```go
parser := structured.NewMarkdownParser()

input := `
- **Name**: John Doe
- **Age**: 30
- Status: Active
`

data, err := parser.Parse(input, nil)
// data = map[string]any{"Name": "John Doe", "Age": "30", "Status": "Active"}
```

### Schema Definition

Define expected output structure manually:

```go
minLen := 1
schema := &structured.Schema{
    Type: "object",
    Properties: map[string]*structured.Schema{
        "name":  {Type: "string", MinLength: &minLen},
        "age":   {Type: "integer", Minimum: ptrFloat(0), Maximum: ptrFloat(150)},
        "tags":  {Type: "array", Items: &structured.Schema{Type: "string"}},
        "email": {Type: "string", Pattern: `^[^@]+@[^@]+$`},
    },
    Required: []string{"name", "age"},
}
```

### Schema Generation from Go Types

Automatically generate a schema from a Go struct using reflection:

```go
type User struct {
    Name  string   `json:"name"`
    Age   int      `json:"age"`
    Email string   `json:"email,omitempty"`
    Tags  []string `json:"tags,omitempty"`
}

schema, err := structured.SchemaFromType(User{})
// Generates schema with type="object", properties for each field,
// required=["name", "age"] (fields without omitempty)
```

### Validation

Validate parsed output against a schema:

```go
validator := structured.NewValidator(true) // strict mode

result, err := validator.Validate(
    `{"name": "Alice", "age": 30}`,
    schema,
)

if result.Valid {
    fmt.Println("Output is valid:", result.Data)
} else {
    for _, e := range result.Errors {
        fmt.Printf("  %s: %s\n", e.Path, e.Message)
    }
}
```

Validation checks include:
- Type matching (string, integer, number, boolean, array, object)
- Required properties
- String length (MinLength, MaxLength)
- Numeric range (Minimum, Maximum)
- Pattern matching (regex)
- Enum value checking
- Array length (MinItems, MaxItems)
- Recursive property and item validation

### JSON Repair

Attempt to fix common issues in malformed JSON:

```go
validator := structured.NewValidator(false)

repaired, err := validator.Repair(
    `{name: "test", value: 42,}`,  // unquoted keys, trailing comma
    schema,
)
// repaired = `{"name": "test", "value": 42}`
```

The repair function handles:
- Trailing commas before `]` or `}`
- Unquoted object keys
- Code block extraction

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "digital.vasic.plugins/pkg/plugin"
    "digital.vasic.plugins/pkg/registry"
    "digital.vasic.plugins/pkg/sandbox"
    "digital.vasic.plugins/pkg/structured"
)

// SimplePlugin implements plugin.Plugin
type SimplePlugin struct {
    name    string
    version string
    state   *plugin.StateTracker
}

func NewSimplePlugin(name, version string) *SimplePlugin {
    return &SimplePlugin{
        name:    name,
        version: version,
        state:   plugin.NewStateTracker(),
    }
}

func (p *SimplePlugin) Name() string    { return p.name }
func (p *SimplePlugin) Version() string { return p.version }

func (p *SimplePlugin) Init(_ context.Context, _ plugin.Config) error {
    return p.state.Transition(plugin.Uninitialized, plugin.Initialized)
}

func (p *SimplePlugin) Start(_ context.Context) error {
    return p.state.Transition(plugin.Initialized, plugin.Running)
}

func (p *SimplePlugin) Stop(_ context.Context) error {
    p.state.Set(plugin.Stopped)
    return nil
}

func (p *SimplePlugin) HealthCheck(_ context.Context) error {
    if p.state.Get() != plugin.Running {
        return fmt.Errorf("not running")
    }
    return nil
}

func main() {
    ctx := context.Background()

    // 1. Create and register plugins
    db := NewSimplePlugin("database", "1.0.0")
    cache := NewSimplePlugin("cache", "1.2.0")
    app := NewSimplePlugin("app", "2.0.0")

    reg := registry.New()
    reg.Register(db)
    reg.Register(cache)
    reg.Register(app)

    // 2. Set dependencies
    reg.SetDependencies("cache", []string{"database"})
    reg.SetDependencies("app", []string{"database", "cache"})

    // 3. Initialize all plugins
    for _, name := range reg.List() {
        p, _ := reg.Get(name)
        p.Init(ctx, plugin.Config{"env": "production"})
    }

    // 4. Start in dependency order
    if err := reg.StartAll(ctx); err != nil {
        log.Fatal(err)
    }
    defer reg.StopAll(ctx)

    // 5. Version checking
    ok, _ := registry.CheckVersionConstraint(cache.Version(), "^1.0.0")
    fmt.Printf("Cache version %s compatible with ^1.0.0: %v\n",
        cache.Version(), ok)

    // 6. Sandboxed health check
    sb := sandbox.NewInProcessSandbox(&sandbox.Config{
        Limits: sandbox.ResourceLimits{Timeout: 5 * time.Second},
    })

    result, _ := sb.Execute(ctx, app, sandbox.Action{Name: "health"})
    fmt.Printf("Health check: duration=%v, error=%q\n",
        result.Duration, result.Error)

    // 7. Structured output parsing
    parser := structured.NewJSONParser()
    data, _ := parser.Parse(`{"status": "ok", "plugins": 3}`, nil)
    fmt.Println("Parsed:", data)

    validator := structured.NewValidator(true)
    vResult, _ := validator.Validate(
        `{"status": "ok", "plugins": 3}`,
        &structured.Schema{
            Type: "object",
            Properties: map[string]*structured.Schema{
                "status":  {Type: "string"},
                "plugins": {Type: "integer"},
            },
            Required: []string{"status"},
        },
    )
    fmt.Println("Valid:", vResult.Valid)
}
```
