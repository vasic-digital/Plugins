package loader

import (
	goplugin "plugin"
)

// pluginHandle abstracts the Go plugin.Plugin type so tests can mock it.
type pluginHandle interface {
	Lookup(symName string) (any, error)
}

// goPluginLookup is an interface that matches *plugin.Plugin's Lookup method.
type goPluginLookup interface {
	Lookup(symName string) (goplugin.Symbol, error)
}

// pluginWrapper wraps *plugin.Plugin to satisfy pluginHandle.
type pluginWrapper struct {
	p goPluginLookup
}

func (w *pluginWrapper) Lookup(symName string) (any, error) {
	return w.p.Lookup(symName)
}

// goPluginOpenFunc is a function variable for the Go plugin.Open (for testing).
var goPluginOpenFunc = func(path string) (goPluginLookup, error) {
	return goplugin.Open(path)
}

// openPlugin opens a Go plugin shared object.
var openPlugin = func(path string) (pluginHandle, error) {
	p, err := goPluginOpenFunc(path)
	if err != nil {
		return nil, err
	}
	return &pluginWrapper{p: p}, nil
}
