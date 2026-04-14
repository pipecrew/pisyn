package gitlab

import (
	"fmt"
	"strings"

	"github.com/pipecrew/pisyn/pkg/pisyn"
)

// gitlabToPisyn is the reverse of pisyn.GitLabVars: maps GitLab CI vars to pisyn constants.
var gitlabToPisyn map[string]string

func init() {
	gitlabToPisyn = make(map[string]string, len(pisyn.GitLabVars))
	for pisynKey, glKey := range pisyn.GitLabVars {
		gitlabToPisyn[glKey] = pisynKey
	}
}

// GenerateGo emits a compilable main.go from a parsed pipeline.
func GenerateGo(r *ParseResult) string {
	p := r.Pipeline
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("package main\n\nimport (\n\t\"log\"\n\n\tps \"github.com/pipecrew/pisyn/pkg/pisyn\"\n")
	w("\t_ \"github.com/pipecrew/pisyn/pkg/synth/gitlab\"\n)\n\n")
	w("func main() {\n\tapp := ps.NewApp()\n\tp := ps.NewPipeline(app, %q)", p.Name)

	// Pipeline-level env
	for k, v := range p.Env {
		w(".\n\t\tSetEnv(%q, %q)", k, v)
	}

	// Triggers from workflow rules (best-effort reverse)
	emitTriggersFromWorkflowRules(&b, p.WorkflowRules)

	// Includes
	for _, inc := range p.Includes {
		switch {
		case inc.Local != "":
			w(".\n\t\tIncludeLocal(%q)", inc.Local)
		case inc.Remote != "":
			w(".\n\t\tIncludeRemote(%q)", inc.Remote)
		case inc.Project != "":
			w(".\n\t\tIncludeProject(%q, %q, %q)", inc.Project, inc.File, inc.Ref)
		case inc.Template != "":
			w(".\n\t\tIncludeTemplate(%q)", inc.Template)
		}
	}

	// Defaults
	if p.Defaults != nil {
		d := p.Defaults
		if d.Image != "" || len(d.BeforeScript) > 0 || len(d.AfterScript) > 0 || len(d.Tags) > 0 {
			w(".\n\t\tSetDefault(ps.JobDefaults{")
			if d.Image != "" {
				w("Image: %q, ", d.Image)
			}
			if len(d.BeforeScript) > 0 {
				w("BeforeScript: %s, ", goStringSlice(d.BeforeScript))
			}
			if len(d.AfterScript) > 0 {
				w("AfterScript: %s, ", goStringSlice(d.AfterScript))
			}
			if len(d.Tags) > 0 {
				w("Tags: %s, ", goStringSlice(d.Tags))
			}
			w("})")
		}
	}

	// Workflow rules that couldn't be reverse-mapped to triggers
	if hasNonTriggerWorkflowRules(p.WorkflowRules) {
		for _, r := range p.WorkflowRules {
			if isTriggerRule(r) {
				continue
			}
			w(".\n\t\tAddWorkflowRule(%s)", goRule(r))
		}
	}

	w("\n\n")

	// Templates (hidden jobs starting with .)
	for _, tmpl := range r.Templates {
		emitTemplate(&b, &tmpl)
	}

	// Stages and jobs
	for _, stage := range p.Stages {
		if len(stage.Jobs) == 0 {
			w("\tps.NewStage(p, %q)\n\n", stage.Name)
			continue
		}
		stageVar := goIdent(stage.Name)
		w("\t%s := ps.NewStage(p, %q)\n", stageVar, stage.Name)
		for _, job := range stage.Jobs {
			emitJob(&b, &job, stageVar)
		}
		w("\n")
	}

	w("\tif err := app.Run(); err != nil {\n\t\tlog.Fatal(err)\n\t}\n}\n")
	return b.String()
}

func emitTemplate(b *strings.Builder, job *pisyn.IRJob) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	tmplVar := goIdent(job.Name)
	w("\t%s := ps.JobTemplate(%q)", tmplVar, job.Name)
	emitJobFields(b, job)
	w("\n\n")
}

func emitJob(b *strings.Builder, job *pisyn.IRJob, stageVar string) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }

	// Check if job has outputs that other jobs reference
	hasOutputs := len(job.Outputs) > 0
	jobVar := "_"
	if hasOutputs {
		jobVar = goIdent(job.Name)
	}

	if hasOutputs {
		w("\t%s := ps.NewJob(%s, %q)", jobVar, stageVar, job.Name)
	} else {
		w("\tps.NewJob(%s, %q)", stageVar, job.Name)
	}
	emitJobFields(b, job)
	w("\n")
	if hasOutputs {
		w("\t// Use %s.OutputRef(%q) in downstream jobs to reference the output\n", jobVar, job.Outputs[0].Name)
		w("\t_ = %s\n", jobVar)
	}
}

