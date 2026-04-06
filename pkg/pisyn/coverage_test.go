package pisyn

import "testing"

func TestGraph(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s1 := NewStage(p, "build")
	NewJob(s1, "compile").Image("alpine")
	s2 := NewStage(p, "test")
	NewJob(s2, "unit").Image("alpine").Needs("compile")

	g := app.Graph()
	if g == "" {
		t.Fatal("graph is empty")
	}
	if !contains(g, "compile") || !contains(g, "unit") {
		t.Errorf("graph missing jobs: %s", g)
	}
	if !contains(g, "-->") {
		t.Error("graph missing edges")
	}
}

func TestIsAllowedToFail(t *testing.T) {
	j := JobTemplate("t")
	if j.IsAllowedToFail() {
		t.Error("should not be allowed to fail by default")
	}
	j.AllowFailure()
	if !j.IsAllowedToFail() {
		t.Error("should be allowed to fail after AllowFailure()")
	}

	j2 := JobTemplate("t2")
	j2.AllowFailureOnExitCodes(1, 2)
	if !j2.IsAllowedToFail() {
		t.Error("should be allowed to fail with exit codes")
	}
}

func TestOutputRef(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "build")
	j := NewJob(s, "gen-version").Output("VERSION", "version.env")

	ref := j.OutputRef("VERSION")
	if ref != "$PISYN_OUTPUT_GEN_VERSION__VERSION" {
		t.Errorf("OutputRef = %q", ref)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")

	tmpl := JobTemplate("base").
		Image("golang:1.26").
		Env("A", "1").
		AddService("postgres:16", "db").
		SetArtifacts(Artifacts{Paths: []string{"bin/"}}).
		SetCache(Cache{Key: "k", Paths: []string{"/cache"}}).
		SetMatrix(map[string][]string{"go": {"1.21"}}).
		SetEnvironment("prod", "https://example.com").
		AddTag("docker").
		AddRule(Rule{If: "$CI"}).
		Output("V", "v.env")

	c := tmpl.Clone(s, "cloned")

	// Modify clone — original should be unaffected
	c.Env("B", "2")
	c.ArtifactsCfg.Paths = append(c.ArtifactsCfg.Paths, "lib/")
	c.CacheCfg.Paths = append(c.CacheCfg.Paths, "/extra")
	c.Tags = append(c.Tags, "extra")

	if tmpl.EnvVars["B"] == "2" {
		t.Error("clone env leaked to template")
	}
	if len(tmpl.ArtifactsCfg.Paths) != 1 {
		t.Error("clone artifacts leaked to template")
	}
	if len(tmpl.CacheCfg.Paths) != 1 {
		t.Error("clone cache leaked to template")
	}
	if len(tmpl.Tags) != 1 {
		t.Error("clone tags leaked to template")
	}
}

func TestIfAddsRule(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	j := NewJob(s, "deploy").If(`$PISYN_COMMIT_BRANCH == "main"`)

	if len(j.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(j.Rules))
	}
	if j.Rules[0].If != `$PISYN_COMMIT_BRANCH == "main"` {
		t.Errorf("rule if = %q", j.Rules[0].If)
	}
}

func TestSetDefault(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI").SetDefault(JobDefaults{
		Image: "golang:1.26",
		Tags:  []string{"docker"},
	})
	if p.Defaults == nil {
		t.Fatal("defaults nil")
	}
	if p.Defaults.Image != "golang:1.26" {
		t.Errorf("default image = %q", p.Defaults.Image)
	}
}

func TestAddServiceWithVars(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	j := NewJob(s, "test").AddServiceWithVars("postgres:16", "db", map[string]string{"POSTGRES_DB": "test"})

	if len(j.ServiceList) != 1 {
		t.Fatal("expected 1 service")
	}
	if j.ServiceList[0].Variables["POSTGRES_DB"] != "test" {
		t.Error("service vars missing")
	}
}

func TestSetInterruptible(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	j := NewJob(s, "test").SetInterruptible(true)

	if j.Interruptible == nil || !*j.Interruptible {
		t.Error("interruptible should be true")
	}
}

func TestOnMR(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI").OnMR("main")
	if p.On.PullRequest == nil || p.On.PullRequest.Branches[0] != "main" {
		t.Error("OnMR should set PullRequest trigger")
	}
}

func TestIncludeTypes(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI").
		IncludeRemote("https://example.com/ci.yml").
		IncludeProject("shared/templates", "ci/lint.yml", "main").
		IncludeTemplate("Auto-DevOps.gitlab-ci.yml")

	if len(p.IncludeList) != 3 {
		t.Fatalf("expected 3 includes, got %d", len(p.IncludeList))
	}
	if p.IncludeList[0].Remote != "https://example.com/ci.yml" {
		t.Error("remote include wrong")
	}
	if p.IncludeList[1].Project != "shared/templates" {
		t.Error("project include wrong")
	}
	if p.IncludeList[2].Template != "Auto-DevOps.gitlab-ci.yml" {
		t.Error("template include wrong")
	}
}

func TestConstructAccessors(t *testing.T) {
	app := NewApp()
	p := NewPipeline(app, "CI")
	s := NewStage(p, "test")
	NewJob(s, "unit")

	if app.Construct.ID() != "App" {
		t.Errorf("app id = %q", app.Construct.ID())
	}
	if len(app.Construct.Children()) != 1 {
		t.Error("app should have 1 child")
	}
	if app.Construct.Node() != app {
		t.Error("app node should be self")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
