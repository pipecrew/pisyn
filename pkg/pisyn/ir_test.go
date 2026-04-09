package pisyn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIR_RoundTrip(t *testing.T) {
	// Build a construct tree with all features
	app := NewApp()
	p := NewPipeline(app, "CI").
		SetEnv("GO_VERSION", "1.26").
		OnPush("main").
		OnPR("main").
		OnSchedule("0 2 * * *").
		AddWorkflowRule(Rule{If: "$CI_MERGE_REQUEST_ID"}).
		IncludeLocal("/templates/base.yml")

	lint := NewStage(p, "lint")
	inter := true
	NewJob(lint, "golint").
		Image("golangci/golangci-lint:v2").
		ImageEntrypoint("").
		ImageUser("linter").
		BeforeScript("echo before").
		Script("golangci-lint run").
		AfterScript("echo after").
		AddStep(Step{Uses: "actions/setup-go@v5", With: map[string]string{"go-version": "1.26"}}).
		Needs("other").
		EmptyNeedsList().
		Dependencies("other").
		RunsOn("ubuntu-latest").
		Env("CGO_ENABLED", "0").
		AddService("postgres:16", "db").
		SetArtifacts(Artifacts{Paths: []string{"bin/"}, ExpireIn: "7 days", Reports: map[string][]string{"junit": {"report.xml"}}}).
		SetCache(Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
		SetMatrix(map[string][]string{"go": {"1.21", "1.26"}}).
		SetEnvironment("production", "https://app.example.com").
		AllowFailure().
		Timeout(30).
		Retry(2).
		SetRetry(RetryConfig{Max: 2, When: []string{"runner_system_failure"}}).
		SetWhen(Manual).
		AddTag("docker").
		AddRule(Rule{If: "$CI_COMMIT_BRANCH == 'main'", When: "manual", Changes: []string{"src/**"}}).
		Output("VERSION", "version.env")
	// Set interruptible via pointer
	for _, j := range lint.Jobs() {
		j.Interruptible = &inter
	}

	// Build → JSON
	ir := app.ToIR()
	data, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	// Load from JSON
	var loaded IRApp
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	// Convert back to construct tree
	app2 := loaded.ToApp()

	// Verify pipeline
	pipelines := app2.Pipelines()
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	p2 := pipelines[0]
	if p2.Name != "CI" {
		t.Errorf("pipeline name = %q, want CI", p2.Name)
	}
	if p2.Env["GO_VERSION"] != "1.26" {
		t.Errorf("pipeline env GO_VERSION = %q", p2.Env["GO_VERSION"])
	}
	if p2.On.Push == nil || len(p2.On.Push.Branches) != 1 {
		t.Error("push trigger missing")
	}
	if len(p2.WorkflowRules) != 1 {
		t.Errorf("expected 1 workflow rule, got %d", len(p2.WorkflowRules))
	}
	if len(p2.IncludeList) != 1 || p2.IncludeList[0].Local != "/templates/base.yml" {
		t.Error("include missing")
	}

	// Verify stage
	stages := p2.Stages()
	if len(stages) != 1 || stages[0].Name != "lint" {
		t.Fatalf("expected stage 'lint', got %v", stages)
	}

	// Verify job
	jobs := stages[0].Jobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]

	checks := []struct {
		name string
		ok   bool
	}{
		{"JobName", j.JobName == "golint"},
		{"Image", j.ImageName == "golangci/golangci-lint:v2"},
		{"ImageEP", len(j.ImageEP) == 1 && j.ImageEP[0] == ""},
		{"ImageUser", j.ImageUsr == "linter"},
		{"Actions", len(j.Actions) == 4}, // before + main + after + step
		{"EmptyNeeds", j.EmptyNeeds},
		{"Runner", j.Runner == "ubuntu-latest"},
		{"Env", j.EnvVars["CGO_ENABLED"] == "0"},
		{"Services", len(j.ServiceList) == 1 && j.ServiceList[0].Alias == "db"},
		{"Artifacts", j.ArtifactsCfg != nil && j.ArtifactsCfg.ExpireIn == "7 days"},
		{"Cache", j.CacheCfg != nil && j.CacheCfg.Key == "go-mod"},
		{"Matrix", j.MatrixCfg != nil},
		{"Environment", j.EnvironmentCfg != nil && j.EnvironmentCfg.Name == "production"},
		{"AllowFailure", j.IsAllowFailure},
		{"Timeout", j.TimeoutMin == 30},
		{"RetryConfig", j.RetryCfg != nil && j.RetryCfg.Max == 2},
		{"When", j.When == Manual},
		{"Tags", len(j.Tags) == 1 && j.Tags[0] == "docker"},
		{"Rules", len(j.Rules) == 1},
		{"Interruptible", j.Interruptible != nil && *j.Interruptible},
		{"Outputs", len(j.OutputList) == 1 && j.OutputList[0].Name == "VERSION"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("round-trip failed for %s", c.name)
		}
	}
}