func emitJobFields(b *strings.Builder, job *pisyn.IRJob) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }

	if job.Image != "" {
		w(".\n\t\tImage(%q)", job.Image)
	}
	if len(job.ImageEntrypoint) > 0 {
		w(".\n\t\tImageEntrypoint(%s)", goStringArgs(job.ImageEntrypoint))
	}
	if job.ImageUser != "" {
		w(".\n\t\tImageUser(%q)", job.ImageUser)
	}

	// Scripts
	for _, a := range job.Actions {
		if a.Script == nil {
			continue
		}
		s := reverseVars(*a.Script)
		switch a.Phase {
		case pisyn.PhaseBefore:
			w(".\n\t\tBeforeScript(%s)", goMultilineScript(s))
		case pisyn.PhaseAfter:
			w(".\n\t\tAfterScript(%s)", goMultilineScript(s))
		default:
			w(".\n\t\tScript(%s)", goMultilineScript(s))
		}
	}

	if len(job.Needs) > 0 {
		w(".\n\t\tNeeds(%s)", goStringArgs(job.Needs))
	}
	if job.EmptyNeeds {
		w(".\n\t\tEmptyNeedsList()")
	}
	if len(job.Dependencies) > 0 {
		w(".\n\t\tDependencies(%s)", goStringArgs(job.Dependencies))
	}

	for k, v := range job.Env {
		w(".\n\t\tEnv(%q, %q)", k, reverseVars(v))
	}

	for _, svc := range job.Services {
		if len(svc.Variables) > 0 {
			w(".\n\t\tAddServiceWithVars(%q, %q, %s)", svc.Image, svc.Alias, goStringMap(svc.Variables))
		} else if svc.Alias != "" {
			w(".\n\t\tAddService(%q, %q)", svc.Image, svc.Alias)
		} else {
			w(".\n\t\tAddService(%q, %q)", svc.Image, svc.Image)
		}
	}

	if job.Artifacts != nil {
		a := job.Artifacts
		w(".\n\t\tSetArtifacts(ps.Artifacts{")
		if len(a.Paths) > 0 {
			w("Paths: %s, ", goStringSlice(a.Paths))
		}
		if a.Name != "" {
			w("Name: %q, ", a.Name)
		}
		if a.ExpireIn != "" {
			w("ExpireIn: %q, ", a.ExpireIn)
		}
		if len(a.Reports) > 0 {
			w("Reports: %s, ", goStringSliceMap(a.Reports))
		}
		w("})")
	}

	if job.Cache != nil {
		w(".\n\t\tSetCache(ps.Cache{Key: %q, Paths: %s})", job.Cache.Key, goStringSlice(job.Cache.Paths))
	}

	if job.Matrix != nil && len(job.Matrix.Dimensions) > 0 {
		w(".\n\t\tSetMatrix(%s)", goStringSliceMap(job.Matrix.Dimensions))
	}

	if job.Environment != nil {
		w(".\n\t\tSetEnvironment(%q, %q)", job.Environment.Name, job.Environment.URL)
	}

	for _, r := range job.Rules {
		w(".\n\t\tAddRule(%s)", goRule(r))
	}

	if job.AllowFailureCfg != nil && len(job.AllowFailureCfg.ExitCodes) > 0 {
		w(".\n\t\tAllowFailureOnExitCodes(%s)", goIntArgs(job.AllowFailureCfg.ExitCodes))
	} else if job.AllowFailure {
		w(".\n\t\tAllowFailure()")
	}

	if len(job.Tags) > 0 {
		w(".\n\t\tAddTag(%s)", goStringArgs(job.Tags))
	}

	if job.TimeoutMin > 0 {
		w(".\n\t\tTimeout(%d)", job.TimeoutMin)
	}

	if job.RetryConfig != nil {
		w(".\n\t\tSetRetry(ps.RetryConfig{Max: %d", job.RetryConfig.Max)
		if len(job.RetryConfig.When) > 0 {
			w(", When: %s", goStringSlice(job.RetryConfig.When))
		}
		w("})")
	} else if job.RetryCount > 0 {
		w(".\n\t\tRetry(%d)", job.RetryCount)
	}

	switch job.When {
	case pisyn.Manual:
		w(".\n\t\tSetWhen(ps.Manual)")
	case pisyn.Always:
		w(".\n\t\tSetWhen(ps.Always)")
	case pisyn.OnFailure:
		w(".\n\t\tSetWhen(ps.OnFailure)")
	}

	if job.Interruptible != nil {
		w(".\n\t\tSetInterruptible(%v)", *job.Interruptible)
	}

	for _, out := range job.Outputs {
		w(".\n\t\tOutput(%q, %q)", out.Name, out.DotenvFile)
	}
}

