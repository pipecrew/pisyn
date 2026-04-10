package pisyn

import (
	"os"
	"testing"
)

// fakeSynth records that it was called.
type fakeSynth struct{ called bool }

func (f *fakeSynth) Synth(_ *App, _ string) error { f.called = true; return nil }

func clearRegistry() {
	for k := range registry {
		delete(registry, k)
	}
}

func TestRegisterPlatform(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	RegisterPlatform("test", func() Synthesizer { return &fakeSynth{} })
	if _, ok := registry["test"]; !ok {
		t.Fatal("platform not registered")
	}
}

func TestRegisterPlatformLowercase(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	RegisterPlatform("GitLab", func() Synthesizer { return &fakeSynth{} })
	if _, ok := registry["gitlab"]; !ok {
		t.Fatal("platform not lowercased")
	}
}

func TestRunAllRegistered(t *testing.T) {
	clearRegistry()
	defer clearRegistry()
	os.Unsetenv("PISYN_PLATFORM")

	f := &fakeSynth{}
	RegisterPlatform("fake", func() Synthesizer { return f })

	app := NewApp()
	app.OutDir = t.TempDir()
	if err := app.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !f.called {
		t.Fatal("synthesizer was not called")
	}
}

func TestRunWithPlatformEnv(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	called := map[string]bool{}
	RegisterPlatform("a", func() Synthesizer {
		s := &fakeSynth{}
		called["a"] = true
		return s
	})
	RegisterPlatform("b", func() Synthesizer {
		s := &fakeSynth{}
		called["b"] = true
		return s
	})

	os.Setenv("PISYN_PLATFORM", "a")
	defer os.Unsetenv("PISYN_PLATFORM")

	app := NewApp()
	app.OutDir = t.TempDir()
	if err := app.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !called["a"] {
		t.Fatal("platform 'a' was not called")
	}
	if called["b"] {
		t.Fatal("platform 'b' should not have been called")
	}
}

func TestRunUnknownPlatform(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	os.Setenv("PISYN_PLATFORM", "nonexistent")
	defer os.Unsetenv("PISYN_PLATFORM")

	app := NewApp()
	if err := app.Run(); err == nil {
		t.Fatal("expected error for unknown platform")
	}
}

func TestRunNoRegisteredPlatforms(t *testing.T) {
	clearRegistry()
	defer clearRegistry()
	os.Unsetenv("PISYN_PLATFORM")

	app := NewApp()
	if err := app.Run(); err == nil {
		t.Fatal("expected error when no platforms registered")
	}
}

