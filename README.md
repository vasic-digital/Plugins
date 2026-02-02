# Plugins

Generic, reusable Go module for plugin lifecycle management, dynamic loading, structured output parsing, and sandboxed execution.

**Module**: `digital.vasic.plugins`

## Packages

- **pkg/plugin** -- Core Plugin interface, Metadata, State, Config with typed accessors
- **pkg/registry** -- Thread-safe registry with dependency-ordered StartAll/StopAll and version constraint checking
- **pkg/loader** -- Dynamic plugin loading via Go shared objects (.so) or external processes
- **pkg/structured** -- Structured output parsing (JSON, YAML, Markdown) with schema validation and repair
- **pkg/sandbox** -- Plugin sandboxing with resource limits, timeout enforcement, and process isolation

## Usage

```go
import (
    "digital.vasic.plugins/pkg/plugin"
    "digital.vasic.plugins/pkg/registry"
)

reg := registry.New()
reg.Register(myPlugin)
reg.SetDependencies("myPlugin", []string{"dep1"})
reg.StartAll(ctx)
defer reg.StopAll(ctx)
```

## Testing

```bash
go test ./... -count=1 -race
```
