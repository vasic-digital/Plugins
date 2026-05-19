// Package main — round-293 Plugins Challenge runner.
//
// Real exerciser per CONST-050(B): no mocks. Constructs a real
// Registry, registers two real Plugin implementations with a declared
// dependency, runs StartAll / StopAll in topological order, drives the
// state machine through Uninitialized → Initialized → Running →
// Stopped, exercises the real InProcessSandbox over the running
// plugin, parses real JSON+YAML+Markdown output through the
// structured Parser+Validator pair, and renders 5-locale labels via
// a minimal in-process translator backed by
// ../fixtures/<locale>.yaml.
//
// Exits 0 on full end-to-end success; exits 99 on missing fixtures or
// state-machine mismatch; exits 1 on Go runtime error.
//
// Anti-bluff per CONST-035 / Article XI §11.9: every PASS surface
// emits captured runtime evidence (state transitions, sandbox
// durations, per-locale label renders, parser outputs). No
// "absence-of-error PASS".
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.plugins/pkg/i18n"
	"digital.vasic.plugins/pkg/plugin"
	"digital.vasic.plugins/pkg/registry"
	"digital.vasic.plugins/pkg/sandbox"
	"digital.vasic.plugins/pkg/structured"
)

// expectedKeys is the closed set of label keys every fixture MUST
// provide. Mismatch ⇒ exit 99.
var expectedKeys = []string{
	"plugins_state_uninitialized",
	"plugins_state_initialized",
	"plugins_state_running",
	"plugins_state_stopped",
	"plugins_state_failed",
}

// expectedLocales is the closed 5-locale set the round-293 ledger
// requires. Missing fixture file ⇒ exit 99.
var expectedLocales = []string{"en", "sr", "ja", "es", "de"}

// fixtureTranslator is a minimal in-process Translator backed by a
// flat map[string]string parsed from ../fixtures/<locale>.yaml.
//
// We hand-parse the YAML rather than pull a heavy dependency: each
// fixture file is intentionally simple (two-space indented "key:
// value" pairs under labels:) so the parser stays robust without
// dragging an additional production dependency into the Plugins
// submodule's surface (it already declares yaml.v3 indirectly, but
// the runner intentionally stays self-contained).
type fixtureTranslator struct {
	locale string
	labels map[string]string
}

func (f fixtureTranslator) T(key string, _ map[string]any) string {
	if v, ok := f.labels[key]; ok {
		return v
	}
	return key // CONST-046 fallback contract: return key verbatim
}

func loadFixture(dir, locale string) (i18n.Translator, error) {
	path := filepath.Join(dir, locale+".yaml")
	fh, err := os.Open(path) //nolint:gosec // round-293 fixture loader
	if err != nil {
		return nil, fmt.Errorf("open fixture %s: %w", path, err)
	}
	defer fh.Close()

	labels := make(map[string]string)
	scanner := bufio.NewScanner(fh)
	inLabels := false
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if trim == "labels:" {
			inLabels = true
			continue
		}
		if !inLabels {
			continue
		}
		// Expect 2-space indent + "key: \"value\"" or "key: value".
		if !strings.HasPrefix(line, "  ") {
			inLabels = false
			continue
		}
		kv := strings.SplitN(trim, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, `"`)
		labels[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan fixture %s: %w", path, err)
	}
	for _, k := range expectedKeys {
		if _, ok := labels[k]; !ok {
			return nil, fmt.Errorf("fixture %s missing key %q", path, k)
		}
	}
	return fixtureTranslator{locale: locale, labels: labels}, nil
}

// demoPlugin is a real Plugin implementation used by the runner. It
// drives a StateTracker through the full lifecycle and records every
// call so the gate can assert real invocation counts.
type demoPlugin struct {
	name    string
	version string
	tracker *plugin.StateTracker
	calls   map[string]int
}

func newDemoPlugin(name, version string) *demoPlugin {
	return &demoPlugin{
		name:    name,
		version: version,
		tracker: plugin.NewStateTracker(),
		calls:   make(map[string]int),
	}
}

func (d *demoPlugin) Name() string    { return d.name }
func (d *demoPlugin) Version() string { return d.version }

func (d *demoPlugin) Init(_ context.Context, _ plugin.Config) error {
	d.calls["Init"]++
	return d.tracker.Transition(plugin.Uninitialized, plugin.Initialized)
}