func TestSynthReturnsErrorForDuplicateJobNames(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st1 := NewStage(p, "build")
	st2 := NewStage(p, "test")

	NewJob(st1, "compile")
	dup := NewJob(st2, "unit")
	dup.JobName = "compile"

	s := &fakeSynth{}
	err := app.Synth(s)
	if err == nil {
		t.Fatal("expected duplicate job name error")
	}
	if err.Error() != `pisyn: duplicate job name "compile" in pipeline "CI"` {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.called {
		t.Fatal("synthesizer should not be called when validation fails")
	}
}

func TestNewAppOutDirEnv(t *testing.T) {
	os.Setenv("PISYN_OUT_DIR", "/custom/out")
	defer os.Unsetenv("PISYN_OUT_DIR")

	app := NewApp()
	if app.OutDir != "/custom/out" {
		t.Fatalf("expected /custom/out, got %s", app.OutDir)
	}
}

func TestNewAppOutDirDefault(t *testing.T) {
	os.Unsetenv("PISYN_OUT_DIR")

	app := NewApp()
	if app.OutDir != "pisyn.out" {
		t.Fatalf("expected pisyn.out, got %s", app.OutDir)
	}
}

func TestPlatformsFromEnvMultiple(t *testing.T) {
	os.Setenv("PISYN_PLATFORM", "gitlab, github , tekton")
	defer os.Unsetenv("PISYN_PLATFORM")

	got := platformsFromEnv()
	if len(got) != 3 || got[0] != "gitlab" || got[1] != "github" || got[2] != "tekton" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestScriptAppends(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")
	j := NewJob(st, "build").
		Script("echo first").
		Script("echo second")

	lines := j.ScriptLines()
	if len(lines) != 2 || lines[0] != "echo first" || lines[1] != "echo second" {
		t.Fatalf("expected 2 script blocks, got %v", lines)
	}
}

func TestPrependScript(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")
	j := NewJob(st, "build").
		BeforeScript("cd src").
		Script("echo main").
		PrependScript("echo prepended")

	lines := j.ScriptLines()
	if len(lines) != 2 || lines[0] != "echo prepended" || lines[1] != "echo main" {
		t.Fatalf("expected prepended before main, got %v", lines)
	}
	bs := j.BeforeScriptLines()
	if len(bs) != 1 || bs[0] != "cd src" {
		t.Fatalf("before_script should be preserved, got %v", bs)
	}
}

func TestSetScript(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")
	j := NewJob(st, "build").
		BeforeScript("cd src").
		Script("echo old1").
		Script("echo old2").
		AfterScript("echo cleanup").
		SetScript("echo new")

	lines := j.ScriptLines()
	if len(lines) != 1 || lines[0] != "echo new" {
		t.Fatalf("expected replaced script, got %v", lines)
	}
	if len(j.BeforeScriptLines()) != 1 {
		t.Fatal("before_script should be preserved")
	}
	if len(j.AfterScriptLines()) != 1 {
		t.Fatal("after_script should be preserved")
	}
}

func TestClonePreservesPhase(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")

	tmpl := JobTemplate("base").
		BeforeScript("cd src").
		Script("echo main").
		AfterScript("echo cleanup")

	cloned := tmpl.Clone(st, "cloned")

	if len(cloned.BeforeScriptLines()) != 1 || cloned.BeforeScriptLines()[0] != "cd src" {
		t.Fatalf("before_script phase lost in clone: %v", cloned.BeforeScriptLines())
	}
	if len(cloned.AfterScriptLines()) != 1 || cloned.AfterScriptLines()[0] != "echo cleanup" {
		t.Fatalf("after_script phase lost in clone: %v", cloned.AfterScriptLines())
	}
	if len(cloned.ScriptLines()) != 1 || cloned.ScriptLines()[0] != "echo main" {
		t.Fatalf("main script lost in clone: %v", cloned.ScriptLines())
	}
}

func TestOnPushPreservesProtected(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI").
		OnPushProtected().
		OnPush("main")

	if !p.On.Push.Protected {
		t.Fatal("OnPush should preserve Protected flag")
	}
	if len(p.On.Push.Branches) != 1 || p.On.Push.Branches[0] != "main" {
		t.Fatalf("expected branches [main], got %v", p.On.Push.Branches)
	}
}

func TestOnPushTagPreservesBranches(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI").
		OnPush("main").
		OnPushTag("v*")

	if len(p.On.Push.Branches) != 1 || p.On.Push.Branches[0] != "main" {
		t.Fatalf("OnPushTag should preserve branches, got %v", p.On.Push.Branches)
	}
	if len(p.On.Push.Tags) != 1 || p.On.Push.Tags[0] != "v*" {
		t.Fatalf("expected tags [v*], got %v", p.On.Push.Tags)
	}
}

func TestDuplicateJobNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate job name")
		}
	}()

	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")
	NewJob(st, "build")
	NewJob(st, "build") // should panic
}

func TestCloneIsolation(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")

	tmpl := JobTemplate("base").
		Image("golang:1.26").
		Env("KEY", "original").
		Script("echo original")

	clone1 := tmpl.Clone(st, "clone1").Env("KEY", "modified").SetScript("echo modified")
	clone2 := tmpl.Clone(st, "clone2")

	// Modifying clone1 should not affect template or clone2
	if tmpl.EnvVars["KEY"] != "original" {
		t.Fatal("template env was modified by clone")
	}
	if clone2.EnvVars["KEY"] != "original" {
		t.Fatal("clone2 env was modified by clone1")
	}
	if clone1.ScriptLines()[0] != "echo modified" {
		t.Fatal("clone1 script not replaced")
	}
	if clone2.ScriptLines()[0] != "echo original" {
		t.Fatal("clone2 script was modified")
	}
}

func TestCloneDuplicateNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate clone name")
		}
	}()

	app := NewApp()
	p := NewPipeline(app, "CI")
	st := NewStage(p, "test")
	tmpl := JobTemplate("base").Script("echo hi")
	tmpl.Clone(st, "job1")
	tmpl.Clone(st, "job1") // should panic
}
