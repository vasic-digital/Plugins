# Changelog

All notable changes to the `digital.vasic.plugins` module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-03

### Added

#### Package `plugin`
- `Plugin` interface defining the core plugin contract: `Name`, `Version`, `Init`, `Start`, `Stop`, `HealthCheck`.
- `State` enum with five lifecycle states: `Uninitialized`, `Initialized`, `Running`, `Stopped`, `Failed`.
- `State.String()` method for human-readable state names.
- `Metadata` struct with JSON/YAML tags for plugin descriptive information (Name, Version, Description, Author, Dependencies).
- `Metadata.Validate()` method ensuring required fields are present.
- `Config` type (`map[string]any`) with typed accessors: `GetString`, `GetInt`, `GetFloat64`, `GetBool`, `GetStringSlice`, `Has`.
- `StateTracker` with thread-safe `Get`, `Set`, and `Transition` methods using `sync.RWMutex`.
- `NewStateTracker()` constructor initializing to `Uninitialized` state.

#### Package `registry`
- `Registry` struct with thread-safe plugin management.
- `New()` constructor for empty registry.
- `Register`, `Get`, `List`, `Remove` methods for plugin CRUD.
- `SetDependencies` for declaring plugin dependency relationships.
- `StartAll` with Kahn's topological sort for dependency-ordered startup.
- `StopAll` with reverse dependency order and collected error reporting.
- Circular dependency detection in `StartAll`/`StopAll`.
- `CheckVersionConstraint` function supporting 7 semver operators: `=`, `>=`, `<=`, `>`, `<`, `^`, `~`, plus wildcard `*`.

#### Package `loader`
- `Loader` interface with `Load` and `LoadDir` methods.
- `Config` struct with `PluginDir`, `AllowedPatterns`, and `AutoRegister` fields.
- `DefaultConfig()` returning sensible defaults.
- `SharedObjectLoader` for loading Go plugins compiled as `.so` shared objects.
- Path validation against allowed glob patterns.
- `ProcessLoader` for loading plugins as external OS processes.
- `processPlugin` adapter wrapping external executables as `plugin.Plugin`.
- Process plugin protocol: `--metadata`, `--init`, `--run`, `--health` flags.
- `ReadProcessMetadata` utility for reading JSON metadata from process stdout.
- `pluginHandle` interface and `openPlugin` variable for testability.

#### Package `sandbox`
- `Sandbox` interface with `Execute` method.
- `ResourceLimits` struct: `MaxMemory`, `MaxCPU`, `MaxDisk`, `Timeout`.
- `DefaultResourceLimits()` returning conservative defaults (256 MB, 50% CPU, 100 MB disk, 30s).
- `Config` struct with `Limits`, `AllowedSyscalls`, `AllowNetwork`, `WorkDir`.
- `DefaultConfig()` constructor.
- `Action` struct with `Name` and `Input` fields.
- `Result` struct with `ID` (UUID), `Output`, `Duration`, `Error`.
- `ProcessSandbox` for child-process isolation with timeout enforcement.
- `InProcessSandbox` for same-process execution with timeout (testing/trusted plugins).
- `RunCommand` utility for executing external commands with sandbox timeout and working directory.

#### Package `structured`
- `OutputFormat` constants: `FormatJSON`, `FormatYAML`, `FormatMarkdown`.
- `Parser` interface with `Parse` method.
- `JSONParser` with automatic code block extraction.
- `YAMLParser` with automatic code block extraction.
- `MarkdownParser` extracting key-value pairs from list items (`- **key**: value`).
- `Schema` struct following JSON Schema semantics: type, properties, required, items, enum, pattern, length/range constraints, description.
- `SchemaFromType` generating schemas from Go structs via reflection and JSON struct tags.
- Recursive type detection in schema generation.
- `Validator` with `Validate`, `ValidateJSON`, and `Repair` methods.
- `ValidationError` with JSONPath-style path reporting.
- `ValidationResult` with valid flag, error list, and parsed data.
- JSON repair: trailing comma removal, unquoted key fixing, code block extraction.
