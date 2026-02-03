# Architecture

## Overview

The `digital.vasic.plugins` module is designed around five orthogonal packages, each addressing a distinct concern in plugin system design. The architecture enforces a strict, acyclic dependency graph with `plugin` as the foundational package and all other packages depending on it at most.

```
+------------------+     +------------------+     +------------------+
|    registry      |     |     loader       |     |     sandbox      |
|  (lifecycle      |     |  (dynamic        |     |  (isolation &    |
|   ordering)      |     |   loading)       |     |   resource       |
+--------+---------+     +--------+---------+     |   limits)        |
         |                        |                +--------+---------+
         |                        |                         |
         +------------+-----------+-------------------------+
                      |
             +--------+---------+
             |     plugin       |
             |  (core types &   |
             |   interfaces)    |
             +------------------+

             +------------------+
             |   structured     |
             |  (output parsing |
             |   & validation)  |
             +------------------+
```

## Design Decisions

### 1. Interface-First Design

Every major concern is defined as an interface before any implementation exists:

- `plugin.Plugin` -- The plugin contract
- `loader.Loader` -- The loading strategy contract
- `sandbox.Sandbox` -- The execution isolation contract
- `structured.Parser` -- The output parsing contract

This enables consumers to program against interfaces and swap implementations without changing application code.

### 2. Minimal Dependency Graph

The module enforces a strict rule: packages may only depend downward, never sideways or upward.

- `plugin` depends on nothing (stdlib only)
- `registry`, `loader`, `sandbox` depend only on `plugin`
- `structured` depends on nothing (stdlib + `gopkg.in/yaml.v3`)
- No package imports another peer package

This makes each package independently testable and prevents circular dependencies.

### 3. Thread Safety by Default

All stateful types use appropriate synchronization:

- `Registry` uses `sync.RWMutex` for concurrent read access with exclusive writes
- `StateTracker` uses `sync.RWMutex` for safe state queries and transitions
- `SharedObjectLoader` uses `sync.Mutex` because Go's `plugin.Open` is not reentrant
- `ProcessSandbox` uses `sync.Mutex` to serialize child process management

Stateless types (`Parser` implementations, `Validator`) are inherently safe for concurrent use.

### 4. Graceful Degradation

- `LoadDir` continues loading remaining plugins when individual loads fail
- `StopAll` collects errors from all plugins rather than stopping at the first failure
- `configFromPayload` returns empty config rather than failing on nil input
- `matchesPattern` returns true when no patterns are configured (open by default)

## Design Patterns

### Abstract Factory Pattern

The `Loader` interface acts as an abstract factory for `plugin.Plugin` instances. Two concrete factories exist:

- **SharedObjectLoader** -- Creates plugins from compiled `.so` shared objects
- **ProcessLoader** -- Creates plugins from external executables

Both produce `plugin.Plugin` values but use fundamentally different mechanisms. Consumers select a factory at configuration time and use the same `Load`/`LoadDir` interface thereafter.

```go
// Abstract factory
type Loader interface {
    Load(path string) (plugin.Plugin, error)
    LoadDir(dir string) ([]plugin.Plugin, error)
}

// Concrete factories
soLoader := loader.NewSharedObjectLoader(cfg)   // Factory A
procLoader := loader.NewProcessLoader(cfg)      // Factory B
```

### Registry Pattern

The `Registry` provides a centralized, thread-safe store for plugin instances indexed by name. It goes beyond a simple map by adding:

- **Uniqueness enforcement** -- Duplicate names are rejected
- **Dependency declaration** -- Plugins declare what they depend on
- **Topological ordering** -- StartAll/StopAll respect dependencies
- **Lifecycle management** -- Coordinated start and stop across all plugins

The topological sort uses Kahn's algorithm: compute in-degree for each node, seed a queue with zero-degree nodes, and process iteratively. Cycle detection occurs when the sorted output is smaller than the input set.

### Strategy Pattern

Three areas use the Strategy pattern with interchangeable implementations:

**Loading strategies:**
- `SharedObjectLoader` -- In-process loading via Go's plugin package
- `ProcessLoader` -- Out-of-process loading via child process management

**Parsing strategies:**
- `JSONParser` -- Parses JSON output
- `YAMLParser` -- Parses YAML output
- `MarkdownParser` -- Extracts key-value pairs from markdown lists

**Sandboxing strategies:**
- `ProcessSandbox` -- OS-level process isolation with resource limits
- `InProcessSandbox` -- Same-process execution with timeout enforcement

