# Contributing

Thank you for your interest in contributing to the Plugins module. This guide covers the development workflow, coding standards, and submission process.

## Prerequisites

- Go 1.24 or later
- Git with SSH access configured
- `gofmt` and `go vet` (included with Go)
- `golangci-lint` (optional but recommended)

## Getting Started

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd Plugins
   ```

2. Verify the build:
   ```bash
   go build ./...
   ```

3. Run tests:
   ```bash
   go test ./... -count=1 -race
   ```

## Development Workflow

### Branch Naming

Create a branch from `main` using one of these prefixes:

| Prefix | Purpose |
|--------|---------|
| `feat/` | New feature |
| `fix/` | Bug fix |
| `refactor/` | Code restructuring without behavior change |
| `test/` | Adding or updating tests |
| `docs/` | Documentation changes |
| `chore/` | Build, CI, or tooling changes |

Example: `feat/toml-parser`, `fix/registry-race-condition`

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) with package scope:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

Examples:
```
feat(structured): add TOML parser implementation
fix(registry): prevent panic on nil plugin registration
test(sandbox): add concurrent execution stress test
docs(plugin): clarify StateTracker transition rules
refactor(loader): extract file validation to shared helper
```

Valid scopes: `plugin`, `registry`, `loader`, `sandbox`, `structured`, `plugins` (for cross-cutting changes).

### Code Quality Checks

Before submitting, run all quality checks:

```bash
# Format code
gofmt -w .

# Vet code
go vet ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run short tests (unit only)
go test ./... -short
```

If you have `golangci-lint` installed:
```bash
golangci-lint run ./...
```

## Coding Standards

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` formatting (enforced)
- Line length: 100 characters maximum for readability

### Import Grouping

Group imports with blank lines separating each group:

```go
import (
    // Standard library
    "context"
    "fmt"
    "sync"

    // Third-party
    "github.com/google/uuid"

    // Internal
    "digital.vasic.plugins/pkg/plugin"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Private fields/functions | camelCase | `validatePath`, `resolveOrder` |
| Exported types/functions | PascalCase | `StateTracker`, `NewValidator` |
| Constants | UPPER_SNAKE_CASE or PascalCase | `FormatJSON`, `Uninitialized` |
| Acronyms | All-caps | `HTTP`, `URL`, `ID`, `JSON`, `YAML` |
| Receivers | 1-2 letters | `r` for Registry, `s` for Sandbox, `p` for Plugin |

### Error Handling

- Always check errors
- Wrap errors with context: `fmt.Errorf("description: %w", err)`
- Use `defer` for cleanup
- Return errors, do not panic

### Interfaces

- Keep interfaces small and focused
- Accept interfaces, return concrete types
- Define interfaces in the package that uses them, not the package that implements them

### Tests

- Use table-driven tests with `testify`
- Naming: `Test<Struct>_<Method>_<Scenario>`
- Each package has its own `_test.go` file
- No cross-package test dependencies

Example:
```go
func TestRegistry_StartAll_CycleDetection(t *testing.T) {
    tests := []struct {
        name    string
        deps    map[string][]string
        wantErr bool
    }{
        {
            name:    "no cycle",
            deps:    map[string][]string{"A": {"B"}},
            wantErr: false,
        },
        {
            name:    "direct cycle",
            deps:    map[string][]string{"A": {"B"}, "B": {"A"}},
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Adding a New Feature

### Adding a New Parser

1. Create the parser struct in `pkg/structured/structured.go`:
   ```go
   type TOMLParser struct{}
   func NewTOMLParser() *TOMLParser { return &TOMLParser{} }
   func (p *TOMLParser) Parse(output string, _ *Schema) (any, error) {
       // implementation
   }
   ```

2. Add the output format constant:
   ```go
   FormatTOML OutputFormat = "toml"
   ```

3. Add tests in `pkg/structured/structured_test.go`

### Adding a New Loader Strategy

1. Create a struct implementing `Loader` in `pkg/loader/loader.go`
2. Implement `Load(path string) (plugin.Plugin, error)` and `LoadDir(dir string) ([]plugin.Plugin, error)`
3. Add a constructor: `func NewMyLoader(cfg *Config) *MyLoader`
4. Add tests in `pkg/loader/loader_test.go`

### Adding a New Sandbox Implementation

1. Create a struct implementing `Sandbox` in `pkg/sandbox/sandbox.go`
2. Implement `Execute(ctx context.Context, p plugin.Plugin, action Action) (*Result, error)`
3. Add a constructor: `func NewMySandbox(cfg *Config) *MySandbox`
4. Add tests in `pkg/sandbox/sandbox_test.go`

## Package Boundaries

When making changes, respect the dependency graph:

- `plugin` -- No internal dependencies. Changes here affect all other packages.
- `registry` -- Depends on `plugin` only.
- `loader` -- Depends on `plugin` only.
- `sandbox` -- Depends on `plugin` only.
- `structured` -- No internal dependencies.

Never introduce circular dependencies. Never import `registry`, `loader`, or `sandbox` from `plugin`.

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes following the standards above
3. Ensure all tests pass with `go test ./... -count=1 -race`
4. Write a clear PR description explaining the change and its motivation
5. Reference any related issues
6. Request review

## Reporting Issues

When reporting bugs, include:
- Go version (`go version`)
- Operating system and architecture
- Minimal reproduction steps
- Expected vs actual behavior
- Error messages and stack traces
