package github_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth/github"
)

func TestSynthGitHub(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI").
		OnPush("main").
		OnPR("main")

	test := pisyn.NewStage(p, "test")
	pisyn.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test ./...").
		SetCache(pisyn.Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
		SetArtifacts(pisyn.Artifacts{Paths: []string{"coverage.out"}}).
		Timeout(15)

	deploy := pisyn.NewStage(p, "deploy")
	pisyn.NewJob(deploy, "deploy-prod").
		Needs("unit-tests").
		Script("echo deploy").
		SetEnvironment("production", "https://app.example.com").
		AllowFailure()

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	for _, want := range []string{
		"name: CI",
		"push:",
		"pull_request:",
		"workflow_dispatch:",
		"runs-on: ubuntu-latest",
		"container:",
		"image: golang:1.26",
		"actions/checkout@v5",
		"actions/cache@v4",
		"go-mod",
		"go test ./...",
		"actions/upload-artifact@v4",
		"if: always()",
		"timeout-minutes: 15",
		"deploy-prod:",
		"needs:",
		"- unit-tests",
		"environment:",
		"name: production",
		"continue-on-error: true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitHubServices(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "integration").
		Image("golang:1.26").
		AddServiceWithVars("postgres:16", "db", map[string]string{"POSTGRES_DB": "test"}).
		Script("go test ./...")

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	for _, want := range []string{
		"services:",
		"db:",
		"image: postgres:16",
		"POSTGRES_DB: test",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSynthGitHubEntrypoint(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "lint").
		Image("ruff:latest").
		ImageEntrypoint("").
		Script("ruff check .")

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	if !strings.Contains(out, `--entrypoint ""`) {
		t.Errorf("missing entrypoint quoting in output:\n%s", out)
	}
}

func TestSynthGitHubConsolidatedScripts(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "build").
		BeforeScript("cd src").
		Script("go build ./...", "go test ./...").
		AfterScript("echo done")

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	// before_script is a separate step, script actions follow
	if !strings.Contains(out, "cd src") || !strings.Contains(out, "go build") {
		t.Errorf("before_script and script not in output:\n%s", out)
	}
	// Verify before_script is its own step (not consolidated into script)
	if !strings.Contains(out, "Before script") {
		t.Errorf("before_script should be a separate step:\n%s", out)
	}
	// after_script should be separate with if: always()
	if !strings.Contains(out, "echo done") {
		t.Errorf("missing after_script:\n%s", out)
	}
}

func TestSynthGitHubVariableTranslation(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "deploy").
		If(pisyn.VarCommitBranch+` == "main"`).
		Env("BRANCH", pisyn.VarCommitBranch).
		Script(`echo "deploying ` + pisyn.VarProjectPath + ` at ` + pisyn.VarCommitSHA + `"`)

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	// Condition should be translated
	if !strings.Contains(out, "github.ref_name") {
		t.Errorf("condition not translated:\n%s", out)
	}
	if strings.Contains(out, "PISYN_COMMIT_BRANCH") {
		t.Errorf("pisyn variable not replaced in condition:\n%s", out)
	}
	// Script should be translated
	if !strings.Contains(out, "github.repository") {
		t.Errorf("PISYN_PROJECT_PATH not translated in script:\n%s", out)
	}
	if !strings.Contains(out, "github.sha") {
		t.Errorf("PISYN_COMMIT_SHA not translated in script:\n%s", out)
	}
}

func TestSynthGitHubStepConstruct(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "build").
		AddStep(pisyn.Step{Uses: "actions/setup-node@v4", With: map[string]string{"node-version": "20"}}).
		Script("npm install", "npm run build").
		AddStep(pisyn.Step{Uses: "actions/deploy-pages@v4", If: "github.ref == 'refs/heads/main'"})

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	for _, want := range []string{
		"actions/checkout@v5",
		"actions/setup-node@v4",
		"node-version:",
		"npm install",
		"npm run build",
		"actions/deploy-pages@v4",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}

	// Verify ordering: setup-node before npm, deploy-pages after
	setupIdx := strings.Index(out, "setup-node")
	npmIdx := strings.Index(out, "npm install")
	deployIdx := strings.Index(out, "deploy-pages")
	if setupIdx > npmIdx || npmIdx > deployIdx {
		t.Errorf("steps not in correct order:\n%s", out)
	}
}

func TestSynthGitHubOutputs(t *testing.T) {
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

	if err := app.Synth(github.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	out := string(b)

	for _, want := range []string{
		"outputs:",
		"needs.gen-version.outputs.version",
		"cat build.env >> $GITHUB_OUTPUT",
		"id: outputs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}