func TestBuild_WritesFile(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	NewJob(s, "unit").Image("alpine").Script("echo hello")

	dir := t.TempDir()
	if err := app.Build(dir); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "pipeline.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var ir IRApp
	if err := json.Unmarshal(data, &ir); err != nil {
		t.Fatal(err)
	}
	if len(ir.Pipelines) != 1 || ir.Pipelines[0].Name != "CI" {
		t.Errorf("unexpected pipeline: %+v", ir.Pipelines)
	}
}

func TestLoadIR_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.json")
	data := `{"pipelines":[{"name":"test","stages":[{"name":"s","jobs":[{"name":"j","image":"alpine"}]}]}]}`
	os.WriteFile(path, []byte(data), 0o644)

	ir, err := LoadIR(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Pipelines) != 1 || ir.Pipelines[0].Stages[0].Jobs[0].Name != "j" {
		t.Error("load failed")
	}

	app := ir.ToApp()
	jobs := app.Pipelines()[0].Stages()[0].Jobs()
	if len(jobs) != 1 || jobs[0].ImageName != "alpine" {
		t.Error("ToApp failed")
	}
}

func TestIR_EmptyApp(t *testing.T) {
	app := NewApp()
	ir := app.ToIR()
	if len(ir.Pipelines) != 0 {
		t.Errorf("expected 0 pipelines, got %d", len(ir.Pipelines))
	}

	data, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}
	var loaded IRApp
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	app2 := loaded.ToApp()
	if len(app2.Pipelines()) != 0 {
		t.Error("expected 0 pipelines after round-trip")
	}
}

func TestIR_MultiPipelineMultiStage(t *testing.T) {
	app := NewApp()
	p1 := NewPipeline(app, "CI")
	s1 := NewStage(p1, "build")
	NewJob(s1, "compile").Image("golang:1.26")
	s2 := NewStage(p1, "test")
	NewJob(s2, "unit").Image("golang:1.26")
	NewJob(s2, "lint").Image("golangci/golangci-lint:v2")

	p2 := NewPipeline(app, "Deploy")
	s3 := NewStage(p2, "deploy")
	NewJob(s3, "push").Image("alpine")

	ir := app.ToIR()
	if len(ir.Pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(ir.Pipelines))
	}
	if len(ir.Pipelines[0].Stages) != 2 {
		t.Fatalf("expected 2 stages in CI, got %d", len(ir.Pipelines[0].Stages))
	}
	if len(ir.Pipelines[0].Stages[1].Jobs) != 2 {
		t.Fatalf("expected 2 jobs in test stage, got %d", len(ir.Pipelines[0].Stages[1].Jobs))
	}

	// Round-trip
	data, _ := json.Marshal(ir)
	var loaded IRApp
	json.Unmarshal(data, &loaded)
	app2 := loaded.ToApp()

	if len(app2.Pipelines()) != 2 {
		t.Fatal("round-trip lost pipelines")
	}
	if len(app2.Pipelines()[0].Stages()[1].Jobs()) != 2 {
		t.Fatal("round-trip lost jobs")
	}
}

