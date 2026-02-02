package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.plugins/pkg/plugin"
)

// --- mock plugin ---

type mockPlugin struct {
	name       string
	version    string
	started    bool
	stopped    bool
	startErr   error
	stopErr    error
	startOrder *[]string
	mu         sync.Mutex
}

func newMock(name, version string) *mockPlugin {
	return &mockPlugin{name: name, version: version}
}

func (m *mockPlugin) Name() string    { return m.name }
func (m *mockPlugin) Version() string { return m.version }

func (m *mockPlugin) Init(_ context.Context, _ plugin.Config) error { return nil }

func (m *mockPlugin) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	if m.startOrder != nil {
		*m.startOrder = append(*m.startOrder, m.name)
	}
	return nil
}

func (m *mockPlugin) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped = true
	return nil
}

func (m *mockPlugin) HealthCheck(_ context.Context) error { return nil }

// --- Register tests ---

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name    string
		plugin  plugin.Plugin
		wantErr bool
		errMsg  string
	}{
		{
			name:    "successful registration",
			plugin:  newMock("p1", "1.0.0"),
			wantErr: false,
		},
		{
			name:    "nil plugin",
			plugin:  nil,
			wantErr: true,
			errMsg:  "cannot register nil plugin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			err := r.Register(tt.plugin)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := New()
	require.NoError(t, r.Register(newMock("p1", "1.0.0")))

	err := r.Register(newMock("p1", "2.0.0"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Register_EmptyName(t *testing.T) {
	r := New()
	err := r.Register(newMock("", "1.0.0"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// --- Get tests ---

func TestRegistry_Get(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Registry)
		query  string
		wantOK bool
	}{
		{
			name: "existing plugin",
			setup: func(r *Registry) {
				_ = r.Register(newMock("p1", "1.0.0"))
			},
			query:  "p1",
			wantOK: true,
		},
		{
			name:   "missing plugin",
			setup:  func(_ *Registry) {},
			query:  "p1",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			tt.setup(r)
			p, ok := r.Get(tt.query)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.NotNil(t, p)
			} else {
				assert.Nil(t, p)
			}
		})
	}
}

// --- List tests ---

func TestRegistry_List(t *testing.T) {
	r := New()
	assert.Empty(t, r.List())

	_ = r.Register(newMock("a", "1.0.0"))
	_ = r.Register(newMock("b", "1.0.0"))

	list := r.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "a")
	assert.Contains(t, list, "b")
}

// --- Remove tests ---

func TestRegistry_Remove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Registry)
		target  string
		wantErr bool
	}{
		{
			name: "remove existing",
			setup: func(r *Registry) {
				_ = r.Register(newMock("p1", "1.0.0"))
			},
			target:  "p1",
			wantErr: false,
		},
		{
			name:    "remove non-existent",
			setup:   func(_ *Registry) {},
			target:  "p1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			tt.setup(r)
			err := r.Remove(tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				_, ok := r.Get(tt.target)
				assert.False(t, ok)
			}
		})
	}
}

// --- StartAll / StopAll tests ---

func TestRegistry_StartAll_NoDeps(t *testing.T) {
	r := New()
	p1 := newMock("p1", "1.0.0")
	p2 := newMock("p2", "1.0.0")
	_ = r.Register(p1)
	_ = r.Register(p2)

	err := r.StartAll(context.Background())
	require.NoError(t, err)
	assert.True(t, p1.started)
	assert.True(t, p2.started)
}

func TestRegistry_StopAll(t *testing.T) {
	r := New()
	p1 := newMock("p1", "1.0.0")
	p2 := newMock("p2", "1.0.0")
	_ = r.Register(p1)
	_ = r.Register(p2)

	err := r.StopAll(context.Background())
	require.NoError(t, err)
	assert.True(t, p1.stopped)
	assert.True(t, p2.stopped)
}

