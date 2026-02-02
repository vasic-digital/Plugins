# CLAUDE.md - Plugins Module

## Overview

`digital.vasic.plugins` is a generic, reusable Go module for plugin lifecycle management, dynamic loading, structured output parsing, and sandboxed execution. It provides a clean plugin interface with dependency-ordered startup/shutdown, version constraint checking, and resource-limited isolation.

**Module**: `digital.vasic.plugins` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -tags=integration ./...   # Integration tests
go test -bench=. ./tests/benchmark/
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length <= 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/plugin` | Core types: Plugin interface, Metadata, State, Config |
| `pkg/registry` | Thread-safe plugin registry with dependency ordering |
| `pkg/loader` | Dynamic loading (SharedObject, Process) |
| `pkg/structured` | Structured output parsing (JSON, YAML, Markdown) and validation |
| `pkg/sandbox` | Plugin sandboxing with resource limits and timeout |

## Key Interfaces

- `plugin.Plugin` -- Plugin contract (Name, Version, Init, Start, Stop, HealthCheck)
- `loader.Loader` -- Dynamic loading (Load, LoadDir)
- `structured.Parser` -- Output parsing (Parse)
- `sandbox.Sandbox` -- Isolated execution (Execute)

## Design Patterns

- **Strategy**: Loader (SharedObject/Process), Parser (JSON/YAML/Markdown), Sandbox (Process/InProcess)
- **Registry**: Thread-safe plugin registry with Kahn's topological sort
- **State Machine**: StateTracker for plugin lifecycle transitions
- **Adapter**: processPlugin wraps external processes as Plugin interface
- **Builder**: Config with typed accessors

## Commit Style

Conventional Commits: `feat(registry): add version constraint checking`