// emitTriggersFromWorkflowRules attempts to reverse-map common GitLab workflow:rules
// patterns back to pisyn trigger methods (OnPush, OnPR, OnPushTag, etc.).
func emitTriggersFromWorkflowRules(b *strings.Builder, rules []pisyn.Rule) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	for _, r := range rules {
		if r.When != "always" || r.If == "" {
			continue
		}
		cond := r.If
		switch {
		case cond == `$CI_MERGE_REQUEST_ID`:
			w(".\n\t\tOnPR(\"main\")")
		case cond == `$CI_COMMIT_REF_PROTECTED == "true"`:
			w(".\n\t\tOnPushProtected()")
		case cond == `$CI_PIPELINE_SOURCE == "schedule"`:
			// Can't determine cron from rule alone
			w(" // TODO: add OnSchedule(\"<cron>\")")
		case strings.HasPrefix(cond, `$CI_COMMIT_BRANCH == "`):
			branch := strings.TrimSuffix(strings.TrimPrefix(cond, `$CI_COMMIT_BRANCH == "`), `"`)
			w(".\n\t\tOnPush(%q)", branch)
		case strings.HasPrefix(cond, `$CI_COMMIT_TAG =~ /^`):
			pattern := strings.TrimSuffix(strings.TrimPrefix(cond, `$CI_COMMIT_TAG =~ /^`), `/`)
			pattern = strings.ReplaceAll(pattern, ".*", "*")
			w(".\n\t\tOnPushTag(%q)", pattern)
		}
	}
}

func isTriggerRule(r pisyn.Rule) bool {
	if r.When != "always" || r.If == "" {
		return false
	}
	c := r.If
	return c == `$CI_MERGE_REQUEST_ID` ||
		c == `$CI_COMMIT_REF_PROTECTED == "true"` ||
		c == `$CI_PIPELINE_SOURCE == "schedule"` ||
		strings.HasPrefix(c, `$CI_COMMIT_BRANCH == "`) ||
		strings.HasPrefix(c, `$CI_COMMIT_TAG =~ /^`)
}

func hasNonTriggerWorkflowRules(rules []pisyn.Rule) bool {
	for _, r := range rules {
		if !isTriggerRule(r) {
			return true
		}
	}
	return false
}

// reverseVars replaces GitLab CI variables with pisyn platform-neutral equivalents.
func reverseVars(s string) string {
	for glVar, pisynVar := range gitlabToPisyn {
		s = strings.ReplaceAll(s, "$"+glVar, "$"+pisynVar)
	}
	return s
}

// Go code formatting helpers

func goIdent(name string) string {
	r := strings.NewReplacer("-", "_", " ", "_", ".", "_", "/", "_")
	s := r.Replace(name)
	if s == "" {
		return "_ident"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "_" + s
	}
	switch s {
	case "break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if",
		"import", "interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var":
		s += "_"
	}
	return s
}

func goStringArgs(ss []string) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(parts, ", ")
}

func goStringSlice(ss []string) string {
	return "[]string{" + goStringArgs(ss) + "}"
}

func goMultilineScript(s string) string {
	if !strings.Contains(s, "\n") {
		if strings.Contains(s, "`") {
			return fmt.Sprintf("%q", s)
		}
		return "`" + s + "`"
	}
	// Multi-line: use raw string literal to preserve as single block
	if strings.Contains(s, "`") {
		return fmt.Sprintf("%q", s)
	}
	return "`" + s + "`"
}

func goIntArgs(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}

func goStringMap(m map[string]string) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%q: %q", k, v))
	}
	return "map[string]string{" + strings.Join(parts, ", ") + "}"
}

func goStringSliceMap(m map[string][]string) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%q: %s", k, goStringSlice(v)))
	}
	return "map[string][]string{" + strings.Join(parts, ", ") + "}"
}

func goRule(r pisyn.Rule) string {
	var parts []string
	if r.If != "" {
		parts = append(parts, fmt.Sprintf("If: %q", reverseVars(r.If)))
	}
	if r.When != "" {
		parts = append(parts, fmt.Sprintf("When: %q", r.When))
	}
	if r.AllowFailure {
		parts = append(parts, "AllowFailure: true")
	}
	if len(r.Changes) > 0 {
		parts = append(parts, fmt.Sprintf("Changes: %s", goStringSlice(r.Changes)))
	}
	if len(r.Exists) > 0 {
		parts = append(parts, fmt.Sprintf("Exists: %s", goStringSlice(r.Exists)))
	}
	if len(r.Variables) > 0 {
		parts = append(parts, fmt.Sprintf("Variables: %s", goStringMap(r.Variables)))
	}
	return "ps.Rule{" + strings.Join(parts, ", ") + "}"
}
