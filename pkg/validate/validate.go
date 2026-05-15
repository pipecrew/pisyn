// Package validate validates generated CI/CD YAML against official JSON schemas.
package validate

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schemas/github-workflow.json schemas/gitlab-ci.json
var schemas embed.FS

var schemaFiles = map[string]string{
	"github": "schemas/github-workflow.json",
	"gitlab": "schemas/gitlab-ci.json",
}

var (
	compiledSchemas   = map[string]*jsonschema.Schema{}
	compiledSchemasMu sync.Mutex
)

// getCompiledSchema returns a cached compiled schema, compiling on first access.
func getCompiledSchema(platform string) (*jsonschema.Schema, error) {
	compiledSchemasMu.Lock()
	defer compiledSchemasMu.Unlock()

	if s, ok := compiledSchemas[platform]; ok {
		return s, nil
	}

	schemaFile, ok := schemaFiles[platform]
	if !ok {
		return nil, fmt.Errorf("no schema for platform: %s", platform)
	}

	schemaData, err := schemas.ReadFile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaFile, schemaDoc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := c.Compile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	compiledSchemas[platform] = compiled
	return compiled, nil
}

// Validate checks a YAML byte slice against the schema for the given platform.
// Returns nil if valid, or an error describing validation failures.
func Validate(platform string, yamlData []byte) error {
	schema, err := getCompiledSchema(platform)
	if err != nil {
		return err
	}

	var doc any
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	doc = convertYAML(doc)

	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("validation failed:\n%s", err)
	}
	return nil
}

// convertYAML recursively converts map[string]any and []any from YAML parsing
// to ensure all types are JSON-schema-compatible.
func convertYAML(v any) any {
	switch v := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, val := range v {
			m[k] = convertYAML(val)
		}
		return m
	case []any:
		a := make([]any, len(v))
		for i, val := range v {
			a[i] = convertYAML(val)
		}
		return a
	case int:
		return float64(v)
	default:
		return v
	}
}
