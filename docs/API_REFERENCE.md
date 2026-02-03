# API Reference

Complete reference for all exported types, functions, and methods in the `digital.vasic.plugins` module.

---

## Package `plugin`

Import: `digital.vasic.plugins/pkg/plugin`

Core types and interfaces for the plugin system.

### Types

#### `State`

```go
type State int
```

Represents the current lifecycle state of a plugin.

**Constants:**

| Constant | Value | Description |
|----------|-------|-------------|
| `Uninitialized` | 0 | Default state before Init is called |
| `Initialized` | 1 | Init completed successfully |
| `Running` | 2 | Start completed successfully |
| `Stopped` | 3 | Stop completed successfully |
| `Failed` | 4 | Plugin encountered a fatal error |

**Methods:**

- `func (s State) String() string` -- Returns a human-readable name for the state (`"uninitialized"`, `"initialized"`, `"running"`, `"stopped"`, `"failed"`, or `"unknown(N)"`).

---

#### `Plugin`

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

The interface that all plugins must implement.

| Method | Description |
|--------|-------------|
| `Name()` | Returns the unique name of the plugin |
| `Version()` | Returns the semantic version of the plugin |
| `Init(ctx, config)` | Initializes the plugin with configuration. Called once before Start. |
| `Start(ctx)` | Starts the plugin. Called after Init. |
| `Stop(ctx)` | Gracefully stops the plugin. |
| `HealthCheck(ctx)` | Returns nil if the plugin is healthy, an error otherwise. |

---

#### `Metadata`

```go
type Metadata struct {
    Name         string   `json:"name" yaml:"name"`
    Version      string   `json:"version" yaml:"version"`
    Description  string   `json:"description" yaml:"description"`
    Author       string   `json:"author" yaml:"author"`
    Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}
```

Descriptive information about a plugin. Supports JSON and YAML serialization.

**Methods:**

- `func (m *Metadata) Validate() error` -- Checks that `Name` and `Version` are non-empty. Returns an error describing the first missing field.

---

#### `Config`

```go
type Config map[string]any
```

Map-based configuration with typed accessor methods. All accessors return the fallback value when the key is missing or the value is not convertible to the requested type.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetString` | `(key, fallback string) string` | Returns string value or fallback |
| `GetInt` | `(key string, fallback int) int` | Returns int value or fallback. Handles `int`, `int64`, `float64` source types. |
| `GetFloat64` | `(key string, fallback float64) float64` | Returns float64 value or fallback. Handles `float64`, `float32`, `int`, `int64`. |
| `GetBool` | `(key string, fallback bool) bool` | Returns bool value or fallback |
| `GetStringSlice` | `(key string) []string` | Returns `[]string` or nil. Handles both `[]string` and `[]any` (filtering non-strings). |
| `Has` | `(key string) bool` | Reports whether the config contains the key |

All methods are nil-safe: calling on a nil Config returns the fallback or nil/false.

---

#### `StateTracker`

```go
type StateTracker struct {
    // unexported fields
}
```

Thread-safe tracker for plugin state transitions using `sync.RWMutex`.

**Constructor:**

- `func NewStateTracker() *StateTracker` -- Creates a new tracker starting in `Uninitialized` state.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Get` | `() State` | Returns the current state (read-locked) |
| `Set` | `(s State)` | Sets the state unconditionally (write-locked) |
| `Transition` | `(expected, next State) error` | Atomically transitions from `expected` to `next`. Returns error if current state does not match `expected`. |

---

## Package `registry`

Import: `digital.vasic.plugins/pkg/registry`

Thread-safe plugin registry with dependency ordering and version constraints.

### Types

#### `Registry`

```go
type Registry struct {
    // unexported fields
}
```

Manages registered plugins with thread-safe operations.

**Constructor:**

