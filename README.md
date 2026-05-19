# Plugins

Generic, reusable Go module for plugin lifecycle management, dynamic loading, structured output parsing, and sandboxed execution. Provides a clean Plugin interface with dependency-ordered startup/shutdown, version constraint checking, Go shared object and process-based loaders, JSON/YAML/Markdown parsers with schema validation, and resource-limited sandbox isolation.

**Module**: `digital.vasic.plugins` (Go 1.24+)

## Architecture

The module provides a complete plugin system from interface definition through loading, registry management, output parsing, and isolated execution. The plugin package defines the core contract. The registry manages lifecycle with topological dependency ordering. Two loader strategies support Go shared objects and external processes. The structured package parses and validates LLM-generated output. The sandbox package provides process isolation with resource limits.

```
pkg/
  plugin/       Core Plugin interface, State machine, Config, Metadata
  registry/     Thread-safe registry with dependency ordering (Kahn's algorithm)
  loader/       Dynamic loading: SharedObject (.so) and Process-based
  structured/   Output parsing (JSON, YAML, Markdown) with schema validation
  sandbox/      Plugin sandboxing with resource limits and timeout enforcement
```

## Package Reference

### pkg/plugin -- Core Types

Defines the Plugin interface and foundational types for the entire plugin system.

**Types:**
- `State` -- Lifecycle states: Uninitialized, Initialized, Running, Stopped, Failed. Implements `String()`.
- `Plugin` -- Interface contract:
  - `Name() string` -- Unique plugin name.
  - `Version() string` -- Semantic version.
  - `Init(ctx context.Context, config Config) error` -- Initialize with configuration.
  - `Start(ctx context.Context) error` -- Start the plugin.
  - `Stop(ctx context.Context) error` -- Graceful shutdown.
  - `HealthCheck(ctx context.Context) error` -- Returns nil if healthy.
- `Metadata` -- Name, Version, Description, Author, Dependencies. Has `Validate() error`.
- `Config` -- Map-based configuration (`map[string]any`) with typed accessors:
  - `GetString(key, fallback) string`
  - `GetInt(key, fallback) int`
  - `GetFloat64(key, fallback) float64`
  - `GetBool(key, fallback) bool`
  - `GetStringSlice(key) []string`
  - `Has(key) bool`
- `StateTracker` -- Thread-safe state machine with `Get()`, `Set(state)`, and `Transition(expected, next) error`.

### pkg/registry -- Plugin Registry

Thread-safe registry managing plugin lifecycle with Kahn's topological sort for dependency ordering and semver constraint checking.

**Types:**
- `Registry` -- Central plugin manager.

**Key Functions:**
- `New() *Registry` -- Creates an empty registry.
- `Registry.Register(p plugin.Plugin) error` -- Adds a plugin (rejects duplicates).
- `Registry.Get(name) (plugin.Plugin, bool)` -- Retrieves a plugin by name.
- `Registry.List() []string` -- Returns all registered plugin names.
- `Registry.Remove(name) error` -- Unregisters a plugin.
- `Registry.SetDependencies(pluginName, dependencies []string) error` -- Declares dependencies.
- `Registry.StartAll(ctx) error` -- Starts all plugins in dependency order (dependencies first).
- `Registry.StopAll(ctx) error` -- Stops all plugins in reverse dependency order (dependents first).
- `CheckVersionConstraint(version, constraint string) (bool, error)` -- Checks semver constraint. Supported operators: `=`, `>=`, `<=`, `>`, `<`, `^` (compatible), `~` (approximate).

**Dependency Resolution:** Uses Kahn's topological sort algorithm. Returns an error if a circular dependency is detected.

### pkg/loader -- Dynamic Plugin Loading

Two loading strategies for plugins: Go shared objects (.so files) and external OS processes.

**Types:**
- `Loader` -- Interface with `Load(path) (plugin.Plugin, error)` and `LoadDir(dir) ([]plugin.Plugin, error)`.
- `Config` -- PluginDir (default `./plugins`), AllowedPatterns (default `*.so`), AutoRegister flag.
- `SharedObjectLoader` -- Loads Go plugins compiled with `-buildmode=plugin`.
- `ProcessLoader` -- Runs plugins as external processes communicating via stdin/stdout.

**Key Functions:**
- `NewSharedObjectLoader(cfg *Config) *SharedObjectLoader` -- Creates a .so loader.
- `SharedObjectLoader.Load(path) (plugin.Plugin, error)` -- Opens a .so, looks up the `Plugin` symbol.
- `SharedObjectLoader.LoadDir(dir) ([]plugin.Plugin, error)` -- Loads all matching files in a directory.
- `NewProcessLoader(cfg *Config) *ProcessLoader` -- Creates a process-based loader.
- `ProcessLoader.Load(path) (plugin.Plugin, error)` -- Starts executable with `--metadata`, wraps as Plugin.
- `ReadProcessMetadata(r *bufio.Reader) (*plugin.Metadata, error)` -- Reads JSON metadata from process stdout.

