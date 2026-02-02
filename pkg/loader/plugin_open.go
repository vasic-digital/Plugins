package loader

import (
	goplugin "plugin"
)

// pluginHandle abstracts the Go plugin.Plugin type so tests can mock it.
type pluginHandle interface {
	Lookup(symName string) (any, error)
}

// pluginWrapper wraps *plugin.Plugin to satisfy pluginHandle.
type pluginWrapper struct {
	p *goplugin.Plugin
}

func (w *pluginWrapper) Lookup(symName string) (any, error) {
	return w.p.Lookup(symName)
}

// openPlugin opens a Go plugin shared object.
var openPlugin = func(path string) (pluginHandle, error) {
	p, err := goplugin.Open(path)
	if err != nil {
		return nil, err
	}
	return &pluginWrapper{p: p}, nil
}
