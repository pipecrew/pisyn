package tekton_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth/tekton"
)

func TestSynthTekton(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")

	test := pisyn.NewStage(p, "test")
	pisyn.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test ./...")

	build := pisyn.NewStage(p, "build")
	pisyn.NewJob(build, "build-binary").
		Image("golang:1.26").
		Needs("unit-tests").
		Script("go build -o app .")

	if err := app.Synth(tekton.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	tektonDir := filepath.Join(dir, "tekton")

	// Check tasks
	b, err := os.ReadFile(filepath.Join(tektonDir, "tasks.yaml"))
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	tasks := string(b)
	for _, want := range []string{
		"apiVersion: tekton.dev/v1",
		"kind: Task",
		"name: unit-tests",
		"image: golang:1.26",
		"go test ./...",
		"name: build-binary",
		"go build -o app .",
	} {
		if !strings.Contains(tasks, want) {
			t.Errorf("tasks missing %q:\n%s", want, tasks)
		}
	}

	// Check pipeline
	b, err = os.ReadFile(filepath.Join(tektonDir, "pipeline.yaml"))
	if err != nil {
		t.Fatalf("read pipeline: %v", err)
	}
	pl := string(b)
	for _, want := range []string{
		"kind: Pipeline",
		"name: ci",
		"taskRef:",
		"name: unit-tests",
		"name: build-binary",
		"runAfter:",
	} {
		if !strings.Contains(pl, want) {
			t.Errorf("pipeline missing %q:\n%s", want, pl)
		}
	}

	// Check pipeline run
	b, err = os.ReadFile(filepath.Join(tektonDir, "pipeline-run.yaml"))
	if err != nil {
		t.Fatalf("read pipeline-run: %v", err)
	}
	run := string(b)
	for _, want := range []string{
		"kind: PipelineRun",
		"generateName: ci-run-",
		"pipelineRef:",
	} {
		if !strings.Contains(run, want) {
			t.Errorf("pipeline-run missing %q:\n%s", want, run)
		}
	}
}

func TestSynthTektonWithBeforeAfterScript(t *testing.T) {
	dir := t.TempDir()
	app := pisyn.NewApp()
	app.OutDir = dir

	p := pisyn.NewPipeline(app, "CI")
	st := pisyn.NewStage(p, "test")
	pisyn.NewJob(st, "build").
		Image("golang:1.26").
		BeforeScript("cd src").
		Script("go build ./...").
		AfterScript("echo done")

	if err := app.Synth(tekton.NewSynthesizer()); err != nil {
		t.Fatalf("synth: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "tekton", "tasks.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(b)

	for _, want := range []string{"cd src", "go build", "echo done"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}
