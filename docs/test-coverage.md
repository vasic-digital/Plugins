# Test Coverage Ledger — Plugins

> **Round 293 deep-doc audit** (template mirror of rounds 220 / 242-289).
> Symbol→test→Challenge mapping for every exported symbol in
> `pkg/plugin`, `pkg/registry`, `pkg/loader`, `pkg/structured`,
> `pkg/sandbox`, `pkg/i18n`.
> Anti-bluff invariant per CONST-035 / Article XI §11.9: every PASS row
> carries either a unit test that exercises the real implementation
> (NOT a mock) or a Challenge with captured runtime evidence.

## How to read this ledger

- **Symbol** — exported identifier as it appears in source.
- **Unit test** — test function that exercises the symbol directly.
- **Edge test** — additional table or property test covering boundary cases.
- **Challenge** — runtime exerciser script that captures positive
  evidence (process exits 0 when the feature works, exits non-zero when
  the planted-mutation runs).
- **Evidence kind** — what the Challenge captures (state transitions,
  parser outputs, sandbox durations, locale labels, etc.).

## `pkg/plugin` — core interface + state machine

| Symbol               | Unit test                                       | Edge test                                              | Challenge                                | Evidence kind             |
|----------------------|-------------------------------------------------|--------------------------------------------------------|------------------------------------------|---------------------------|
| `Plugin` (interface) | `plugin_test.TestPlugin_*`                      | `plugin_i18n_test.TestPlugin_TranslatorOverride`       | `runner` drives db+api lifecycle         | StartAll/StopAll markers  |
| `State`              | `plugin_test.TestState_String`                  | `plugin_test.TestState_UnknownNumeric`                 | `runner` enumerates 5 terminal states    | state-label stdout        |
| `Metadata`           | `plugin_test.TestMetadata_Validate`             | `plugin_i18n_test.TestMetadata_TranslatedError`        | `runner` validates real metadata         | error-message text        |
| `Config`             | `plugin_test.TestConfig_GetString/Int/Bool`     | `plugin_test.TestConfig_NilSafe`                       | `runner` Init with real Config           | Init evidence + call map  |
| `StateTracker`       | `plugin_test.TestStateTracker_Transition`       | `plugin_test.TestStateTracker_Concurrent`              | `runner` drives Uninit→Init→Run→Stop     | per-state transitions     |
| `SetTranslator`      | `plugin_i18n_test.TestSetTranslator_Wired`      | `plugin_i18n_test.TestSetTranslator_NilFallback`       | `runner` keeps NoopTranslator default    | NoopTranslator round-trip |

## `pkg/registry` — dependency-ordered registry

| Symbol                     | Unit test                                  | Edge test                                            | Challenge                                | Evidence kind            |
|----------------------------|--------------------------------------------|------------------------------------------------------|------------------------------------------|--------------------------|
| `Registry`                 | `registry_test.TestRegistry_*`             | `registry_test.TestRegistry_Concurrent`              | `runner` register+SetDependencies+List   | List stdout              |
| `New`                      | `registry_test.TestNew`                    | n/a                                                  | `runner` constructs real registry        | non-nil pointer          |
| `Register`                 | `registry_test.TestRegister_*`             | `registry_test.TestRegister_Nil/Empty/Duplicate`     | `runner` registers db + api              | registered names stdout  |
| `Get` / `List` / `Remove`  | `registry_test.TestGet/List/Remove`        | `registry_test.TestRemove_NotFound`                  | `runner` lists registered plugins        | List output verified     |
| `SetDependencies`          | `registry_test.TestSetDependencies`        | `registry_test.TestSetDependencies_Unknown`          | `runner` declares api→db dep             | dep declaration stdout   |
| `StartAll` / `StopAll`     | `registry_test.TestStartAll/StopAll`       | `registry_test.TestStart_CycleDetected`              | `runner` runs full lifecycle             | dep-ordered run + reverse|
| `CheckVersionConstraint`   | `registry_test.TestCheckVersionConstraint` | `registry_test.TestCheckVersionConstraint_Invalid`   | `runner` exercises 2.1.0 (api version)   | constraint check pass    |

## `pkg/loader` — dynamic plugin loading