- `func New() *Registry` -- Creates a new empty Registry.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Register` | `(p plugin.Plugin) error` | Adds a plugin. Returns error if nil, empty name, or duplicate. |
| `Get` | `(name string) (plugin.Plugin, bool)` | Returns plugin by name and whether it was found. |
| `List` | `() []string` | Returns names of all registered plugins. |
| `Remove` | `(name string) error` | Unregisters a plugin by name. Returns error if not found. Also removes dependencies. |
| `SetDependencies` | `(pluginName string, dependencies []string) error` | Declares dependencies for a plugin. Returns error if plugin not found. |
| `StartAll` | `(ctx context.Context) error` | Starts all plugins in dependency order (Kahn's topological sort). Returns error on cycle detection or start failure. |
| `StopAll` | `(ctx context.Context) error` | Stops all plugins in reverse dependency order. Collects and joins all stop errors. |

### Functions

#### `CheckVersionConstraint`

```go
func CheckVersionConstraint(version, constraint string) (bool, error)
```

Checks whether a semver version string satisfies a constraint.

**Parameters:**
- `version` -- Semantic version string (e.g., `"1.2.3"`)
- `constraint` -- Version constraint with operator (e.g., `">=1.0.0"`, `"^1.2.3"`, `"~1.2.0"`)

**Supported operators:**

| Operator | Meaning | Example |
|----------|---------|---------|
| `=` | Exact match (default when no operator) | `=1.2.3` |
| `>=` | Greater than or equal | `>=1.0.0` |
| `<=` | Less than or equal | `<=2.0.0` |
| `>` | Strictly greater than | `>1.0.0` |
| `<` | Strictly less than | `<2.0.0` |
| `^` | Compatible (same major) | `^1.2.3` means `>=1.2.3, <2.0.0` |
| `~` | Approximate (same minor) | `~1.2.3` means `>=1.2.3, <1.3.0` |
| `*` | Any version | Always returns true |

**Returns:** `(bool, error)` -- Whether the constraint is satisfied, or an error for invalid version/constraint format.

---

## Package `loader`

Import: `digital.vasic.plugins/pkg/loader`

Dynamic plugin loading from shared objects and external processes.

### Interfaces

#### `Loader`

```go
type Loader interface {
    Load(path string) (plugin.Plugin, error)
    LoadDir(dir string) ([]plugin.Plugin, error)
}
```

| Method | Description |
|--------|-------------|
| `Load(path)` | Loads a single plugin from the given file path |
| `LoadDir(dir)` | Loads all plugins from the given directory |

### Types

#### `Config`

```go
type Config struct {
    PluginDir       string   `json:"plugin_dir" yaml:"plugin_dir"`
    AllowedPatterns []string `json:"allowed_patterns" yaml:"allowed_patterns"`
    AutoRegister    bool     `json:"auto_register" yaml:"auto_register"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `PluginDir` | `string` | Default directory to scan for plugins |
| `AllowedPatterns` | `[]string` | Glob patterns restricting which files may be loaded |
| `AutoRegister` | `bool` | Whether to auto-register loaded plugins |

**Constructor:**

- `func DefaultConfig() *Config` -- Returns `Config{PluginDir: "./plugins", AllowedPatterns: []string{"*.so"}, AutoRegister: true}`.

---

#### `SharedObjectLoader`

```go
type SharedObjectLoader struct {
    // unexported fields
}
```

Loads Go plugins compiled as shared objects (`.so` files, Linux/macOS only).

**Constructor:**

- `func NewSharedObjectLoader(cfg *Config) *SharedObjectLoader` -- Creates a loader for `.so` files. Uses `DefaultConfig()` if cfg is nil.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Load` | `(path string) (plugin.Plugin, error)` | Opens a `.so` file, looks up the `Plugin` symbol, and returns it. Validates path against allowed patterns. |
| `LoadDir` | `(dir string) ([]plugin.Plugin, error)` | Walks directory and loads all matching `.so` files. Skips files that fail to load. Uses `config.PluginDir` when dir is empty. |

---

#### `ProcessLoader`

```go
type ProcessLoader struct {
    // unexported fields
}
```

Runs plugins as separate OS processes communicating via stdin/stdout JSON.

**Constructor:**

- `func NewProcessLoader(cfg *Config) *ProcessLoader` -- Creates a process-based loader. Uses `DefaultConfig()` with `AllowedPatterns: ["*"]` if cfg is nil.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Load` | `(path string) (plugin.Plugin, error)` | Runs the executable with `--metadata`, parses JSON metadata, returns a `processPlugin` adapter. |
| `LoadDir` | `(dir string) ([]plugin.Plugin, error)` | Loads all executables in the directory. Skips entries that fail. |

### Functions

#### `ReadProcessMetadata`

```go
func ReadProcessMetadata(r *bufio.Reader) (*plugin.Metadata, error)
```

Reads plugin metadata from a process's stdout. Expects a single JSON line conforming to `plugin.Metadata`. Validates the metadata after parsing.

---

## Package `sandbox`

Import: `digital.vasic.plugins/pkg/sandbox`

Plugin isolation and resource-limited execution.

### Interfaces

#### `Sandbox`

```go
type Sandbox interface {
    Execute(ctx context.Context, p plugin.Plugin, action Action) (*Result, error)
}
```

Executes a plugin action in isolation.

### Types

#### `ResourceLimits`

```go
type ResourceLimits struct {
    MaxMemory int64         `json:"max_memory" yaml:"max_memory"`
    MaxCPU    int           `json:"max_cpu" yaml:"max_cpu"`
    MaxDisk   int64         `json:"max_disk" yaml:"max_disk"`
    Timeout   time.Duration `json:"timeout" yaml:"timeout"`
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MaxMemory` | `int64` | 256 MB | Maximum memory in bytes (0 = unlimited) |
| `MaxCPU` | `int` | 50 | CPU percentage (0 = unlimited, 100 = one core) |
| `MaxDisk` | `int64` | 100 MB | Maximum disk in bytes (0 = unlimited) |
| `Timeout` | `time.Duration` | 30s | Timeout for sandboxed action |

**Constructor:**

- `func DefaultResourceLimits() ResourceLimits` -- Returns conservative defaults (256 MB memory, 50% CPU, 100 MB disk, 30s timeout).

---

#### `Config`

```go
type Config struct {
    Limits          ResourceLimits `json:"limits" yaml:"limits"`
    AllowedSyscalls []string       `json:"allowed_syscalls,omitempty" yaml:"allowed_syscalls,omitempty"`
    AllowNetwork    bool           `json:"allow_network" yaml:"allow_network"`
    WorkDir         string         `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Limits` | `ResourceLimits` | (see above) | Resource constraints |
| `AllowedSyscalls` | `[]string` | nil (all allowed) | Restricts permitted syscalls |
| `AllowNetwork` | `bool` | false | Enables/disables network access |
| `WorkDir` | `string` | "" | Working directory for sandboxed process |

**Constructor:**

- `func DefaultConfig() *Config` -- Returns `Config{Limits: DefaultResourceLimits(), AllowNetwork: false}`.

---

#### `Action`

```go
type Action struct {
    Name  string `json:"name"`
    Input any    `json:"input,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Action name: `"health"`, `"init"`, `"start"`, `"stop"` |
| `Input` | `any` | Payload for the action (optional) |

---

#### `Result`

```go
type Result struct {
    ID       string        `json:"id"`
    Output   any           `json:"output,omitempty"`
    Duration time.Duration `json:"duration"`
    Error    string        `json:"error,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | UUID identifying this execution |
| `Output` | `any` | Returned data (optional) |
| `Duration` | `time.Duration` | Execution wall-clock time |
| `Error` | `string` | Error message, empty on success |

---

#### `ProcessSandbox`

```go
type ProcessSandbox struct {
    // unexported fields
}
```

Runs plugin actions as separate OS processes.

**Constructor:**

- `func NewProcessSandbox(cfg *Config) *ProcessSandbox` -- Creates a process-isolated sandbox. Uses `DefaultConfig()` if cfg is nil.

**Methods:**

- `func (s *ProcessSandbox) Execute(ctx context.Context, p plugin.Plugin, action Action) (*Result, error)` -- Dispatches the action to the plugin with timeout enforcement. Returns error only for nil plugin. Timeout and action errors are captured in `Result.Error`.

---

#### `InProcessSandbox`

```go
type InProcessSandbox struct {
    // unexported fields
}
```

Runs actions in the same process with timeout enforcement. Suitable for testing and trusted plugins.

**Constructor:**

- `func NewInProcessSandbox(cfg *Config) *InProcessSandbox` -- Creates an in-process sandbox. Uses `DefaultConfig()` if cfg is nil.

**Methods:**

- `func (s *InProcessSandbox) Execute(ctx context.Context, p plugin.Plugin, action Action) (*Result, error)` -- Executes the action synchronously with a context timeout. Returns error only for nil plugin.

### Functions

#### `RunCommand`

```go
func RunCommand(ctx context.Context, cfg *Config, name string, args ...string) (string, error)
```

Executes an external command with the sandbox's timeout and working directory. Returns trimmed combined output and any execution error. Uses `DefaultConfig()` if cfg is nil.

---

## Package `structured`

Import: `digital.vasic.plugins/pkg/structured`

Structured output parsing, schema validation, and repair.

### Constants

#### `OutputFormat`

```go
type OutputFormat string
```

| Constant | Value | Description |
|----------|-------|-------------|
| `FormatJSON` | `"json"` | JSON format |
| `FormatYAML` | `"yaml"` | YAML format |
| `FormatMarkdown` | `"markdown"` | Markdown format |

### Interfaces

#### `Parser`

```go
type Parser interface {
    Parse(output string, schema *Schema) (any, error)
}
```

Parses a raw string into structured data. The `schema` parameter is available for format-aware parsing but is not required by current implementations.

### Types

#### `Schema`

```go
type Schema struct {
    Type        string             `json:"type" yaml:"type"`
    Properties  map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
    Required    []string           `json:"required,omitempty" yaml:"required,omitempty"`
    Items       *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
    Enum        []any              `json:"enum,omitempty" yaml:"enum,omitempty"`
    Pattern     string             `json:"pattern,omitempty" yaml:"pattern,omitempty"`
    MinLength   *int               `json:"minLength,omitempty" yaml:"minLength,omitempty"`
    MaxLength   *int               `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
    Minimum     *float64           `json:"minimum,omitempty" yaml:"minimum,omitempty"`
    Maximum     *float64           `json:"maximum,omitempty" yaml:"maximum,omitempty"`
    MinItems    *int               `json:"minItems,omitempty" yaml:"minItems,omitempty"`
    MaxItems    *int               `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
    Description string             `json:"description,omitempty" yaml:"description,omitempty"`
}
```

Defines the expected structure of parsed output. Follows JSON Schema semantics.

| Field | Type | Description |
|-------|------|-------------|
| `Type` | `string` | Type name: `"string"`, `"integer"`, `"number"`, `"boolean"`, `"array"`, `"object"` |
| `Properties` | `map[string]*Schema` | Property schemas for object types |
| `Required` | `[]string` | Required property names for object types |
| `Items` | `*Schema` | Item schema for array types |
| `Enum` | `[]any` | Allowed values for string types |
| `Pattern` | `string` | Regex pattern for string types |
| `MinLength` / `MaxLength` | `*int` | String length constraints |
| `Minimum` / `Maximum` | `*float64` | Numeric range constraints |
| `MinItems` / `MaxItems` | `*int` | Array length constraints |
| `Description` | `string` | Human-readable description |

---

#### `JSONParser`

```go
type JSONParser struct{}
```

**Constructor:**

- `func NewJSONParser() *JSONParser`

**Methods:**

- `func (p *JSONParser) Parse(output string, _ *Schema) (any, error)` -- Parses JSON. Automatically extracts content from `` ```json `` code blocks.

---

#### `YAMLParser`

```go
type YAMLParser struct{}
```

**Constructor:**

- `func NewYAMLParser() *YAMLParser`

**Methods:**

- `func (p *YAMLParser) Parse(output string, _ *Schema) (any, error)` -- Parses YAML. Automatically extracts content from `` ```yaml `` code blocks.

---

#### `MarkdownParser`

```go
type MarkdownParser struct{}
```

**Constructor:**

- `func NewMarkdownParser() *MarkdownParser`

**Methods:**

- `func (p *MarkdownParser) Parse(output string, _ *Schema) (any, error)` -- Extracts key-value pairs from markdown list items. Matches patterns like `- **key**: value` and `- key: value`. Returns error if no structured data found.

---

#### `ValidationError`

```go
type ValidationError struct {
    Path    string `json:"path"`
    Message string `json:"message"`
    Value   string `json:"value,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Path` | `string` | JSONPath-style location (e.g., `$.users[0].name`) |
| `Message` | `string` | Description of the validation failure |
| `Value` | `string` | The offending value (truncated, optional) |

---

#### `ValidationResult`

```go
type ValidationResult struct {
    Valid  bool              `json:"valid"`
    Errors []ValidationError `json:"errors,omitempty"`
    Data   any               `json:"data,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Valid` | `bool` | Whether validation passed |
| `Errors` | `[]ValidationError` | List of validation failures |
| `Data` | `any` | The parsed data (set even on failure) |

---

#### `Validator`

```go
type Validator struct {
    // unexported fields
}
```

**Constructor:**

- `func NewValidator(strictMode bool) *Validator` -- Creates a validator. `strictMode` is reserved for future use.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Validate` | `(output string, schema *Schema) (*ValidationResult, error)` | Validates a raw string against a schema. Delegates to `ValidateJSON`. |
| `ValidateJSON` | `(output string, schema *Schema) (*ValidationResult, error)` | Parses JSON and validates against schema. Returns `ValidationResult` (never returns error for invalid input; uses `result.Valid` instead). |
| `Repair` | `(output string, schema *Schema) (string, error)` | Attempts to fix common JSON issues (trailing commas, unquoted keys, code block wrapping). Returns pretty-printed JSON on success. |

### Functions

#### `SchemaFromType`

```go
func SchemaFromType(t any) (*Schema, error)
```

Generates a `Schema` from a Go value using reflection. Examines struct fields via `json` struct tags for property names and `omitempty` for required/optional status. Reads `description` struct tags for field descriptions.

**Supported Go types:**

| Go Type | Schema Type |
|---------|-------------|
| `string` | `"string"` |
| `int`, `int8`...`int64`, `uint`...`uint64` | `"integer"` |
| `float32`, `float64` | `"number"` |
| `bool` | `"boolean"` |
| `[]T`, `[N]T` | `"array"` with Items |
| `map[K]V` | `"object"` |
| `struct` | `"object"` with Properties |
| `interface` | `"object"` |

Handles pointer types by dereferencing. Detects recursive types to prevent infinite loops.
