// Package synth provides shared utilities for synthesizers.
package synth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
		valNode := toYAMLNode(m.values[k])
		node.Content = append(node.Content, keyNode, valNode)
	}
	return node, nil
}

// toYAMLNode converts a value to a *yaml.Node, preserving any *yaml.Node
// values found in maps or slices (which keeps style hints like LiteralStyle).
func toYAMLNode(v any) *yaml.Node {
	switch val := v.(type) {
	case *yaml.Node:
		return val
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for k, v2 := range val {
			n.Content = append(n.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: k},
				toYAMLNode(v2),
			)
		}
		return n
	default:
		// Fall back to marshal/unmarshal for everything else
		n := &yaml.Node{}
		b, err := yaml.Marshal(v)
		if err != nil {
			return n
		}
		if err := yaml.Unmarshal(b, n); err != nil {
			return n
		}
		if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
			return n.Content[0]
		}
		return n
	}
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
	b = []byte(fixQuotedMultilineScalars(string(b)))
	return os.WriteFile(filepath.Join(dir, name), b, 0o644)
}

// quotedMultilineRe matches YAML values that are double-quoted strings with
// \n escapes. Captures: (1) prefix before the quote, (2) quoted content.
// Matches both list items (- "...") and map values (key: "...").
var quotedMultilineRe = regexp.MustCompile(`(?m)^(\s+(?:- |\S+: ))"((?:[^"\\]|\\.)*)"\s*$`)

// unicodeEscapeRe matches \UXXXXXXXX escape sequences in YAML double-quoted strings.
var unicodeEscapeRe = regexp.MustCompile(`\\U([0-9A-Fa-f]{8})`)

// fixQuotedMultilineScalars converts double-quoted YAML scalars that contain
// \n escapes into literal block scalars (|). This works around a yaml.v3
// limitation where LiteralStyle is ignored for strings with non-BMP Unicode.
func fixQuotedMultilineScalars(s string) string {
	return quotedMultilineRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := quotedMultilineRe.FindStringSubmatch(match)
		if len(sub) < 3 || !strings.Contains(sub[2], "\\n") {
			return match
		}
		prefix := sub[1] // e.g. "    - " or "              run: "
		content := sub[2]

		// Unescape YAML double-quoted string sequences.
		// Order matters: replace \\ first (via placeholder) to avoid
		// misinterpreting \\n as \+newline instead of \n literal.
		const placeholder = "\x00BACKSLASH\x00"
		content = strings.ReplaceAll(content, "\\\\", placeholder)
		content = strings.ReplaceAll(content, "\\n", "\n")
		content = strings.ReplaceAll(content, "\\\"", "\"")
		content = strings.ReplaceAll(content, "\\t", "\t")
		content = strings.ReplaceAll(content, placeholder, "\\")
		content = unicodeEscapeRe.ReplaceAllStringFunc(content, func(m string) string {
			var cp int
			_, _ = fmt.Sscanf(m, "\\U%08x", &cp)
			return string(rune(cp))
		})

		// Determine base indent for block content.
		// For "    - ", base is "    " and block indent is "      " (base + 2)
		// For "              run: ", base is "              " and block indent is base + 2
		var baseIndent string
		if strings.HasSuffix(prefix, "- ") {
			baseIndent = strings.TrimSuffix(prefix, "- ")
		} else {
			// Map value like "run: " — strip the key part, keep only leading whitespace
			baseIndent = prefix[:len(prefix)-len(strings.TrimLeft(prefix, " "))]
		}
		blockIndent := baseIndent + "  "

		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		var b strings.Builder
		b.WriteString(prefix + "|\n")
		for _, line := range lines {
			if line == "" {
				b.WriteString("\n")
			} else {
				b.WriteString(blockIndent + line + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	})
}
