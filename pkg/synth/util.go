// Package synth provides shared utilities for synthesizers.
package synth

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// OrderedMap preserves insertion order for YAML output.
type OrderedMap struct {
	keys   []string
	values map[string]any
}

// NewOrderedMap creates a new ordered map.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: map[string]any{}}
}

// Set adds or updates a key-value pair, preserving insertion order.
func (m *OrderedMap) Set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// MarshalYAML implements yaml.Marshaler with ordered keys.
func (m *OrderedMap) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range m.keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
		valNode := &yaml.Node{}
		b, err := yaml.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, valNode); err != nil {
			return nil, err
		}
		// Unwrap document node
		if valNode.Kind == yaml.DocumentNode && len(valNode.Content) > 0 {
			valNode = valNode.Content[0]
		}
		node.Content = append(node.Content, keyNode, valNode)
	}
	return node, nil
}

// WriteYAML marshals data to YAML and writes it to dir/name.
func WriteYAML(dir, name string, data any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), b, 0o644)
}
