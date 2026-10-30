package runner

import (
	"testing"

	"github.com/pipecrew/pisyn/pkg/pisyn"
)

func newTestApp(fn func(*pisyn.App)) *pisyn.App {
	app := pisyn.NewApp()
	fn(app)
	return app
}

func TestPlan_SingleStage(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine")
		pisyn.NewJob(s, "b").Image("alpine")
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(plan.Groups))
	}
	if len(plan.Groups[0].Jobs) != 2 {
		t.Fatalf("expected 2 jobs in group, got %d", len(plan.Groups[0].Jobs))
	}
}

func TestPlan_MultiStage_ImplicitDeps(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s1 := pisyn.NewStage(p, "build")
		pisyn.NewJob(s1, "compile").Image("alpine")
		s2 := pisyn.NewStage(p, "test")
		pisyn.NewJob(s2, "unit").Image("alpine")
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(plan.Groups))
	}
	if plan.Groups[0].Jobs[0].JobName != "compile" {
		t.Fatalf("expected compile first, got %s", plan.Groups[0].Jobs[0].JobName)
	}
	// unit depends on compile implicitly
	if len(plan.Deps["unit"]) != 1 || plan.Deps["unit"][0] != "compile" {
		t.Fatalf("expected unit to depend on compile, got %v", plan.Deps["unit"])
	}
}

func TestPlan_ExplicitNeeds(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s1 := pisyn.NewStage(p, "build")
		pisyn.NewJob(s1, "a").Image("alpine")
		pisyn.NewJob(s1, "b").Image("alpine")
		s2 := pisyn.NewStage(p, "test")
		pisyn.NewJob(s2, "c").Image("alpine").Needs("a") // only needs a, not b
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	deps := plan.Deps["c"]
	if len(deps) != 1 || deps[0] != "a" {
		t.Fatalf("expected c to depend only on a, got %v", deps)
	}
}

func TestPlan_EmptyNeeds(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s1 := pisyn.NewStage(p, "build")
		pisyn.NewJob(s1, "a").Image("alpine")
		s2 := pisyn.NewStage(p, "test")
		pisyn.NewJob(s2, "b").Image("alpine").EmptyNeedsList()
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// b has empty needs — should be in same group as a (no deps)
	if len(plan.Groups) != 1 {
		t.Fatalf("expected 1 group (both parallelizable), got %d", len(plan.Groups))
	}
	if len(plan.Deps["b"]) != 0 {
		t.Fatalf("expected no deps for b, got %v", plan.Deps["b"])
	}
}

func TestPlan_FilterByJob(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine")
		pisyn.NewJob(s, "b").Image("alpine")
	})

	plan, err := Plan(app, RunOpts{Job: "b"})
	if err != nil {
		t.Fatal(err)
	}
	jobs := plan.AllJobs()
	if len(jobs) != 1 || jobs[0].JobName != "b" {
		t.Fatalf("expected only job b, got %v", jobs)
	}
}

func TestPlan_FilterByStage(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s1 := pisyn.NewStage(p, "build")
		pisyn.NewJob(s1, "compile").Image("alpine")
		s2 := pisyn.NewStage(p, "test")
		pisyn.NewJob(s2, "unit").Image("alpine")
	})

	plan, err := Plan(app, RunOpts{Stage: "test"})
	if err != nil {
		t.Fatal(err)
	}
	jobs := plan.AllJobs()
	if len(jobs) != 1 || jobs[0].JobName != "unit" {
		t.Fatalf("expected only unit, got %v", jobs)
	}
}

func TestPlan_JobNotFound(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine")
	})

	_, err := Plan(app, RunOpts{Job: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestPlan_WarnsOnMissingNeedsDep(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine").Needs("nonexistent")
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(plan.Warnings), plan.Warnings)
	}
}

func TestPlan_CircularDependency(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine").Needs("b")
		pisyn.NewJob(s, "b").Image("alpine").Needs("a")
	})

	_, err := Plan(app, RunOpts{})
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestPlan_OptionalNeedSkippedSilently(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine").
			Need(pisyn.NeedEntry{Job: "nonexistent", Optional: true})
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 0 {
		t.Errorf("optional missing need should not warn, got: %v", plan.Warnings)
	}
}

func TestPlan_RequiredMissingNeedWarns(t *testing.T) {
	app := newTestApp(func(app *pisyn.App) {
		p := pisyn.NewPipeline(app, "CI")
		s := pisyn.NewStage(p, "test")
		pisyn.NewJob(s, "a").Image("alpine").
			Need(pisyn.NeedEntry{Job: "nonexistent", Optional: false})
	})

	plan, err := Plan(app, RunOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 1 {
		t.Fatalf("expected 1 warning for required missing need, got %d: %v", len(plan.Warnings), plan.Warnings)
	}
}
