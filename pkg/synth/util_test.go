package synth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestToYAMLNode_PreservesYAMLNode(t *testing.T) {
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: "hello", Style: yaml.LiteralStyle}
	got := toYAMLNode(n)
	if got != n {
		t.Error("expected same pointer back for *yaml.Node input")
	}
}

func TestToYAMLNode_MapWithNestedYAMLNode(t *testing.T) {
	inner := &yaml.Node{Kind: yaml.ScalarNode, Value: "preserved", Style: yaml.LiteralStyle}
	m := map[string]any{"key": inner, "other": "plain"}
	got := toYAMLNode(m)

	if got.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode, got %d", got.Kind)
	}
	// Find the "key" entry and verify the node is preserved
	for i := 0; i < len(got.Content)-1; i += 2 {
		if got.Content[i].Value == "key" {
			if got.Content[i+1] != inner {
				t.Error("expected inner *yaml.Node to be preserved in map")
			}
			return
		}
	}
	t.Error("key not found in mapping node")
}

func TestToYAMLNode_PlainValues(t *testing.T) {
	tests := []struct {
		name string
		in   any
		kind yaml.Kind
	}{
		{"string", "hello", yaml.ScalarNode},
		{"int", 42, yaml.ScalarNode},
		{"string_slice", []string{"a", "b"}, yaml.SequenceNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toYAMLNode(tt.in)
			if got.Kind != tt.kind {
				t.Errorf("expected kind %d, got %d", tt.kind, got.Kind)
			}
		})
	}
}

func TestFixQuotedMultilineScalars_WithEmoji(t *testing.T) {
	// Simulate what yaml.v3 produces for a multi-line string with non-BMP Unicode
	input := `    - "echo hello\\necho \U0001F41E\\necho done"` + "\n"
	got := fixQuotedMultilineScalars(input)

	if strings.Contains(got, `\U0001F41E`) {
		t.Error("Unicode escape was not converted")
	}
	if !strings.Contains(got, "🐞") {
		t.Error("expected emoji 🐞 in output")
	}
	if !strings.Contains(got, "- |") {
		t.Error("expected literal block scalar indicator")
	}
	// Verify block structure
	lines := strings.Split(got, "\n")
	if !strings.HasSuffix(lines[0], "- |") {
		t.Errorf("first line should end with '- |', got: %q", lines[0])
	}
}

func TestFixQuotedMultilineScalars_PreservesSimpleStrings(t *testing.T) {
	input := `    - "just a simple string"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if got != input {
		t.Errorf("simple string should be unchanged, got: %q", got)
	}
}

func TestFixQuotedMultilineScalars_PreservesNonListQuotedStrings(t *testing.T) {
	// Top-level key: value without indentation should not match
	input := `key: "some\\nvalue"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if got != input {
		t.Errorf("non-indented quoted string should be unchanged, got: %q", got)
	}
}

func TestFixQuotedMultilineScalars_MapValue(t *testing.T) {
	// GitHub Actions style: run: "..." with emoji
	input := `              run: "echo \U0001F41E\necho done"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if strings.Contains(got, `\U0001F41E`) {
		t.Error("Unicode escape was not converted")
	}
	if !strings.Contains(got, "🐞") {
		t.Error("expected emoji 🐞 in output")
	}
	if !strings.Contains(got, "run: |") {
		t.Errorf("expected 'run: |' block scalar, got:\n%s", got)
	}
	// Content should be indented relative to the key
	lines := strings.Split(got, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), got)
	}
	// Content lines should have more indent than the run: key
	for _, l := range lines[1:] {
		if l == "" {
			continue
		}
		if !strings.HasPrefix(l, "                ") { // 16 spaces
			t.Errorf("content line has wrong indent: %q", l)
		}
	}
}

func TestFixQuotedMultilineScalars_UnescapesBackslash(t *testing.T) {
	// yaml.v3 output for "line1\npath\\to\\file\necho 🐞"
	input := `    - "line1\npath\\to\\file\necho \U0001F41E"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if !strings.Contains(got, `path\to\file`) {
		t.Errorf("backslashes not unescaped correctly, got:\n%s", got)
	}
	if !strings.Contains(got, "🐞") {
		t.Errorf("emoji not unescaped, got:\n%s", got)
	}
}

