// Package importer detects CI platform from file paths and content.
package importer

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DetectPlatform returns "gitlab" or "github" based on file extension and content.
// For .yml/.yaml files without a recognizable path, it returns "" so the caller
// can fall back to DetectPlatformFromContent.
func DetectPlatform(path string) (string, error) {
	ext := strings.ToLower(path)
	if !strings.HasSuffix(ext, ".yml") && !strings.HasSuffix(ext, ".yaml") {
		return "", fmt.Errorf("not a YAML file: %q", path)
	}
	// Fast path: recognizable file paths
	if strings.Contains(path, ".gitlab-ci") {
		return "gitlab", nil
	}
	if strings.Contains(path, ".github/workflows") {
		return "github", nil
	}
	// Valid YAML but unrecognizable path — caller should use DetectPlatformFromContent
	return "", nil
}

// DetectPlatformFromContent inspects YAML top-level keys to distinguish platforms.
// GitLab CI has "stages" or job-name keys; GitHub Actions has "jobs" + "on".
func DetectPlatformFromContent(data []byte) string {
	var top map[string]yaml.Node
	if yaml.Unmarshal(data, &top) != nil {
		return ""
	}
	if _, ok := top["jobs"]; ok {
		if _, ok := top["on"]; ok {
			return "github"
		}
	}
	// GitLab: has "stages" or any key that looks like a job (not a known GitHub key)
	if _, ok := top["stages"]; ok {
		return "gitlab"
	}
	return "gitlab" // default: GitLab is more common and has looser structure
}