| Symbol                  | Unit test                                       | Edge test                                                | Challenge                              | Evidence kind            |
|-------------------------|-------------------------------------------------|----------------------------------------------------------|----------------------------------------|--------------------------|
| `Loader` (interface)    | `loader_test.TestLoader_Interface`              | `loader_test.TestLoader_DefaultConfigFields`             | `runner` (in-tree register path)       | constructor return       |
| `Config`                | `loader_test.TestConfig_Defaults`               | `loader_test.TestConfig_Override`                        | n/a (default-config exercise)          | non-nil pointer          |
| `SharedObjectLoader`    | `loader_test.TestSharedObjectLoader_*`          | `loader_test.TestSharedObjectLoader_BadPath`             | (Linux-gated CGO runner)               | `.so` open success       |
| `ProcessLoader`         | `loader_test.TestProcessLoader_*`               | `loader_test.TestProcessLoader_Timeout`                  | (process exec gated by `_loader/`)     | exec.Cmd start success   |
| `processPlugin`         | `loader_test.TestProcessPlugin_Lifecycle`       | `loader_test.TestProcessPlugin_StopIdempotent`           | exercised transitively                 | child-pid lifecycle      |

## `pkg/structured` — JSON / YAML / Markdown parsing + validation

| Symbol                  | Unit test                                       | Edge test                                                | Challenge                                  | Evidence kind            |
|-------------------------|-------------------------------------------------|----------------------------------------------------------|--------------------------------------------|--------------------------|
| `OutputFormat`          | `structured_test.TestOutputFormat_String`       | `structured_test.TestOutputFormat_Unknown`               | `runner` JSON+YAML+MD all parsed           | format-tagged stdout     |
| `Schema`                | `structured_test.TestSchema_*`                  | `structured_test.TestSchemaFromType_NestedStruct`        | `runner` parser exercise without Schema    | schema-less parse OK     |
| `SchemaFromType`        | `structured_test.TestSchemaFromType_*`          | `structured_test.TestSchemaFromType_Nil`                 | (covered by unit test matrix)              | reflect schema returned  |
| `Parser` (interface)    | `structured_test.TestParser_Interface`          | n/a                                                      | `runner` invokes 3 real parsers            | parser dispatch stdout   |
| `JSONParser` / `New…`   | `structured_test.TestJSONParser_*`              | `structured_test.TestJSONParser_Malformed`               | `runner` parses real plugin status JSON    | decoded JSON map         |
| `YAMLParser` / `New…`   | `structured_test.TestYAMLParser_*`              | `structured_test.TestYAMLParser_Empty`                   | `runner` parses real plugin status YAML    | decoded YAML map         |
| `MarkdownParser` / `…`  | `structured_test.TestMarkdownParser_*`          | `structured_test.TestMarkdownParser_NoStructured`        | `runner` parses real md key-value list     | decoded markdown map     |
| `ValidationError`       | `structured_test.TestValidationError_String`    | n/a (struct)                                             | (exercised via Validate path)              | error fields populated   |
| `ValidationResult`      | `structured_test.TestValidationResult_OK`       | `structured_test.TestValidationResult_Aggregate`         | (exercised via Validate path)              | aggregate result struct  |
| `Validator` / `New…`    | `structured_test.TestValidator_*`               | `structured_test.TestValidator_NestedRequired`           | (covered by unit test matrix)              | per-field violations     |
| `Validate` / `…JSON`    | `structured_test.TestValidator_Validate*`       | `structured_test.TestValidator_StrictMode`               | (covered by unit test matrix)              | violation count          |
| `Repair`                | `structured_test.TestValidator_Repair`          | `structured_test.TestValidator_Repair_Idempotent`        | (covered by unit test matrix)              | repaired payload         |

## `pkg/sandbox` — process / in-process isolation