func TestFixQuotedMultilineScalars_UnescapesQuotes(t *testing.T) {
	// yaml.v3 output for `echo "hello"\necho 🐞 done`
	input := `    - "echo \"hello\"\necho \U0001F41E done"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if !strings.Contains(got, `echo "hello"`) {
		t.Errorf("quotes not unescaped correctly, got:\n%s", got)
	}
}

func TestFixQuotedMultilineScalars_UnescapesTabs(t *testing.T) {
	// yaml.v3 output for "col1\tcol2\ncol3\tcol4 🐞"
	input := `    - "col1\tcol2\ncol3\tcol4 \U0001F41E"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if !strings.Contains(got, "col1\tcol2") {
		t.Errorf("tabs not unescaped correctly, got:\n%s", got)
	}
}

func TestFixQuotedMultilineScalars_EmptyLines(t *testing.T) {
	// yaml.v3 output for "line1\n\nline3 🐞"
	input := `    - "line1\n\nline3 \U0001F41E"` + "\n"
	got := fixQuotedMultilineScalars(input)
	lines := strings.Split(got, "\n")
	foundEmpty := false
	for _, l := range lines {
		if l == "" {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Errorf("expected empty line in block scalar, got:\n%s", got)
	}
}

func TestFixQuotedMultilineScalars_MultipleUnicodeEscapes(t *testing.T) {
	input := `    - "echo \U0001F41E\\necho \U0001F6A8"` + "\n"
	got := fixQuotedMultilineScalars(input)
	if !strings.Contains(got, "🐞") {
		t.Error("missing 🐞")
	}
	if !strings.Contains(got, "🚨") {
		t.Error("missing 🚨")
	}
}

func TestFixQuotedMultilineScalars_IndentPreserved(t *testing.T) {
	// 8-space indent (deeply nested)
	input := `        - "a\\nb"` + "\n"
	got := fixQuotedMultilineScalars(input)
	lines := strings.Split(got, "\n")
	if !strings.HasPrefix(lines[0], "        - |") {
		t.Errorf("indent not preserved, got: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "          a") {
		t.Errorf("block content indent wrong, got: %q", lines[1])
	}
}

func TestWriteYAML_MultilineEmojiScript(t *testing.T) {
	dir := t.TempDir()
	data := map[string]any{
		"script": []string{
			"echo hello",
			"function log() {\n  echo 🐞\n}",
		},
	}
	if err := WriteYAML(dir, "test.yml", data); err != nil {
		t.Fatalf("WriteYAML: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "test.yml"))
	out := string(b)

	if strings.Contains(out, `\U0001F41E`) {
		t.Errorf("Unicode escape should be fixed in output:\n%s", out)
	}
	if !strings.Contains(out, "🐞") {
		t.Errorf("emoji should be present in output:\n%s", out)
	}
	if !strings.Contains(out, "- |") {
		t.Errorf("multi-line script should use block scalar:\n%s", out)
	}
}

func TestWriteYAML_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	if err := WriteYAML(dir, "test.yml", map[string]string{"key": "val"}); err != nil {
		t.Fatalf("WriteYAML: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "test.yml")); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestOrderedMap_PreservesInsertionOrder(t *testing.T) {
	m := NewOrderedMap()
	m.Set("z", "last")
	m.Set("a", "first")
	m.Set("m", "middle")

	b, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(b)
	zIdx := strings.Index(out, "z:")
	aIdx := strings.Index(out, "a:")
	mIdx := strings.Index(out, "m:")
	if zIdx > aIdx || aIdx > mIdx {
		t.Errorf("order not preserved: z=%d a=%d m=%d\n%s", zIdx, aIdx, mIdx, out)
	}
}

func TestOrderedMap_PreservesYAMLNodeStyle(t *testing.T) {
	m := NewOrderedMap()
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	seq.Content = append(seq.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "line1\nline2",
		Style: yaml.LiteralStyle,
	})
	m.Set("script", seq)

	b, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, "- |") {
		t.Errorf("LiteralStyle not preserved:\n%s", out)
	}
}