**Process Plugin Protocol:**
- `--metadata` -- Print JSON `Metadata` to stdout.
- `--init` -- Read JSON config from stdin, initialize.
- `--run` -- Start the plugin process.
- `--health` -- Print "ok" if healthy.

### pkg/structured -- Structured Output Parsing

Parses and validates LLM-generated structured output in JSON, YAML, and Markdown formats with schema validation and auto-repair.

**Types:**
- `OutputFormat` -- FormatJSON, FormatYAML, FormatMarkdown.
- `Schema` -- JSON Schema-like structure with Type, Properties, Required, Items, Enum, Pattern, Min/MaxLength, Min/Maximum, Min/MaxItems, Description.
- `Parser` -- Interface with `Parse(output string, schema *Schema) (any, error)`.
- `JSONParser` -- Parses JSON (strips code blocks).
- `YAMLParser` -- Parses YAML (strips code blocks).
- `MarkdownParser` -- Extracts key-value pairs from markdown list items (`- **key**: value`).
- `ValidationError` -- Path, Message, Value.
- `ValidationResult` -- Valid flag, Errors, and parsed Data.
- `Validator` -- Validates parsed output against a Schema with optional strict mode.

**Key Functions:**
- `SchemaFromType(t any) (*Schema, error)` -- Generates a Schema from a Go struct type via reflection.
- `NewJSONParser() *JSONParser` / `NewYAMLParser() *YAMLParser` / `NewMarkdownParser() *MarkdownParser`
- `NewValidator(strictMode bool) *Validator` -- Creates a schema validator.
- `Validator.Validate(output, schema) (*ValidationResult, error)` -- Validates raw string as JSON.
- `Validator.ValidateJSON(output, schema) (*ValidationResult, error)` -- Validates JSON with full schema checks.
- `Validator.Repair(output, schema) (string, error)` -- Fixes trailing commas and unquoted keys.

### pkg/sandbox -- Plugin Sandboxing

Isolates plugin execution with resource limits, timeouts, and optional process-level separation.

**Types:**
- `ResourceLimits` -- MaxMemory (256MB), MaxCPU (50%), MaxDisk (100MB), Timeout (30s).
- `Config` -- Limits, AllowedSyscalls, AllowNetwork, WorkDir.
- `Action` -- Named action with input payload.
- `Result` -- ID, Output, Duration, Error string.
- `Sandbox` -- Interface with `Execute(ctx, plugin, action) (*Result, error)`.
- `ProcessSandbox` -- Runs actions in child processes with resource constraints.
- `InProcessSandbox` -- Runs actions in-process with timeout enforcement (for trusted plugins).

**Key Functions:**
- `NewProcessSandbox(cfg *Config) *ProcessSandbox` -- Creates an OS-process sandbox.
- `NewInProcessSandbox(cfg *Config) *InProcessSandbox` -- Creates an in-process sandbox.
- `ProcessSandbox.Execute(ctx, plugin, action) (*Result, error)` -- Executes with timeout and limits.
- `InProcessSandbox.Execute(ctx, plugin, action) (*Result, error)` -- Executes with timeout only.
- `RunCommand(ctx, cfg, name string, args ...string) (string, error)` -- Runs an external command within sandbox constraints.

**Supported Actions:** "health", "init", "start", "stop".

## Usage Examples

### Plugin Registry with Dependencies

```go
reg := registry.New()
reg.Register(databasePlugin) // implements plugin.Plugin
reg.Register(cachePlugin)
reg.Register(appPlugin)

reg.SetDependencies("app", []string{"database", "cache"})
reg.SetDependencies("cache", []string{"database"})

// Starts in order: database -> cache -> app
err := reg.StartAll(ctx)
defer reg.StopAll(ctx)
```

### Version Constraint Checking

```go
ok, _ := registry.CheckVersionConstraint("1.5.2", "^1.2.0") // true (>=1.2.0, <2.0.0)
ok, _ = registry.CheckVersionConstraint("2.0.0", "~1.2.0")  // false (>=1.2.0, <1.3.0)
ok, _ = registry.CheckVersionConstraint("1.2.3", ">=1.0.0")  // true
```

### Dynamic Loading from Shared Objects

```go
loader := loader.NewSharedObjectLoader(&loader.Config{
    PluginDir:       "/opt/plugins",
    AllowedPatterns: []string{"*.so"},
})
plugins, err := loader.LoadDir("")
for _, p := range plugins {
    reg.Register(p)
}
```

### Structured Output Validation

```go
schema, _ := structured.SchemaFromType(MyStruct{})
parser := structured.NewJSONParser()

data, err := parser.Parse(llmOutput, schema)

validator := structured.NewValidator(true)
result, _ := validator.Validate(llmOutput, schema)
if !result.Valid {
    repaired, _ := validator.Repair(llmOutput, schema)
}
```

### Sandboxed Execution

```go
sandbox := sandbox.NewProcessSandbox(&sandbox.Config{
    Limits: sandbox.ResourceLimits{
        MaxMemory: 128 * 1024 * 1024, // 128 MB
        Timeout:   10 * time.Second,
    },
    AllowNetwork: false,
})

result, err := sandbox.Execute(ctx, myPlugin, sandbox.Action{
    Name: "health",
})
fmt.Printf("Duration: %v, Error: %s\n", result.Duration, result.Error)
```