| Symbol                       | Unit test                                       | Edge test                                              | Challenge                              | Evidence kind            |
|------------------------------|-------------------------------------------------|--------------------------------------------------------|----------------------------------------|--------------------------|
| `ResourceLimits`             | `sandbox_test.TestResourceLimits_Defaults`      | `sandbox_test.TestResourceLimits_Override`             | `runner` uses real limits              | timeout enforcement      |
| `DefaultResourceLimits`      | `sandbox_test.TestResourceLimits_Defaults`      | n/a                                                    | `runner` uses default limits           | non-nil struct           |
| `Config` / `DefaultConfig`   | `sandbox_test.TestConfig_Defaults`              | `sandbox_test.TestConfig_NilSafe`                      | `runner` constructs sandbox Config     | sandbox Config wired     |
| `Action`                     | `sandbox_test.TestAction_Marshal`               | n/a (struct)                                           | `runner` issues "health" action        | action dispatch stdout   |
| `Result`                     | `sandbox_test.TestResult_OK`                    | `sandbox_test.TestResult_Error`                        | `runner` reads real Result fields      | id + duration values     |
| `Sandbox` (interface)        | `sandbox_test.TestSandbox_Interface`            | `sandbox_i18n_test.TestSandbox_Translated*`            | `runner` runs InProcessSandbox         | per-action result stdout |
| `ProcessSandbox` / `New…`    | `sandbox_test.TestProcessSandbox_*`             | `sandbox_test.TestProcessSandbox_TimeoutEnforced`      | (covered by ProcessSandbox unit suite) | timeout exceeded         |
| `InProcessSandbox` / `New…`  | `sandbox_test.TestInProcessSandbox_*`           | `sandbox_test.TestInProcessSandbox_NilPlugin`          | `runner` runs real health-check        | duration + id stdout     |
| `Execute`                    | `sandbox_test.TestExecute_*`                    | `sandbox_i18n_test.TestExecute_TimeoutLocalised`       | `runner` invokes Execute               | non-zero duration        |
| `RunCommand`                 | `sandbox_test.TestRunCommand_*`                 | `sandbox_test.TestRunCommand_Timeout`                  | (covered by sandbox unit suite)        | combined output captured |

## `pkg/i18n` — locale-aware translation seam

| Symbol            | Unit test                                  | Edge test                                  | Challenge                                | Evidence kind             |
|-------------------|--------------------------------------------|--------------------------------------------|------------------------------------------|---------------------------|
| `Translator`      | `translator_test.TestNoopTranslator_T`     | n/a (interface)                            | `runner` 5-locale label render           | locale-tagged stdout      |
| `NoopTranslator`  | `translator_test.TestNoopTranslator_T`     | `translator_test.TestNoopTranslator_Empty` | `runner` falls back to NoopTranslator    | verbatim key echo         |
| `T`               | `translator_test.TestNoopTranslator_T`     | `translator_test.TestT_NilParams`          | `runner` 5 fixture invocations           | translated string slice   |

## Round-293 Challenge harness

- **`challenges/runner/main.go`** — real Plugins exerciser. Constructs
  a real Registry, registers two real Plugin implementations with a
  declared dependency, runs StartAll / StopAll in topological order,
  drives the state machine through `Uninitialized → Initialized →
  Running → Stopped`, exercises the real `InProcessSandbox` over the
  running plugin (health action with non-zero captured duration),
  parses real JSON / YAML / Markdown payloads through the
  `structured.Parser` implementations, and renders 5-locale labels via
  a minimal in-process translator backed by
  `challenges/fixtures/<locale>.yaml`. Exits 0 on full end-to-end
  success; exits 99 on missing fixtures, parser failure, or
  state-machine mismatch.
- **`challenges/plugins_describe_challenge.sh`** — paired-mutation
  gate. Invokes the runner; verifies stdout matches the 5-locale
  ledger, the StartAll/StopAll lifecycle markers, the sandbox
  health-check evidence, the parser output line, and the terminal
  PASS marker. With `PLUGINS_DESCRIBE_MUTATE=1`, it plants a
  forbidden token expectation that the runner CANNOT satisfy and
  asserts the runner FAILS the gate (exit 99). This proves the gate
  is not a tautology — the mutation deliberately breaks the
  invariant and the harness catches the break.

## Anti-bluff posture

- No mock is imported from production code (`pkg/*` files not ending
  `_test.go`); mocks are confined to unit-test sources per CONST-050(A).
- No hardcoded user-facing strings in production code; user-facing
  error messages route through `translator.T(...)` per CONST-046, and
  the runner intentionally uses the project's own `i18n.Translator`
  interface to render the 5-locale evidence stream.
- The Challenge runner uses real package code paths (no `httptest`
  stand-in, no in-memory parser shim). The state-machine evidence is
  the real `pkg/plugin.StateTracker` advancing through real
  transitions; the sandbox evidence is the real
  `pkg/sandbox.InProcessSandbox.Execute` returning a real `Result`
  with measured `Duration`.
- Paired-mutation step proves the gate's positive-detection capability
  — see `challenges/plugins_describe_challenge.sh`.

## Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17 archival)

> "all existing tests and Challenges do work in anti-bluff manner -
> they MUST confirm that all tested codebase really works as expected!
> We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completition
> and full usability by end users of the product!"