func TestRegistry_StartAll_WithDeps(t *testing.T) {
	r := New()
	var order []string
	pA := newMock("a", "1.0.0")
	pA.startOrder = &order
	pB := newMock("b", "1.0.0")
	pB.startOrder = &order

	_ = r.Register(pA)
	_ = r.Register(pB)
	// b depends on a, so a should start first.
	_ = r.SetDependencies("b", []string{"a"})

	err := r.StartAll(context.Background())
	require.NoError(t, err)

	require.Len(t, order, 2)
	assert.Equal(t, "a", order[0])
	assert.Equal(t, "b", order[1])
}

func TestRegistry_StartAll_CircularDeps(t *testing.T) {
	r := New()
	_ = r.Register(newMock("a", "1.0.0"))
	_ = r.Register(newMock("b", "1.0.0"))
	_ = r.SetDependencies("a", []string{"b"})
	_ = r.SetDependencies("b", []string{"a"})

	err := r.StartAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

func TestRegistry_StartAll_Error(t *testing.T) {
	r := New()
	p := newMock("fail", "1.0.0")
	p.startErr = fmt.Errorf("boom")
	_ = r.Register(p)

	err := r.StartAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRegistry_StopAll_Error(t *testing.T) {
	r := New()
	p := newMock("fail", "1.0.0")
	p.stopErr = fmt.Errorf("stop-err")
	_ = r.Register(p)

	err := r.StopAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop-err")
}

// --- Concurrency test ---

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", i)
			_ = r.Register(newMock(name, "1.0.0"))
			r.Get(name)
			r.List()
		}(i)
	}
	wg.Wait()

	assert.Len(t, r.List(), 100)
}

// --- SetDependencies tests ---

func TestRegistry_SetDependencies(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Registry)
		plugin  string
		deps    []string
		wantErr bool
	}{
		{
			name: "valid",
			setup: func(r *Registry) {
				_ = r.Register(newMock("p1", "1.0.0"))
			},
			plugin:  "p1",
			deps:    []string{"dep1"},
			wantErr: false,
		},
		{
			name:    "plugin not found",
			setup:   func(_ *Registry) {},
			plugin:  "p1",
			deps:    []string{"dep1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			tt.setup(r)
			err := r.SetDependencies(tt.plugin, tt.deps)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- CheckVersionConstraint tests ---

func TestCheckVersionConstraint(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		wantMatch  bool
		wantErr    bool
	}{
		{"exact match", "1.2.3", "1.2.3", true, false},
		{"exact mismatch", "1.2.3", "1.2.4", false, false},
		{"equal op", "1.2.3", "=1.2.3", true, false},
		{"gte match", "1.3.0", ">=1.2.0", true, false},
		{"gte exact", "1.2.0", ">=1.2.0", true, false},
		{"gte fail", "1.1.0", ">=1.2.0", false, false},
		{"lte match", "1.1.0", "<=1.2.0", true, false},
		{"lte fail", "1.3.0", "<=1.2.0", false, false},
		{"gt match", "1.3.0", ">1.2.0", true, false},
		{"gt fail", "1.2.0", ">1.2.0", false, false},
		{"lt match", "1.1.0", "<1.2.0", true, false},
		{"lt fail", "1.2.0", "<1.2.0", false, false},
		{"caret match", "1.5.0", "^1.2.0", true, false},
		{"caret upper", "2.0.0", "^1.2.0", false, false},
		{"caret lower", "1.1.0", "^1.2.0", false, false},
		{"tilde match", "1.2.5", "~1.2.0", true, false},
		{"tilde upper", "1.3.0", "~1.2.0", false, false},
		{"tilde lower", "1.1.9", "~1.2.0", false, false},
		{"wildcard", "5.0.0", "*", true, false},
		{"empty constraint", "1.0.0", "", true, false},
		{"invalid version", "bad", "1.0.0", false, true},
		{"invalid constraint", "1.0.0", "bad", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := CheckVersionConstraint(tt.version, tt.constraint)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMatch, ok)
			}
		})
	}
}