Each strategy set shares a common interface, allowing consumers to select at runtime.

### Template Method Pattern

The `processPlugin` adapter follows the Template Method pattern. It provides a fixed structure for plugin lifecycle operations (Init, Start, Stop, HealthCheck) while delegating the actual work to external processes:

- `Init` -- Marshals config to JSON, pipes to `--init` flag
- `Start` -- Launches the process with `--run` flag
- `Stop` -- Sends SIGINT, falls back to SIGKILL
- `HealthCheck` -- Runs `--health` and checks for "ok" output

The template is the process communication protocol; the method variation is what each external plugin does internally.

### Adapter Pattern

`processPlugin` is a classic adapter: it wraps an external executable (which knows nothing about Go interfaces) and presents it as a `plugin.Plugin`. The adaptation happens through:

- **Metadata discovery**: Running `--metadata` and parsing JSON output
- **Config serialization**: Marshaling `plugin.Config` to JSON for stdin
- **Health translation**: Converting stdout text ("ok") to error/nil
- **Lifecycle mapping**: Mapping interface methods to process signals

### State Machine Pattern

`StateTracker` implements a simple state machine with five states:

```
Uninitialized --(Init)--> Initialized --(Start)--> Running --(Stop)--> Stopped
                                                      |
                                                      +--(error)--> Failed
```

The `Transition` method enforces valid transitions by checking the current state before allowing a change. Invalid transitions return descriptive errors. The `Set` method bypasses validation for cases like error handling where forced state changes are needed.

### Builder Pattern

`plugin.Config` uses a builder-like approach with typed accessors. Rather than requiring callers to perform type assertions on `map[string]any` values, Config provides safe accessor methods with fallback defaults:

```go
cfg.GetString("key", "default")
cfg.GetInt("key", 0)
cfg.GetFloat64("key", 0.0)
cfg.GetBool("key", false)
cfg.GetStringSlice("key")
cfg.Has("key")
```

This pattern centralizes type coercion logic (handling `int` vs `int64` vs `float64` for numeric types) and eliminates scattered type assertions throughout consumer code.

## Schema Validation Architecture

The `structured` package implements a recursive schema validator that mirrors JSON Schema semantics:

1. **Schema definition** -- `Schema` struct with type, properties, constraints
2. **Schema generation** -- `SchemaFromType` uses reflection to auto-generate schemas from Go structs
3. **Recursive validation** -- `validateValue` dispatches by type, recursing into object properties and array items
4. **Path tracking** -- Errors include JSONPath-style paths (e.g., `$.users[0].name`)
5. **Repair** -- `Repair` applies regex-based fixes (trailing commas, unquoted keys) before re-validation

The validator supports two modes:
- **Strict mode** (`strictMode: true`) -- All validation rules enforced
- **Lenient mode** (`strictMode: false`) -- Future use for partial validation

## Error Handling Strategy

The module follows Go conventions for error handling:

- All errors are wrapped with `fmt.Errorf("context: %w", err)` for chain inspection
- `StopAll` collects multiple errors and joins them with semicolons
- Validation errors are structured (`ValidationError` with Path, Message, Value)
- The `Repair` function returns the original input when repair succeeds but re-marshaling fails

## Package Size and Complexity

| Package | Lines of Code | Exported Types | Exported Functions |
|---------|--------------|----------------|-------------------|
| `plugin` | ~240 | 5 (Plugin, State, Metadata, Config, StateTracker) | 2 (NewStateTracker, Metadata.Validate) |
| `registry` | ~310 | 1 (Registry) | 2 (New, CheckVersionConstraint) |
| `loader` | ~355 | 4 (Loader, Config, SharedObjectLoader, ProcessLoader) | 4 (DefaultConfig, NewSharedObjectLoader, NewProcessLoader, ReadProcessMetadata) |
| `sandbox` | ~275 | 7 (ResourceLimits, Config, Action, Result, Sandbox, ProcessSandbox, InProcessSandbox) | 5 (DefaultResourceLimits, DefaultConfig, NewProcessSandbox, NewInProcessSandbox, RunCommand) |
| `structured` | ~475 | 8 (OutputFormat, Schema, Parser, JSONParser, YAMLParser, MarkdownParser, ValidationError, ValidationResult, Validator) | 6 (SchemaFromType, NewJSONParser, NewYAMLParser, NewMarkdownParser, NewValidator) |