func TestIR_NilFieldsHandled(t *testing.T) {
	// Job with all nil optional fields
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	NewJob(s, "minimal")

	ir := app.ToIR()
	data, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}

	var loaded IRApp
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	app2 := loaded.ToApp()
	j := app2.Pipelines()[0].Stages()[0].Jobs()[0]

	// EnvVars should be initialized (not nil)
	if j.EnvVars == nil {
		t.Error("EnvVars should be initialized to empty map, not nil")
	}
	// Optional fields should be nil
	if j.ArtifactsCfg != nil || j.CacheCfg != nil || j.MatrixCfg != nil {
		t.Error("optional configs should be nil")
	}
	if j.Interruptible != nil {
		t.Error("interruptible should be nil")
	}
}

func TestIR_OmitsEmptyJSON(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	NewJob(s, "simple").Image("alpine").Script("echo hi")

	ir := app.ToIR()
	data, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}

	str := string(data)
	// These empty fields should be omitted
	for _, field := range []string{"tags", "rules", "matrix", "cache", "artifacts", "retry_config"} {
		if strings.Contains(str, `"`+field+`"`) {
			t.Errorf("expected %q to be omitted from JSON", field)
		}
	}
	// These should be present
	for _, field := range []string{"name", "image", "actions"} {
		if !strings.Contains(str, `"`+field+`"`) {
			t.Errorf("expected %q to be present in JSON", field)
		}
	}
}

func TestIR_ScriptLinesPreserved(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	NewJob(s, "j").
		BeforeScript("echo before").
		Script("echo main1", "echo main2").
		AfterScript("echo after").
		AddStep(Step{Uses: "actions/checkout@v4"})

	ir := app.ToIR()
	data, _ := json.Marshal(ir)
	var loaded IRApp
	json.Unmarshal(data, &loaded)
	app2 := loaded.ToApp()

	j := app2.Pipelines()[0].Stages()[0].Jobs()[0]
	before := j.BeforeScriptLines()
	main := j.ScriptLines()
	after := j.AfterScriptLines()

	if len(before) != 1 || before[0] != "echo before" {
		t.Errorf("before scripts: %v", before)
	}
	if len(main) != 1 || main[0] != "echo main1\necho main2" {
		t.Errorf("main scripts: %v", main)
	}
	if len(after) != 1 || after[0] != "echo after" {
		t.Errorf("after scripts: %v", after)
	}

	// Step should survive
	stepCount := 0
	for _, a := range j.Actions {
		if a.Step != nil {
			stepCount++
		}
	}
	if stepCount != 1 {
		t.Errorf("expected 1 step, got %d", stepCount)
	}
}

func TestIR_TriggersPreserved(t *testing.T) {
	app := NewApp()
	NewPipeline(app, "CI").
		OnPush("main", "develop").
		OnPushProtected().
		OnPR("main").
		OnSchedule("0 2 * * *")

	ir := app.ToIR()
	data, _ := json.Marshal(ir)
	var loaded IRApp
	json.Unmarshal(data, &loaded)
	app2 := loaded.ToApp()

	p := app2.Pipelines()[0]
	if p.On.Push == nil || len(p.On.Push.Branches) != 2 {
		t.Error("push branches lost")
	}
	if !p.On.Push.Protected {
		t.Error("push protected lost")
	}
	if p.On.PullRequest == nil || p.On.PullRequest.Branches[0] != "main" {
		t.Error("PR trigger lost")
	}
	if len(p.On.Schedule) != 1 || p.On.Schedule[0].Cron != "0 2 * * *" {
		t.Error("schedule trigger lost")
	}
}

func TestIR_OnPushTagPreserved(t *testing.T) {
	app := NewApp()
	NewPipeline(app, "Release").
		OnPushTag("v*")

	ir := app.ToIR()
	data, _ := json.Marshal(ir)
	var loaded IRApp
	json.Unmarshal(data, &loaded)
	app2 := loaded.ToApp()

	p := app2.Pipelines()[0]
	if p.On.Push == nil || len(p.On.Push.Tags) != 1 || p.On.Push.Tags[0] != "v*" {
		t.Error("push tags lost in IR roundtrip")
	}
}

func TestLoadIR_FileNotFound(t *testing.T) {
	_, err := LoadIR("/nonexistent/pipeline.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadIR_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	_, err := LoadIR(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
