package gitlab_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func TestSynthGitLab(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI").
		SetEnv("GO_VERSION", "1.26").
		OnPush("main")

	test := pisyn.NewStage(p, "test")
	pisyn.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test ./...").
		SetCache(pisyn.Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
		SetArtifacts(pisyn.Artifacts{Paths: []string{"coverage.out"}, ExpireIn: "7 days"}).
		Timeout(15).
		Retry(2)

	deploy := pisyn.NewStage(p, "deploy")
	pisyn.NewJob(deploy, "deploy-prod").
		Image("alpine:latest").
		Needs("unit-tests").
		Script("echo deploy").
		SetEnvironment("production", "https://app.example.com").
		SetWhen(pisyn.Manual)

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	for _, want := range []string{
		"stages:",
		"- test",
		"- deploy",
		"variables:",
		"GO_VERSION:",
		"unit-tests:",
		"image: golang:1.26",
		"script:",
		"- go test ./...",
		"stage: test",
		"cache:",
		"key: go-mod",
		"artifacts:",
		"expire_in: 7 days",
		"timeout: 15m",
		"retry: 2",
		"deploy-prod:",
		"stage: deploy",
		"needs:",
		"- unit-tests",
		"when: manual",
		"environment:",
		"name: production",
		"url: https://app.example.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitLabWorkflowRules(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI").
		AddWorkflowRule(pisyn.Rule{If: "$CI_MERGE_REQUEST_ID"}).
		AddWorkflowRule(pisyn.Rule{
			If:        `$CI_COMMIT_REF_PROTECTED == "true"`,
			Variables: map[string]string{"TYPE": "protected"},
		})

	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "build").Script("echo hi")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	for _, want := range []string{
		"workflow:",
		"$CI_MERGE_REQUEST_ID",
		"TYPE: protected",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitLabAdvancedJob(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")

	pisyn.NewJob(st, "complex").
		Image("golang:1.26").
		ImageEntrypoint("").
		ImageUser("runner").
		If(pisyn.VarCommitBranch+` == "main"`).
		AddRule(pisyn.Rule{Changes: []string{"**/*.go"}, When: "always"}).
		AddTag("docker").
		SetInterruptible(true).
		EmptyNeedsList().
		AllowFailureOnExitCodes(1, 2).
		SetRetry(pisyn.RetryConfig{Max: 2, When: []string{"script_failure"}}).
		AddServiceWithVars("postgres:16", "db", map[string]string{"POSTGRES_DB": "test"}).
		SetMatrix(map[string][]string{"go": {"1.21", "1.26"}}).
		Script("go test ./...")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	for _, want := range []string{
		"entrypoint:",
		"user: runner",
		`$CI_COMMIT_BRANCH == "main"`,
		"changes:",
		"tags:",
		"- docker",
		"interruptible: true",
		"needs: []",
		"exit_codes:",
		"script_failure",
		"POSTGRES_DB: test",
		"parallel:",
		"matrix:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitLabOutputs(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	genStage := pisyn.NewStage(p, "generate")
	gen := pisyn.NewJob(genStage, "gen-version").
		Script(`echo "VERSION=1.2.3" > build.env`).
		Output("VERSION", "build.env")

	deployStage := pisyn.NewStage(p, "deploy")
	pisyn.NewJob(deployStage, "deploy").
		Needs("gen-version").
		Script("echo deploying " + gen.OutputRef("VERSION"))

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	for _, want := range []string{
		"dotenv:",
		"build.env",
		"$VERSION",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Should NOT contain the pisyn placeholder
	if strings.Contains(out, "PISYN_OUTPUT") {
		t.Errorf("untranslated PISYN_OUTPUT in output:\n%s", out)
	}
}

func TestSynthGitLabOnPushTag(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "Release").OnPushTag("v*")
	s := pisyn.NewStage(p, "release")
	pisyn.NewJob(s, "goreleaser").Image("golang:1.26").Script("echo release")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	if !strings.Contains(out, `$CI_COMMIT_TAG =~ /^v.*/`) {
		t.Errorf("missing tag workflow rule in output:\n%s", out)
	}
}

func TestSynthGitLabMultilineScript(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "multi").
		Script("echo single line").
		Script("line1\nline2\nline3").
		BeforeScript("before1\nbefore2").
		AfterScript("after1\nafter2")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	// Multi-line scripts should use block scalar style
	if !strings.Contains(out, "- |-") && !strings.Contains(out, "- |") {
		t.Errorf("expected block scalar for multi-line script:\n%s", out)
	}
	// Single-line should remain as plain scalar
	if !strings.Contains(out, "- echo single line") {
		t.Errorf("missing single-line script:\n%s", out)
	}
	// Multi-line content should be present
	for _, want := range []string{"line1", "line2", "line3", "before1", "before2", "after1", "after2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitLabEmojiScript(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "emoji").
		Script("echo 🐞 start\necho 🚨 end")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	// Emoji should appear as actual characters, not \U escapes
	if strings.Contains(out, `\U0001F41E`) || strings.Contains(out, `\U0001F6A8`) {
		t.Errorf("emoji rendered as escape sequences:\n%s", out)
	}
	if !strings.Contains(out, "🐞") {
		t.Errorf("missing 🐞 in output:\n%s", out)
	}
	if !strings.Contains(out, "🚨") {
		t.Errorf("missing 🚨 in output:\n%s", out)
	}
	// Should be block scalar, not double-quoted
	if strings.Contains(out, `"echo`) {
		t.Errorf("multi-line emoji script should not be double-quoted:\n%s", out)
	}
}

func TestSynthGitLabMixedEmojiAndPlainScripts(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "mixed").
		Script("echo plain").
		Script("echo 🐞 bug\necho fixed").
		Script("echo also plain")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".gitlab-ci.yml"))
	out := string(b)

	if !strings.Contains(out, "- echo plain") {
		t.Errorf("plain script missing:\n%s", out)
	}
	if !strings.Contains(out, "- echo also plain") {
		t.Errorf("second plain script missing:\n%s", out)
	}
	if !strings.Contains(out, "🐞") {
		t.Errorf("emoji missing:\n%s", out)
	}
	if strings.Contains(out, `\U`) {
		t.Errorf("Unicode escapes should not appear:\n%s", out)
	}
}
