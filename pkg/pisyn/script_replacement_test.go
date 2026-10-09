package pisyn

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestScriptReplacementPreservesSteps(t *testing.T) {
	for _, tc := range []struct {
		name    string
		phase   ActionPhase
		replace func(*Job, ...string) *Job
	}{
		{"main", PhaseMain, (*Job).SetScript},
		{"before", PhaseBefore, (*Job).BeforeScript},
		{"after", PhaseAfter, (*Job).AfterScript},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp()
			stage := NewStage(NewPipeline(app, "CI"), "test")
			job := NewJob(stage, "unit")
			old, untouched := "echo old", "echo untouched"
			step := &Step{Uses: "example/action@v1", With: map[string]string{"message": "café 日本語"}}
			job.Actions = []Action{
				{Script: &old, Phase: tc.phase},
				{Step: step, Phase: tc.phase},
				{Script: &untouched, Phase: ActionPhase((int(tc.phase) + 1) % 3)},
			}
			if got := tc.replace(job, "echo new", "echo done"); got != job {
				t.Fatal("replacement must remain chainable")
			}
			assertActions := func(t *testing.T, actions []Action) {
				t.Helper()
				var steps []*Step
				var replaced []string
				var other []string
				for _, action := range actions {
					if action.Step != nil {
						steps = append(steps, action.Step)
						if action.Phase != tc.phase {
							t.Errorf("step phase changed: %v", action.Phase)
						}
					}
					if action.Script != nil {
						if action.Phase == tc.phase {
							replaced = append(replaced, *action.Script)
						} else {
							other = append(other, *action.Script)
						}
					}
				}
				if !reflect.DeepEqual(steps, []*Step{step}) {
					t.Errorf("steps = %#v; want retained action step", steps)
				}
				if !reflect.DeepEqual(replaced, []string{"echo new\necho done"}) {
					t.Errorf("replacement scripts = %q", replaced)
				}
				if !reflect.DeepEqual(other, []string{untouched}) {
					t.Errorf("other phase scripts = %q", other)
				}
			}
			assertActions(t, job.Actions)
			// Exercise the public disk round-trip, not only the in-memory builder.
			dir := t.TempDir()
			if err := app.Build(dir); err != nil {
				t.Fatal(err)
			}
			ir, err := LoadIR(filepath.Join(dir, "pipeline.json"))
			if err != nil {
				t.Fatal(err)
			}
			restored := ir.ToApp().Pipelines()[0].Stages()[0].Jobs()[0]
			assertActions(t, restored.Actions)
		})
	}
}

func TestSetScriptPreservesStepOnlyJob(t *testing.T) {
	job := JobTemplate("steps").AddStep(Step{Uses: "actions/checkout@v4"})
	job.SetScript("go test ./...").SetScript("go test -race ./...")
	if len(job.Actions) != 2 || job.Actions[0].Step == nil {
		t.Fatalf("repeated replacement lost the action step: %+v", job.Actions)
	}
	if got := job.ScriptLines(); !reflect.DeepEqual(got, []string{"go test -race ./..."}) {
		t.Errorf("main scripts = %q", got)
	}
}
