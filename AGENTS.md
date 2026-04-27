# AGENTS.md - Plugins Module

## Module Overview

`digital.vasic.plugins` is a generic, reusable Go module for plugin lifecycle management, dynamic loading, structured output parsing, and sandboxed execution. It provides a clean Plugin interface with dependency-ordered startup/shutdown, version constraint checking, and resource-limited isolation.

**Module path**: `digital.vasic.plugins`
**Go version**: 1.24+
**Dependencies**: `github.com/google/uuid` (runtime), `github.com/stretchr/testify` (test), `gopkg.in/yaml.v3` (runtime)

## Package Responsibilities

| Package | Path | Responsibility |
|---------|------|----------------|
| `plugin` | `pkg/plugin/` | Core types: `Plugin` interface, `State` enum, `Metadata` struct, `Config` map with typed accessors, `StateTracker` for thread-safe lifecycle transitions. This is the foundational package with no internal dependencies. |
| `registry` | `pkg/registry/` | Thread-safe `Registry` with plugin registration, dependency declaration, topological-sort startup/shutdown (Kahn's algorithm), and semver constraint checking. Depends on `plugin`. |
| `loader` | `pkg/loader/` | Dynamic plugin loading via two strategies: `SharedObjectLoader` (Go `.so` plugins) and `ProcessLoader` (external executables communicating via JSON). Depends on `plugin`. |
| `sandbox` | `pkg/sandbox/` | Isolated execution with resource limits (memory, CPU, disk, timeout). Two implementations: `ProcessSandbox` (child process isolation) and `InProcessSandbox` (same-process with timeout). Depends on `plugin`. |
| `structured` | `pkg/structured/` | Structured output parsing (JSON, YAML, Markdown), schema generation from Go types via reflection, schema validation with detailed error paths, and JSON repair. No dependency on other internal packages. |

## Dependency Graph

```
registry   --->  plugin
loader     --->  plugin
sandbox    --->  plugin
structured      (standalone, no internal deps)
```

`plugin` is the leaf package. `registry`, `loader`, and `sandbox` depend only on `plugin`. `structured` is fully independent.

## Key Files

| File | Purpose |
|------|---------|
| `pkg/plugin/plugin.go` | Plugin interface, State enum, Metadata, Config, StateTracker |
| `pkg/registry/registry.go` | Registry struct, dependency ordering, version constraint checking |
| `pkg/loader/loader.go` | Loader interface, SharedObjectLoader, ProcessLoader, processPlugin adapter |
| `pkg/loader/plugin_open.go` | Go plugin.Open abstraction (pluginHandle interface for testability) |
| `pkg/sandbox/sandbox.go` | Sandbox interface, ProcessSandbox, InProcessSandbox, ResourceLimits, RunCommand |
| `pkg/structured/structured.go` | Parser interface, JSONParser, YAMLParser, MarkdownParser, Schema, Validator, Repair |
| `pkg/plugin/plugin_test.go` | Plugin package unit tests |
| `pkg/registry/registry_test.go` | Registry package unit tests |
| `pkg/loader/loader_test.go` | Loader package unit tests |
| `pkg/sandbox/sandbox_test.go` | Sandbox package unit tests |
| `pkg/structured/structured_test.go` | Structured package unit tests |
| `go.mod` | Module definition and dependencies |
| `CLAUDE.md` | AI coding assistant instructions |
| `README.md` | User-facing documentation with quick start |

## Agent Coordination Guide

### Division of Work

When multiple agents work on this module simultaneously, divide work by package boundary:

1. **Plugin Agent** -- Owns `pkg/plugin/`. Changes to core types (Plugin interface, State, Config, Metadata) affect `registry`, `loader`, and `sandbox`. Must coordinate with all other agents before modifying the `Plugin` interface or `Config` type.
2. **Registry Agent** -- Owns `pkg/registry/`. Manages plugin registration and lifecycle ordering. Changes to `Registry` methods or dependency resolution rarely affect other packages.
3. **Loader Agent** -- Owns `pkg/loader/`. Manages dynamic loading strategies. Can add new loader implementations independently as long as they return `plugin.Plugin`.
4. **Sandbox Agent** -- Owns `pkg/sandbox/`. Manages isolation strategies. Can add new sandbox implementations independently as long as they satisfy the `Sandbox` interface.
5. **Structured Agent** -- Owns `pkg/structured/`. Fully independent package. New parsers, validators, or schema features can be added without coordinating with other agents.

### Coordination Rules

- **Plugin interface changes** require all agents (registry, loader, sandbox) to update. The `Plugin` interface is the shared contract.
- **Config type changes** may affect loader (processPlugin uses Config) and sandbox (configFromPayload). Coordinate accordingly.
- **State enum changes** affect only code using StateTracker, typically plugin implementations.
- **structured package** is independent. Changes here never require updates to other packages.
- **Test isolation**: Each package has its own `_test.go` file. No cross-package test imports.
- **No circular dependencies**: The dependency graph is strictly acyclic. Never import `registry`, `loader`, or `sandbox` from `plugin`.

### Safe Parallel Changes

These changes can be made simultaneously without coordination:
- Adding a new parser implementation to `pkg/structured/`
- Adding a new sandbox implementation to `pkg/sandbox/`
- Adding new loader strategy to `pkg/loader/`
- Adding new version constraint operators to `pkg/registry/`
- Adding new typed accessors to `plugin.Config`
- Adding new tests to any package
- Updating documentation

### Changes Requiring Coordination

- Modifying the `Plugin` interface methods
- Changing `Config` type definition
- Modifying `State` enum values
- Changing `Metadata` struct fields
- Modifying `Loader` or `Sandbox` interface signatures

## Build and Test Commands

```bash
# Build all packages
go build ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run unit tests only (short mode)
go test ./... -short

# Run integration tests
go test -tags=integration ./...

# Run benchmarks
go test -bench=. ./tests/benchmark/

# Run a specific test
go test -v -run TestRegistry_StartAll ./pkg/registry/

# Format code
gofmt -w .

# Vet code
go vet ./...
```

## Commit Conventions

Follow Conventional Commits with package scope:

```
feat(plugin): add Priority field to Metadata
feat(registry): add version range constraint
feat(loader): add gRPC-based loader strategy
feat(sandbox): add cgroup resource enforcement
feat(structured): add TOML parser
fix(registry): prevent race condition in StartAll
test(sandbox): add timeout enforcement test
docs(plugins): update API reference
refactor(loader): extract path validation to helper
```

## Thread Safety Notes

- `Registry` is fully thread-safe. All public methods use `sync.RWMutex` for read/write protection.
- `StateTracker` uses `sync.RWMutex` for safe concurrent Get/Set/Transition operations.
- `SharedObjectLoader` uses `sync.Mutex` to serialize Load operations (Go plugin.Open is not reentrant).
- `ProcessLoader` uses `sync.Mutex` to protect the managed process map.
- `ProcessSandbox` uses `sync.Mutex` to serialize sandbox executions.
- `InProcessSandbox` is stateless and safe for concurrent use.
- `Config` (map type) is not inherently thread-safe; callers must not mutate it concurrently after passing to Init.
- All `Parser` implementations (JSON, YAML, Markdown) are stateless and safe for concurrent use.
- `Validator` is safe for concurrent use (strictMode is immutable after construction).

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->



<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.
