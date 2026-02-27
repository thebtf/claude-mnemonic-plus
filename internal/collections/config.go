// Package collections manages YAML-based collection configuration.
package collections

import (
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Collection describes a named document namespace.
type Collection struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	PathContext  map[string]string `yaml:"path_context"`
}

// Config is the top-level YAML structure.
type Config struct {
	Collections []Collection `yaml:"collections"`
}

// Registry holds loaded collections, keyed by name.
type Registry struct {
	byName map[string]*Collection
	order  []string // preserves definition order
}

// Load reads the YAML file at path and returns a Registry.
// If the file does not exist, Load returns an empty Registry (not an error).
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{byName: make(map[string]*Collection)}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	r := &Registry{
		byName: make(map[string]*Collection, len(cfg.Collections)),
	}
	for i := range cfg.Collections {
		c := &cfg.Collections[i]
		r.byName[c.Name] = c
		r.order = append(r.order, c.Name)
	}
	return r, nil
}

// Get returns a collection by name. Returns (nil, false) if not found.
func (r *Registry) Get(name string) (*Collection, bool) {
	c, ok := r.byName[name]
	return c, ok
}

// All returns all collections in definition order.
func (r *Registry) All() []*Collection {
	result := make([]*Collection, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.byName[name])
	}
	return result
}

// Names returns a sorted list of collection names.
func (r *Registry) Names() []string {
	names := make([]string, len(r.order))
	copy(names, r.order)
	sort.Strings(names)
	return names
}