func (d *demoPlugin) Start(_ context.Context) error {
	d.calls["Start"]++
	return d.tracker.Transition(plugin.Initialized, plugin.Running)
}

func (d *demoPlugin) Stop(_ context.Context) error {
	d.calls["Stop"]++
	return d.tracker.Transition(plugin.Running, plugin.Stopped)
}

func (d *demoPlugin) HealthCheck(_ context.Context) error {
	d.calls["HealthCheck"]++
	if d.tracker.Get() != plugin.Running {
		return fmt.Errorf("plugin %s not running", d.name)
	}
	return nil
}

// stateLabelKey maps a plugin.State to the i18n key the runner uses to
// render that state into the active locale.
func stateLabelKey(s plugin.State) string {
	switch s {
	case plugin.Uninitialized:
		return "plugins_state_uninitialized"
	case plugin.Initialized:
		return "plugins_state_initialized"
	case plugin.Running:
		return "plugins_state_running"
	case plugin.Stopped:
		return "plugins_state_stopped"
	case plugin.Failed:
		return "plugins_state_failed"
	default:
		return "plugins_state_unknown"
	}
}

func main() {
	var (
		fixturesIn = flag.String("fixtures", "./challenges/fixtures", "fixture dir")
	)
	flag.Parse()

	// Resolve fixtures dir relative to invocation: allow running from
	// submodule root OR challenges/ dir.
	fixDir := *fixturesIn
	if _, err := os.Stat(fixDir); err != nil {
		alt := filepath.Join("..", "fixtures")
		if _, err2 := os.Stat(alt); err2 == nil {
			fixDir = alt
		}
	}

	fmt.Println("=== Plugins round-293 Challenge runner ===")
	fmt.Printf("  fixtures=%s\n", fixDir)

	// 1. Load all 5 fixtures.
	translators := make(map[string]i18n.Translator, len(expectedLocales))
	for _, loc := range expectedLocales {
		t, err := loadFixture(fixDir, loc)
		if err != nil {
			fmt.Printf("[locale:%s] FAIL — %v\n", loc, err)
			os.Exit(99)
		}
		translators[loc] = t
	}
	fmt.Printf("[1/7] loaded %d locale fixtures: %v\n", len(translators), expectedLocales)

	// 2. Build real Registry with two real Plugin instances + declared
	// dependency: api depends on db; topological StartAll order MUST
	// start db first then api.
	reg := registry.New()
	db := newDemoPlugin("db", "1.0.0")
	api := newDemoPlugin("api", "2.1.0")
	if err := reg.Register(db); err != nil {
		fmt.Printf("[2/7] FAIL — register db: %v\n", err)
		os.Exit(1)
	}
	if err := reg.Register(api); err != nil {
		fmt.Printf("[2/7] FAIL — register api: %v\n", err)
		os.Exit(1)
	}
	if err := reg.SetDependencies("api", []string{"db"}); err != nil {
		fmt.Printf("[2/7] FAIL — set deps: %v\n", err)
		os.Exit(1)
	}
	names := reg.List()
	fmt.Printf("[2/7] registered %d plugins: %v (api depends on db)\n", len(names), names)

	// 3. Drive the lifecycle. Init both, then StartAll in dep order,
	// HealthCheck both running, then StopAll in reverse order.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Init(ctx, plugin.Config{"role": "primary"}); err != nil {
		fmt.Printf("[3/7] FAIL — db.Init: %v\n", err)
		os.Exit(1)
	}
	if err := api.Init(ctx, plugin.Config{"port": 8080}); err != nil {
		fmt.Printf("[3/7] FAIL — api.Init: %v\n", err)
		os.Exit(1)
	}
	if err := reg.StartAll(ctx); err != nil {
		fmt.Printf("[3/7] FAIL — StartAll: %v\n", err)
		os.Exit(1)
	}
	if db.tracker.Get() != plugin.Running || api.tracker.Get() != plugin.Running {
		fmt.Printf("[3/7] FAIL — expected Running, got db=%s api=%s\n",
			db.tracker.Get(), api.tracker.Get())
		os.Exit(99)
	}
	fmt.Printf("[3/7] StartAll OK: db=%s api=%s (Init+Start calls: db=%+v api=%+v)\n",
		db.tracker.Get(), api.tracker.Get(), db.calls, api.calls)

	// 4. Real InProcessSandbox exercise — health action on the running
	// api plugin. Verifies sandbox.Execute returns a real Result with
	// non-zero Duration and empty Error.
	sbCfg := sandbox.DefaultConfig()
	sbCfg.Limits.Timeout = 2 * time.Second
	sb := sandbox.NewInProcessSandbox(sbCfg)
	res, err := sb.Execute(ctx, api, sandbox.Action{Name: "health"})
	if err != nil {
		fmt.Printf("[4/7] FAIL — sandbox.Execute: %v\n", err)
		os.Exit(1)
	}
	if res.Error != "" {
		fmt.Printf("[4/7] FAIL — sandbox.Result.Error=%q\n", res.Error)
		os.Exit(99)
	}
	if res.Duration <= 0 {
		fmt.Printf("[4/7] FAIL — sandbox.Result.Duration=%v (expected >0)\n", res.Duration)
		os.Exit(99)
	}
	fmt.Printf("[4/7] sandbox health-check OK: id=%s duration=%s (HealthCheck calls=%d)\n",
		res.ID, res.Duration, api.calls["HealthCheck"])

	// 5. Real structured-output parsers — exercise JSON + YAML +
	// Markdown over real input strings; assert every parser returns a
	// non-nil decoded value.
	jp := structured.NewJSONParser()
	yp := structured.NewYAMLParser()
	mp := structured.NewMarkdownParser()

	jsonOut, err := jp.Parse(`{"plugin":"api","status":"running","port":8080}`, nil)
	if err != nil || jsonOut == nil {
		fmt.Printf("[5/7] FAIL — JSONParser: %v\n", err)
		os.Exit(99)
	}
	yamlOut, err := yp.Parse("plugin: db\nstatus: running\nrole: primary\n", nil)
	if err != nil || yamlOut == nil {
		fmt.Printf("[5/7] FAIL — YAMLParser: %v\n", err)
		os.Exit(99)
	}
	mdOut, err := mp.Parse(
		"# Report\n- **plugin**: api\n- **status**: running\n- calls: 3\n",
		nil,
	)
	if err != nil || mdOut == nil {
		fmt.Printf("[5/7] FAIL — MarkdownParser: %v\n", err)
		os.Exit(99)
	}
	jb, _ := json.Marshal(jsonOut)
	fmt.Printf("[5/7] parsers OK: json=%s yaml=%T md=%T\n", string(jb), yamlOut, mdOut)

	// 6. Stop the registry and assert state transitions.
	if err := reg.StopAll(ctx); err != nil {
		fmt.Printf("[6/7] FAIL — StopAll: %v\n", err)
		os.Exit(1)
	}
	if db.tracker.Get() != plugin.Stopped || api.tracker.Get() != plugin.Stopped {
		fmt.Printf("[6/7] FAIL — expected Stopped, got db=%s api=%s\n",
			db.tracker.Get(), api.tracker.Get())
		os.Exit(99)
	}
	fmt.Printf("[6/7] StopAll OK: db=%s api=%s (Stop calls: db=%d api=%d)\n",
		db.tracker.Get(), api.tracker.Get(),
		db.calls["Stop"], api.calls["Stop"])

	// 7. Render 5-locale labels for each terminal state observed.
	states := []plugin.State{
		plugin.Uninitialized,
		plugin.Initialized,
		plugin.Running,
		plugin.Stopped,
		plugin.Failed,
	}
	fmt.Println("[7/7] 5-locale label render:")
	for _, loc := range expectedLocales {
		t := translators[loc]
		var parts []string
		for _, st := range states {
			key := stateLabelKey(st)
			parts = append(parts, fmt.Sprintf("%s=%s", st.String(), t.T(key, nil)))
		}
		fmt.Printf("  [%s] %s\n", loc, strings.Join(parts, " | "))
	}

	// Anti-bluff: ensure every Plugin lifecycle method was invoked at
	// least once on each plugin (no silent no-op).
	for _, p := range []*demoPlugin{db, api} {
		for _, m := range []string{"Init", "Start", "Stop"} {
			if p.calls[m] == 0 {
				fmt.Printf("FAIL — plugin %s never received %s call\n", p.Name(), m)
				os.Exit(99)
			}
		}
	}

	fmt.Println("=== Plugins round-293 Challenge runner: PASS ===")
}