### Plugin Config with Typed Accessors

```go
cfg := plugin.Config{
    "host":     "localhost",
    "port":     5432,
    "debug":    true,
    "timeout":  30.5,
    "features": []string{"auth", "logging"},
}

host := cfg.GetString("host", "127.0.0.1")     // "localhost"
port := cfg.GetInt("port", 3000)                 // 5432
debug := cfg.GetBool("debug", false)             // true
features := cfg.GetStringSlice("features")       // ["auth", "logging"]
```

## Configuration

All packages provide Config structs with `DefaultConfig()` constructors. Pass `nil` to constructors to use production-ready defaults.

## Testing

```bash
go test ./... -count=1 -race       # All tests with race detection
go test ./... -short                # Unit tests only
go test -tags=integration ./...    # Integration tests
go test -bench=. ./tests/benchmark/ # Benchmarks
```

## Integration with HelixAgent

The Plugins module powers HelixAgent's extensible plugin system:
- Plugin interface defines the contract for all 10+ HelixAgent plugins (mcp, lsp, acp, embeddings, vision, etc.)
- Registry manages plugin lifecycle with dependency ordering during server boot
- Process loader enables external plugin binaries
- Structured output parsing validates LLM-generated tool call responses
- Sandbox isolation protects against misbehaving plugins with resource limits

The internal adapter at `internal/adapters/plugins/` bridges these generic types to HelixAgent-specific interfaces.

## Round-293 deep-doc + Challenge enrichment (anti-bluff)

This submodule ships a deep-documentation pack and a paired-mutation
Challenge gate per the round-293 sweep (template mirror of rounds 220
/ 242-289). The goal is to give every exported symbol either a unit
test that exercises the real implementation OR a runtime Challenge
that captures positive evidence — never an "absence-of-error PASS".

- **Test coverage ledger:** [`docs/test-coverage.md`](docs/test-coverage.md)
  — symbol→test→Challenge mapping for `pkg/plugin`, `pkg/registry`,
  `pkg/loader`, `pkg/structured`, `pkg/sandbox`, `pkg/i18n`.
- **Real exerciser:** [`challenges/runner/main.go`](challenges/runner/main.go)
  — constructs a real `Registry`, registers two real `Plugin`
  implementations with a declared dependency (`api` depends on `db`),
  drives the full lifecycle (`Init → Start → HealthCheck → Stop`),
  exercises the real `InProcessSandbox` over the running plugin, parses
  real JSON + YAML + Markdown payloads through the `structured.Parser`
  trio, and renders 5-locale labels via fixtures under
  `challenges/fixtures/<locale>.yaml`.
- **Paired-mutation gate:** [`challenges/plugins_describe_challenge.sh`](challenges/plugins_describe_challenge.sh)
  — normal run asserts the 5-locale ledger, lifecycle markers,
  sandbox evidence, parser output line, and terminal PASS. With
  `PLUGINS_DESCRIBE_MUTATE=1`, plants a token the runner CANNOT emit
  and asserts exit 99 — proving the gate has positive detection
  capability (not a tautology).

```bash
# Run the real Plugins exerciser.
go run ./challenges/runner -fixtures ./challenges/fixtures

# Paired-mutation gate (normal run — exit 0).
bash ./challenges/plugins_describe_challenge.sh

# Paired-mutation gate (mutation leg — exit 99 expected).
PLUGINS_DESCRIBE_MUTATE=1 bash ./challenges/plugins_describe_challenge.sh
```

### Anti-bluff guarantees

- **No mock from production code.** `pkg/*` files not ending `_test.go`
  do NOT import `mocks/` packages. The runner uses real package code
  paths only — real `Registry`, real `Plugin`, real `InProcessSandbox`,
  real `JSONParser` / `YAMLParser` / `MarkdownParser`. Per CONST-050(A).
- **No hardcoded user-facing strings.** Every user-facing error
  message in `pkg/plugin` and `pkg/sandbox` routes through
  `translator.T(key, params)`; the production default is
  `i18n.NoopTranslator` (returns the key verbatim), and consuming
  projects wire their own catalogue. Per CONST-046.
- **Captured runtime evidence.** Every PASS line in the runner stdout
  carries observable evidence — state transitions (`db=running
  api=running`), sandbox duration (`duration=3.2µs`), parser outputs
  (`json={"plugin":"api",...}`), and the 5-locale label render. Per
  CONST-035 / Article XI §11.9.
- **Submodule-decoupled.** This module is project-not-aware
  (CONST-051(B)); the runner intentionally avoids any HelixCode-specific
  import and could be lifted into a different consuming project
  unchanged.

### Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17 archival)

> "all existing tests and Challenges do work in anti-bluff manner -
> they MUST confirm that all tested codebase really works as expected!
> We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completition
> and full usability by end users of the product!"

## License

Proprietary.
