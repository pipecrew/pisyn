package runner

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveLocalVars returns a map of PISYN_* variable names to their local values,
// resolved from the git repo in the given directory.
func ResolveLocalVars(dir string) map[string]string {
	projPath := projectPath(dir)
	ns := ""
	if i := strings.Index(projPath, "/"); i > 0 {
		ns = projPath[:i]
	}

	return map[string]string{
		"PISYN_COMMIT_SHA":        git(dir, "rev-parse", "HEAD"),
		"PISYN_COMMIT_BRANCH":     git(dir, "rev-parse", "--abbrev-ref", "HEAD"),
		"PISYN_COMMIT_TAG":        git(dir, "describe", "--tags", "--exact-match"),
		"PISYN_COMMIT_MESSAGE":    git(dir, "log", "-1", "--format=%s"),
		"PISYN_DEFAULT_BRANCH":    "main",
		"PISYN_PROJECT_DIR":       "/workspace",
		"PISYN_PROJECT_NAME":      filepath.Base(dir),
		"PISYN_PROJECT_PATH":      projPath,
		"PISYN_PROJECT_NAMESPACE": ns,
		"PISYN_PIPELINE_ID":       "local",
		"PISYN_JOB_ID":            "local",
		"PISYN_JOB_TOKEN":         "",
		"PISYN_MR_ID":             "",
		"PISYN_MR_SOURCE_BRANCH":  "",
		"PISYN_MR_TARGET_BRANCH":  "",
		"PISYN_REF_PROTECTED":     "false",
	}
}

// ExpandVars replaces $PISYN_* references in env values with resolved local values.
// Uses single-pass replacement to avoid non-deterministic double-expansion.
func ExpandVars(env map[string]string, resolved map[string]string) map[string]string {
	// Build replacer once from all pairs
	pairs := make([]string, 0, len(resolved)*2)
	for varName, varVal := range resolved {
		pairs = append(pairs, "$"+varName, varVal)
	}
	r := strings.NewReplacer(pairs...)

	out := make(map[string]string, len(env))
	for key, val := range env {
		out[key] = r.Replace(val)
	}
	return out
}

func git(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func projectPath(dir string) string {
	remote := git(dir, "remote", "get-url", "origin")
	if remote == "" {
		return filepath.Base(dir)
	}
	// Handle SSH: git@github.com:org/repo.git
	if i := strings.Index(remote, ":"); i > 0 && !strings.Contains(remote[:i], "/") {
		return strings.TrimSuffix(remote[i+1:], ".git")
	}
	// Handle HTTPS: https://github.com/org/repo.git
	parts := strings.Split(strings.TrimSuffix(remote, ".git"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return filepath.Base(dir)
}
