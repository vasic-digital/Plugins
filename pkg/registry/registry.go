// Package registry provides a thread-safe plugin registry with dependency
// ordering, version constraint checking, and lifecycle management.
package registry

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"digital.vasic.plugins/pkg/plugin"
)

// Registry manages registered plugins with thread-safe operations.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]plugin.Plugin
	deps    map[string][]string // plugin name -> dependency names
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{
		plugins: make(map[string]plugin.Plugin),
		deps:    make(map[string][]string),
	}
}

// Register adds a plugin to the registry. Returns an error if a plugin
// with the same name is already registered.
func (r *Registry) Register(p plugin.Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot register nil plugin")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = p
	return nil
}

// Get returns the plugin with the given name and true, or nil and false
// if not found.
func (r *Registry) Get(name string) (plugin.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[name]
	return p, exists
}

// List returns the names of all registered plugins.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// Remove unregisters a plugin by name. Returns an error if not found.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	delete(r.plugins, name)
	delete(r.deps, name)
	return nil
}

// SetDependencies declares that pluginName depends on the given list
// of dependency names. Used by StartAll/StopAll for ordering.
func (r *Registry) SetDependencies(pluginName string, dependencies []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[pluginName]; !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	r.deps[pluginName] = dependencies
	return nil
}

// StartAll starts all registered plugins in dependency order.
// Plugins with no dependencies are started first. If a cycle is
// detected an error is returned.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	order, err := r.resolveOrder()
	plugins := r.plugins
	r.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	for _, name := range order {
		p, ok := plugins[name]
		if !ok {
			continue
		}
		if err := p.Start(ctx); err != nil {
			return fmt.Errorf("failed to start plugin %s: %w", name, err)
		}
	}
	return nil
}

// StopAll stops all registered plugins in reverse dependency order.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	order, err := r.resolveOrder()
	plugins := r.plugins
	r.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Reverse order: dependents stop before dependencies.
	var errs []string
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		p, ok := plugins[name]
		if !ok {
			continue
		}
		if err := p.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping plugins: %s",
			strings.Join(errs, "; "))
	}
	return nil
}

// resolveOrder performs topological sort (Kahn's algorithm) on the
// dependency graph. Must be called with at least r.mu.RLock held.
func (r *Registry) resolveOrder() ([]string, error) {
	inDegree := make(map[string]int)
	for name := range r.plugins {
		inDegree[name] = 0
	}

	// Build in-degree map.
	for name, deps := range r.deps {
		if _, ok := r.plugins[name]; !ok {
			continue
		}
		for _, dep := range deps {
			if _, ok := r.plugins[dep]; ok {
				inDegree[name]++
			}
		}
	}

	// Seed queue with zero in-degree nodes.
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// Find plugins that depend on node and decrement their in-degree.
		for name, deps := range r.deps {
			if _, ok := r.plugins[name]; !ok {
				continue
			}
			for _, dep := range deps {
				if dep == node {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = append(queue, name)
					}
				}
			}
		}
	}

	if len(order) != len(r.plugins) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return order, nil
}

// CheckVersionConstraint checks whether a plugin's version satisfies a
// semver constraint. Supported operators: =, >=, <=, >, <, ^, ~.
func CheckVersionConstraint(version, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true, nil
	}

	var op string
	var target string

	for _, prefix := range []string{">=", "<=", "^", "~", ">", "<", "="} {
		if strings.HasPrefix(constraint, prefix) {
			op = prefix
			target = strings.TrimSpace(constraint[len(prefix):])
			break
		}
	}
	if op == "" {
		op = "="
		target = constraint
	}

	vParts, err := parseSemver(version)
	if err != nil {
		return false, fmt.Errorf("invalid version %q: %w", version, err)
	}
	tParts, err := parseSemver(target)
	if err != nil {
		return false, fmt.Errorf("invalid constraint version %q: %w", target, err)
	}

	cmp := compareSemver(vParts, tParts)

	switch op {
	case "=":
		return cmp == 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<=":
		return cmp <= 0, nil
	case ">":
		return cmp > 0, nil
	case "<":
		return cmp < 0, nil
	case "^":
		// ^1.2.3 means >=1.2.3, <2.0.0
		if cmp < 0 {
			return false, nil
		}
		next := [3]int{tParts[0] + 1, 0, 0}
		return compareSemver(vParts, next) < 0, nil
	case "~":
		// ~1.2.3 means >=1.2.3, <1.3.0
		if cmp < 0 {
			return false, nil
		}
		next := [3]int{tParts[0], tParts[1] + 1, 0}
		return compareSemver(vParts, next) < 0, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", op)
	}
}

func parseSemver(v string) ([3]int, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("expected major.minor.patch, got %q", v)
	}
	var result [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("invalid number %q: %w", p, err)
		}
		result[i] = n
	}
	return result, nil
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
