package gitlab

import (
	"strings"
	"testing"

	"github.com/pipecrew/pisyn/pkg/pisyn"
)

func TestGoIdent(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"build-app", "build_app"},
		{"unit tests", "unit_tests"},
		{".template", "_template"},
		{"a/b", "a_b"},
		{"1-build", "_1_build"},
		{"99bottles", "_99bottles"},
		{"type", "type_"},
		{"func", "func_"},
		{"var", "var_"},
		{"map", "map_"},
		{"range", "range_"},
		{"return", "return_"},
		{"", "_ident"},
		{"normal", "normal"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := goIdent(tt.in)
			if got != tt.want {
				t.Errorf("goIdent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGoMultilineScript_SingleLine(t *testing.T) {
	got := goMultilineScript("echo hello")
	if got != "`echo hello`" {
		t.Errorf("expected backtick string, got: %s", got)
	}
}

func TestGoMultilineScript_SingleLineWithBacktick(t *testing.T) {
	got := goMultilineScript("echo `date`")
	if !strings.HasPrefix(got, `"`) {
		t.Errorf("expected double-quoted string for backtick content, got: %s", got)
	}
}

func TestGoMultilineScript_MultiLine(t *testing.T) {
	got := goMultilineScript("line1\nline2\nline3")
	if got != "`line1\nline2\nline3`" {
		t.Errorf("expected raw string literal, got: %s", got)
	}
}

func TestGoMultilineScript_MultiLineWithBacktick(t *testing.T) {
	got := goMultilineScript("echo `date`\necho done")
	if strings.Contains(got, "`echo") {
		t.Errorf("should not use backtick string when content has backticks, got: %s", got)
	}
	if !strings.HasPrefix(got, `"`) {
		t.Errorf("expected double-quoted string, got: %s", got)
	}
}

func TestGoMultilineScript_WithQuotes(t *testing.T) {
	got := goMultilineScript(`echo "hello world"`)
	// Backtick strings handle double quotes without escaping
	if got != "`echo \"hello world\"`" {
		t.Errorf("expected backtick string with quotes, got: %s", got)
	}
}

func TestGoMultilineScript_WithEmoji(t *testing.T) {
	got := goMultilineScript("echo 🐞\necho 🚨")
	if got != "`echo 🐞\necho 🚨`" {
		t.Errorf("expected raw string with emoji, got: %s", got)
	}
}

func TestGenerateGo_MultilineScript(t *testing.T) {
	r := &ParseResult{Pipeline: &pisyn.IRPipeline{
		Name: "CI",
		Stages: []pisyn.IRStage{{
			Name: "test",
			Jobs: []pisyn.IRJob{{
				Name:  "build",
				Image: "golang:1.26",
				Actions: []pisyn.Action{
					{Script: strPtr("echo hello")},
					{Script: strPtr("line1\nline2")},
				},
			}},
		}},
	}}

	got := GenerateGo(r)

	// Single-line should use backtick
	if !strings.Contains(got, "Script(`echo hello`)") {
		t.Errorf("single-line script should use backtick:\n%s", got)
	}
	// Multi-line should use backtick raw string
	if !strings.Contains(got, "Script(`line1\nline2`)") {
		t.Errorf("multi-line script should use backtick raw string:\n%s", got)
	}
}

func TestGenerateGo_ScriptWithBacktick(t *testing.T) {
	r := &ParseResult{Pipeline: &pisyn.IRPipeline{
		Name: "CI",
		Stages: []pisyn.IRStage{{
			Name: "test",
			Jobs: []pisyn.IRJob{{
				Name: "build",
				Actions: []pisyn.Action{
					{Script: strPtr("echo `date`")},
				},
			}},
		}},
	}}

	got := GenerateGo(r)

	// Should fall back to double-quoted
	if strings.Contains(got, "Script(`echo") {
		t.Errorf("script with backtick should not use raw string:\n%s", got)
	}
}

func TestGenerateGo_BeforeAfterScripts(t *testing.T) {
	before := "setup1\nsetup2"
	after := "cleanup1\ncleanup2"
	r := &ParseResult{Pipeline: &pisyn.IRPipeline{
		Name: "CI",
		Stages: []pisyn.IRStage{{
			Name: "test",
			Jobs: []pisyn.IRJob{{
				Name: "build",
				Actions: []pisyn.Action{
					{Script: &before, Phase: pisyn.PhaseBefore},
					{Script: strPtr("echo main")},
					{Script: &after, Phase: pisyn.PhaseAfter},
				},
			}},
		}},
	}}

	got := GenerateGo(r)

	if !strings.Contains(got, "BeforeScript(`setup1\nsetup2`)") {
		t.Errorf("before_script should use backtick:\n%s", got)
	}
	if !strings.Contains(got, "AfterScript(`cleanup1\ncleanup2`)") {
		t.Errorf("after_script should use backtick:\n%s", got)
	}
}

func strPtr(s string) *string { return &s }

func TestGenerateGo_TemplateFromHiddenJob(t *testing.T) {
	r := &ParseResult{
		Pipeline: &pisyn.IRPipeline{
			Name: "CI",
			Stages: []pisyn.IRStage{{
				Name: "build",
				Jobs: []pisyn.IRJob{{
					Name:  "build-app",
					Image: "golang:1.26",
					Actions: []pisyn.Action{
						{Script: strPtr("go build ./...")},
					},
				}},
			}},
		},
		Templates: []pisyn.IRJob{
			{
				Name:    ".buildJavaTemplate",
				Image:   "maven:3.9",
				Actions: []pisyn.Action{{Script: strPtr("mvn clean package")}},
				Cache:   &pisyn.Cache{Key: "maven", Paths: []string{".m2/repository"}},
			},
		},
	}

	got := GenerateGo(r)

	if !strings.Contains(got, `ps.JobTemplate(".buildJavaTemplate")`) {
		t.Errorf("expected JobTemplate call, got:\n%s", got)
	}
	if !strings.Contains(got, `Image("maven:3.9")`) {
		t.Errorf("expected template image, got:\n%s", got)
	}
	if !strings.Contains(got, `SetCache(ps.Cache{`) {
		t.Errorf("expected template cache, got:\n%s", got)
	}
	// Template should appear before stages
	tmplIdx := strings.Index(got, "JobTemplate")
	stageIdx := strings.Index(got, "NewStage")
	if tmplIdx > stageIdx {
		t.Errorf("template should appear before stages")
	}
}

func TestParse_HiddenJobAsTemplate(t *testing.T) {
	yaml := []byte(`
stages:
  - build
.myTemplate:
  image: alpine:latest
  script:
    - echo template
build-job:
  stage: build
  script:
    - echo build
`)
	r, err := Parse(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(r.Templates))
	}
	tmpl := r.Templates[0]
	if tmpl.Name != ".myTemplate" {
		t.Errorf("expected name .myTemplate, got %q", tmpl.Name)
	}
	if tmpl.Image != "alpine:latest" {
		t.Errorf("expected image alpine:latest, got %q", tmpl.Image)
	}
}

func TestParse_VariableTypes(t *testing.T) {
	yml := []byte(`
stages:
  - build
variables:
  STRING_VAR: "hello"
  INT_VAR: 42
  BOOL_VAR: true
  EXPANDED_VAR:
    value: "expanded_value"
    description: "A variable with description"
build:
  stage: build
  script:
    - echo test
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	env := r.Pipeline.Env
	for _, tc := range []struct{ key, want string }{
		{"STRING_VAR", "hello"},
		{"INT_VAR", "42"},
		{"BOOL_VAR", "true"},
		{"EXPANDED_VAR", "expanded_value"},
	} {
		if got := env[tc.key]; got != tc.want {
			t.Errorf("env[%q] = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestParse_EmptyNeeds(t *testing.T) {
	yml := []byte(`
stages:
  - build
  - test
build:
  stage: build
  script:
    - echo build
test:
  stage: test
  needs: []
  script:
    - echo test
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, stage := range r.Pipeline.Stages {
		for _, job := range stage.Jobs {
			if job.Name == "test" {
				if !job.EmptyNeeds {
					t.Error("expected EmptyNeeds=true for job with needs: []")
				}
				return
			}
		}
	}
	t.Error("test job not found")
}

func TestParse_CacheKeyObjectForm(t *testing.T) {
	yml := []byte(`
stages:
  - test
test-job:
  stage: test
  image: python:3.12
  cache:
    key:
      files:
        - poetry.lock
        - uv.lock
    fallback_keys:
      - py-fallback
    paths:
      - .local
  script:
    - echo test
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, stage := range r.Pipeline.Stages {
		for _, job := range stage.Jobs {
			if job.Name == "test-job" {
				if job.Cache == nil {
					t.Fatal("expected cache to be set")
				}
				if job.Cache.Key != "poetry.lock-uv.lock" {
					t.Errorf("cache key = %q, want %q", job.Cache.Key, "poetry.lock-uv.lock")
				}
				if len(job.Cache.Paths) != 1 || job.Cache.Paths[0] != ".local" {
					t.Errorf("cache paths = %v, want [.local]", job.Cache.Paths)
				}
				return
			}
		}
	}
	t.Error("test-job not found")
}

func TestParse_CacheKeyWithPrefix(t *testing.T) {
	yml := []byte(`
stages:
  - test
test-job:
  stage: test
  cache:
    key:
      files:
        - go.sum
      prefix: go-mod
    paths:
      - /go/pkg/mod
  script:
    - echo test
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, stage := range r.Pipeline.Stages {
		for _, job := range stage.Jobs {
			if job.Name == "test-job" {
				if job.Cache == nil {
					t.Fatal("expected cache to be set")
				}
				if job.Cache.Key != "go-mod-go.sum" {
					t.Errorf("cache key = %q, want %q", job.Cache.Key, "go-mod-go.sum")
				}
				return
			}
		}
	}
	t.Error("test-job not found")
}

func TestParse_EnvironmentActionOnStop(t *testing.T) {
	yml := []byte(`
stages:
  - deploy
deploy-review:
  stage: deploy
  script:
    - echo deploy
  environment:
    name: review/$CI_COMMIT_REF_SLUG
    url: https://review.example.com
    on_stop: stop-review
stop-review:
  stage: deploy
  script:
    - echo stop
  environment:
    name: review/$CI_COMMIT_REF_SLUG
    action: stop
  when: manual
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var deployJob, stopJob *pisyn.IRJob
	for _, stage := range r.Pipeline.Stages {
		for i := range stage.Jobs {
			switch stage.Jobs[i].Name {
			case "deploy-review":
				deployJob = &stage.Jobs[i]
			case "stop-review":
				stopJob = &stage.Jobs[i]
			}
		}
	}
	if deployJob == nil {
		t.Fatal("deploy-review job not found")
	}
	if deployJob.Environment == nil {
		t.Fatal("deploy-review environment should not be nil")
	}
	if deployJob.Environment.OnStop != "stop-review" {
		t.Errorf("deploy-review on_stop = %q, want stop-review", deployJob.Environment.OnStop)
	}
	if deployJob.Environment.URL != "https://review.example.com" {
		t.Errorf("deploy-review url = %q", deployJob.Environment.URL)
	}

	if stopJob == nil {
		t.Fatal("stop-review job not found")
	}
	if stopJob.Environment == nil {
		t.Fatal("stop-review environment should not be nil")
	}
	if stopJob.Environment.Action != "stop" {
		t.Errorf("stop-review action = %q, want stop", stopJob.Environment.Action)
	}
	if stopJob.Environment.Name != "review/$CI_COMMIT_REF_SLUG" {
		t.Errorf("stop-review name = %q", stopJob.Environment.Name)
	}
}

func TestGenerateGo_EnvironmentOnStop(t *testing.T) {
	r := &ParseResult{Pipeline: &pisyn.IRPipeline{
		Name: "CI",
		Stages: []pisyn.IRStage{{
			Name: "deploy",
			Jobs: []pisyn.IRJob{
				{
					Name:  "deploy-review",
					Image: "alpine",
					Environment: &pisyn.Environment{
						Name:   "review",
						URL:    "https://review.example.com",
						OnStop: "stop-review",
					},
					Actions: []pisyn.Action{{Script: strPtr("echo deploy")}},
				},
				{
					Name:  "stop-review",
					Image: "alpine",
					Environment: &pisyn.Environment{
						Name:   "review",
						Action: "stop",
					},
					Actions: []pisyn.Action{{Script: strPtr("echo stop")}},
				},
			},
		}},
	}}

	got := GenerateGo(r)

	if !strings.Contains(got, `SetEnvironmentStop("review")`) {
		t.Errorf("expected SetEnvironmentStop for stop job:\n%s", got)
	}
	if !strings.Contains(got, `SetEnvironmentOpts(`) {
		t.Errorf("expected SetEnvironmentOpts for deploy job:\n%s", got)
	}
	if !strings.Contains(got, `OnStop: "stop-review"`) {
		t.Errorf("expected OnStop field in SetEnvironmentOpts:\n%s", got)
	}
}

func TestParse_CacheKeyStringForm(t *testing.T) {
	yml := []byte(`
stages:
  - test
test-job:
  stage: test
  cache:
    key: my-key
    paths:
      - .cache
  script:
    - echo test
`)
	r, err := Parse(yml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, stage := range r.Pipeline.Stages {
		for _, job := range stage.Jobs {
			if job.Name == "test-job" {
				if job.Cache == nil {
					t.Fatal("expected cache to be set")
				}
				if job.Cache.Key != "my-key" {
					t.Errorf("cache key = %q, want %q", job.Cache.Key, "my-key")
				}
				return
			}
		}
	}
	t.Error("test-job not found")
}
