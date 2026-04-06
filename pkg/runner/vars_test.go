package runner

import (
	"testing"
)

func TestExpandVars_BasicSubstitution(t *testing.T) {
	resolved := map[string]string{
		"PISYN_COMMIT_SHA":    "abc123",
		"PISYN_COMMIT_BRANCH": "main",
	}
	env := map[string]string{
		"VERSION": "sha-$PISYN_COMMIT_SHA",
		"BRANCH":  "$PISYN_COMMIT_BRANCH",
		"PLAIN":   "no-vars-here",
	}

	out := ExpandVars(env, resolved)

	if out["VERSION"] != "sha-abc123" {
		t.Errorf("VERSION = %q, want %q", out["VERSION"], "sha-abc123")
	}
	if out["BRANCH"] != "main" {
		t.Errorf("BRANCH = %q, want %q", out["BRANCH"], "main")
	}
	if out["PLAIN"] != "no-vars-here" {
		t.Errorf("PLAIN = %q, want %q", out["PLAIN"], "no-vars-here")
	}
}

func TestExpandVars_NoDoubleExpansion(t *testing.T) {
	// If a resolved value contains $PISYN_*, it should NOT be expanded again
	resolved := map[string]string{
		"PISYN_PROJECT_NAME": "$PISYN_COMMIT_SHA-app",
		"PISYN_COMMIT_SHA":   "abc123",
	}
	env := map[string]string{
		"NAME": "$PISYN_PROJECT_NAME",
	}

	out := ExpandVars(env, resolved)

	// Single-pass: $PISYN_PROJECT_NAME → "$PISYN_COMMIT_SHA-app" (literal, not expanded further)
	if out["NAME"] != "$PISYN_COMMIT_SHA-app" {
		t.Errorf("NAME = %q, want %q (no double expansion)", out["NAME"], "$PISYN_COMMIT_SHA-app")
	}
}

func TestExpandVars_MultipleVarsInOneValue(t *testing.T) {
	resolved := map[string]string{
		"PISYN_PROJECT_PATH": "org/repo",
		"PISYN_COMMIT_SHA":   "abc123",
	}
	env := map[string]string{
		"TAG": "$PISYN_PROJECT_PATH:$PISYN_COMMIT_SHA",
	}

	out := ExpandVars(env, resolved)

	if out["TAG"] != "org/repo:abc123" {
		t.Errorf("TAG = %q, want %q", out["TAG"], "org/repo:abc123")
	}
}

func TestExpandVars_EmptyEnv(t *testing.T) {
	out := ExpandVars(map[string]string{}, map[string]string{"PISYN_X": "y"})
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}

func TestResolveLocalVars_HasAllKeys(t *testing.T) {
	vars := ResolveLocalVars(".")
	expected := []string{
		"PISYN_COMMIT_SHA", "PISYN_COMMIT_BRANCH", "PISYN_COMMIT_TAG",
		"PISYN_COMMIT_MESSAGE", "PISYN_DEFAULT_BRANCH", "PISYN_PROJECT_DIR",
		"PISYN_PROJECT_NAME", "PISYN_PROJECT_PATH", "PISYN_PROJECT_NAMESPACE",
		"PISYN_PIPELINE_ID", "PISYN_JOB_ID", "PISYN_JOB_TOKEN",
		"PISYN_MR_ID", "PISYN_MR_SOURCE_BRANCH", "PISYN_MR_TARGET_BRANCH",
		"PISYN_REF_PROTECTED",
	}
	for _, key := range expected {
		if _, ok := vars[key]; !ok {
			t.Errorf("missing key %s", key)
		}
	}
}
