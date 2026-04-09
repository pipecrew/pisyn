// Package gitlab synthesizes pisyn pipelines to GitLab CI YAML.
package gitlab

import (
	"fmt"
	"strings"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth"
)

// Synthesizer generates GitLab CI configuration files.
type Synthesizer struct{}

// NewSynthesizer creates a new GitLab CI synthesizer.
func NewSynthesizer() *Synthesizer { return &Synthesizer{} }

func init() {
	pisyn.RegisterPlatform("gitlab", func() pisyn.Synthesizer { return NewSynthesizer() })
}

// Synth generates GitLab CI YAML from the app's construct tree.
func (syn *Synthesizer) Synth(app *pisyn.App, outDir string) error {
	pipelines := app.Pipelines()
	if len(pipelines) == 1 {
		out := syn.renderPipeline(pipelines[0])
		return synth.WriteYAML(outDir, ".gitlab-ci.yml", out)
	}
	return syn.renderMerged(pipelines, outDir)
}

// renderMerged merges multiple pipelines into a single .gitlab-ci.yml.
// Each pipeline's triggers become job-level rules so jobs only run in the right context.
func (syn *Synthesizer) renderMerged(pipelines []*pisyn.Pipeline, outDir string) error {
	cfg := synth.NewOrderedMap()

	// Collect all stages, includes, env, and workflow rules across pipelines
	var allStages []string
	var allWorkflowRules []map[string]any
	var allIncludes []pisyn.Include
	allEnv := map[string]string{}
	seen := map[string]bool{}

	for _, pipeline := range pipelines {
		for _, stage := range pipeline.Stages() {
			if !seen[stage.Name] {
				allStages = append(allStages, stage.Name)
				seen[stage.Name] = true
			}
		}
		if len(pipeline.WorkflowRules) > 0 {
			allWorkflowRules = append(allWorkflowRules, renderRuleList(pipeline.WorkflowRules)...)
		} else if wfRules := renderWorkflowRules(pipeline.On); len(wfRules) > 0 {
			allWorkflowRules = append(allWorkflowRules, wfRules...)
		}
		for key, val := range pipeline.Env {
			allEnv[key] = translateVars(val)
		}
		allIncludes = append(allIncludes, pipeline.IncludeList...)
	}

	if len(allIncludes) > 0 {
		cfg.Set("include", renderIncludes(allIncludes))
	}
	if len(allStages) > 0 {
		cfg.Set("stages", allStages)
	}
	if len(allWorkflowRules) > 0 {
		cfg.Set("workflow", map[string]any{"rules": allWorkflowRules})
	}
	if len(allEnv) > 0 {
		cfg.Set("variables", allEnv)
	}

	// Render jobs, injecting pipeline-level trigger rules onto jobs that don't have their own
	for _, pipeline := range pipelines {
		pipelineRules := renderWorkflowRules(pipeline.On)
		if len(pipeline.WorkflowRules) > 0 {
			pipelineRules = renderRuleList(pipeline.WorkflowRules)
		}
		for _, stage := range pipeline.Stages() {
			for _, job := range stage.Jobs() {
				rendered := renderJob(job, stage.Name)
				if len(job.Rules) == 0 && len(pipelineRules) > 0 {
					rendered["rules"] = pipelineRules
				}
				cfg.Set(job.JobName, rendered)
			}
		}
	}

	return synth.WriteYAML(outDir, ".gitlab-ci.yml", cfg)
}

func (syn *Synthesizer) renderPipeline(pipeline *pisyn.Pipeline) *synth.OrderedMap {
	cfg := synth.NewOrderedMap()

	// include: must come first in GitLab CI
	if len(pipeline.IncludeList) > 0 {
		cfg.Set("include", renderIncludes(pipeline.IncludeList))
	}

	stages := pipeline.Stages()
	if len(stages) > 0 {
		names := make([]string, len(stages))
		for idx, stage := range stages {
			names[idx] = stage.Name
		}
		cfg.Set("stages", names)
	}

	if len(pipeline.WorkflowRules) > 0 {
		cfg.Set("workflow", map[string]any{"rules": renderRuleList(pipeline.WorkflowRules)})
	} else if wfRules := renderWorkflowRules(pipeline.On); len(wfRules) > 0 {
		cfg.Set("workflow", map[string]any{"rules": wfRules})
	}

	if len(pipeline.Env) > 0 {
		translated := make(map[string]string, len(pipeline.Env))
		for key, val := range pipeline.Env {
			translated[key] = translateVars(val)
		}
		cfg.Set("variables", translated)
	}

	if pipeline.Defaults != nil {
		d := map[string]any{}
		if pipeline.Defaults.Image != "" {
			d["image"] = pipeline.Defaults.Image
		}
		if len(pipeline.Defaults.BeforeScript) > 0 {
			d["before_script"] = pipeline.Defaults.BeforeScript
		}
		if len(pipeline.Defaults.AfterScript) > 0 {
			d["after_script"] = pipeline.Defaults.AfterScript
		}
		if len(pipeline.Defaults.Tags) > 0 {
			d["tags"] = pipeline.Defaults.Tags
		}
		if len(d) > 0 {
			cfg.Set("default", d)
		}
	}

	for _, stage := range stages {
		for _, job := range stage.Jobs() {
			cfg.Set(job.JobName, renderJob(job, stage.Name))
		}
	}

	return cfg
}

func renderRuleList(rules []pisyn.Rule) []map[string]any {
	out := make([]map[string]any, len(rules))
	for idx, rule := range rules {
		entry := map[string]any{}
		if rule.If != "" {
			entry["if"] = translateVars(rule.If)
		}
		if rule.When != "" {
			entry["when"] = rule.When
		}
		if rule.AllowFailure {
			entry["allow_failure"] = true
		}
		if len(rule.Changes) > 0 {
			entry["changes"] = rule.Changes
		}
		if len(rule.Exists) > 0 {
			entry["exists"] = rule.Exists
		}
		if len(rule.Variables) > 0 {
			entry["variables"] = rule.Variables
		}
		out[idx] = entry
	}
	return out
}

func renderWorkflowRules(triggers pisyn.Triggers) []map[string]any {
	var rules []map[string]any
	if triggers.Push != nil {
		if triggers.Push.Protected {
			rules = append(rules, map[string]any{
				"if":   `$CI_COMMIT_REF_PROTECTED == "true"`,
				"when": "always",
			})
		}
		for _, branch := range triggers.Push.Branches {
			rules = append(rules, map[string]any{
				"if":   fmt.Sprintf(`$CI_COMMIT_BRANCH == "%s"`, branch),
				"when": "always",
			})
		}
		for _, tag := range triggers.Push.Tags {
			rules = append(rules, map[string]any{
				"if":   fmt.Sprintf(`$CI_COMMIT_TAG =~ /^%s/`, tagPatternToRegex(tag)),
				"when": "always",
			})
		}
	}
	if triggers.PullRequest != nil {
		rules = append(rules, map[string]any{
			"if":   `$CI_MERGE_REQUEST_ID`,
			"when": "always",
		})
	}
	if len(triggers.Schedule) > 0 {
		rules = append(rules, map[string]any{
			"if":   `$CI_PIPELINE_SOURCE == "schedule"`,
			"when": "always",
		})
	}
	return rules
}

// tagPatternToRegex converts a glob-style tag pattern (e.g. "v*") to a regex fragment.
func tagPatternToRegex(pattern string) string {
	return strings.ReplaceAll(pattern, "*", ".*")
}

// renderIncludes converts pisyn Include entries to GitLab CI include format.
func renderIncludes(includes []pisyn.Include) []map[string]any {
	out := make([]map[string]any, len(includes))
	for idx, inc := range includes {
		entry := map[string]any{}
		switch {
		case inc.Local != "":
			entry["local"] = inc.Local
		case inc.Remote != "":
			entry["remote"] = inc.Remote
		case inc.Project != "":
			entry["project"] = inc.Project
			entry["file"] = inc.File
			if inc.Ref != "" {
				entry["ref"] = inc.Ref
			}
		case inc.Template != "":
			entry["template"] = inc.Template
		}
		out[idx] = entry
	}
	return out
}

// renderJob converts a pisyn Job into a GitLab CI job configuration map.
func renderJob(job *pisyn.Job, stageName string) map[string]any {
	cfg := map[string]any{"stage": stageName}

	// Container environment
	setImage(cfg, job)
	setServices(cfg, job)

	// Script phases (steps/actions are GitHub-only and intentionally ignored)
	setScripts(cfg, job)

	// Job ordering and dependencies
	setNeeds(cfg, job)
	setVariables(cfg, job)

	// Build artifacts and job outputs (dotenv)
	setArtifacts(cfg, job)
	setCache(cfg, job)

	// Execution control
	setMatrix(cfg, job)
	setEnvironment(cfg, job)
	setRules(cfg, job)
	setFailurePolicy(cfg, job)
	setTags(cfg, job)
	setTimeout(cfg, job)
	setRetry(cfg, job)
	setWhen(cfg, job)
	setInterruptible(cfg, job)

	return cfg
}

// setImage renders the container image. Uses the short string form when no
// entrypoint or docker user override is needed, otherwise the object form.
func setImage(cfg map[string]any, job *pisyn.Job) {
	if job.ImageName == "" {
		return
	}
	if len(job.ImageEP) == 0 && job.ImageUsr == "" {
		cfg["image"] = job.ImageName
		return
	}
	img := map[string]any{"name": job.ImageName}
	if len(job.ImageEP) > 0 {
		img["entrypoint"] = job.ImageEP
	}
	if job.ImageUsr != "" {
		img["docker"] = map[string]any{"user": job.ImageUsr}
	}
	cfg["image"] = img
}

// setServices renders sidecar containers (e.g. postgres, redis).
// Uses the short string form when no alias or variables are needed.
func setServices(cfg map[string]any, job *pisyn.Job) {
	if len(job.ServiceList) == 0 {
		return
	}
	svcs := make([]any, len(job.ServiceList))
	for idx, svc := range job.ServiceList {
		if svc.Alias == "" && len(svc.Variables) == 0 {
			svcs[idx] = svc.Image
			continue
		}
		entry := map[string]any{"name": svc.Image}
		if svc.Alias != "" {
			entry["alias"] = svc.Alias
		}
		if len(svc.Variables) > 0 {
			entry["variables"] = svc.Variables
		}
		svcs[idx] = entry
	}
	cfg["services"] = svcs
}

// setScripts renders before_script, script, and after_script from the job's
// action list. Only script-type actions are included; step-type actions
// (GitHub uses: directives) are intentionally skipped.
func setScripts(cfg map[string]any, job *pisyn.Job) {
	if before := job.BeforeScriptLines(); len(before) > 0 {
		cfg["before_script"] = translateSlice(before)
	}
	if scripts := job.ScriptLines(); len(scripts) > 0 {
		cfg["script"] = translateSlice(scripts)
	}
	if after := job.AfterScriptLines(); len(after) > 0 {
		cfg["after_script"] = translateSlice(after)
	}
}

// setNeeds renders job dependencies. An empty needs list (needs: []) means
// the job starts immediately without waiting for prior stages.
func setNeeds(cfg map[string]any, job *pisyn.Job) {
	if len(job.NeedsList) > 0 {
		cfg["needs"] = job.NeedsList
	} else if job.EmptyNeeds {
		cfg["needs"] = []any{}
	}
	if len(job.DependencyList) > 0 {
		cfg["dependencies"] = job.DependencyList
	}
}

// setVariables renders job-level environment variables with variable translation.
func setVariables(cfg map[string]any, job *pisyn.Job) {
	if len(job.EnvVars) == 0 {
		return
	}
	translated := make(map[string]string, len(job.EnvVars))
	for key, val := range job.EnvVars {
		translated[key] = translateVars(val)
	}
	cfg["variables"] = translated
}

// setArtifacts renders artifact paths, reports, and job outputs.
// Job outputs use GitLab's dotenv artifact reports to pass data between jobs.
func setArtifacts(cfg map[string]any, job *pisyn.Job) {
	if job.ArtifactsCfg == nil && len(job.OutputList) == 0 {
		return
	}
	artifacts := map[string]any{}
	if job.ArtifactsCfg != nil {
		if len(job.ArtifactsCfg.Paths) > 0 {
			artifacts["paths"] = job.ArtifactsCfg.Paths
		}
		if job.ArtifactsCfg.Name != "" {
			artifacts["name"] = job.ArtifactsCfg.Name
		}
		if job.ArtifactsCfg.ExpireIn != "" {
			artifacts["expire_in"] = job.ArtifactsCfg.ExpireIn
		}
		if len(job.ArtifactsCfg.Reports) > 0 {
			artifacts["reports"] = job.ArtifactsCfg.Reports
		}
	}
	// Job outputs are implemented as dotenv artifact reports in GitLab.
	// Consuming jobs receive the variables automatically via needs.
	if len(job.OutputList) > 0 {
		reports, _ := artifacts["reports"].(map[string][]string)
		if reports == nil {
			reports = map[string][]string{}
		}
		for _, output := range job.OutputList {
			reports["dotenv"] = append(reports["dotenv"], output.DotenvFile)
		}
		artifacts["reports"] = reports
	}
	cfg["artifacts"] = artifacts
}

// setCache renders the cache configuration for dependency caching.
func setCache(cfg map[string]any, job *pisyn.Job) {
	if job.CacheCfg == nil {
		return
	}
	cfg["cache"] = map[string]any{"key": job.CacheCfg.Key, "paths": job.CacheCfg.Paths}
}

// setMatrix renders parallel matrix builds (parallel:matrix in GitLab).
func setMatrix(cfg map[string]any, job *pisyn.Job) {
	if job.MatrixCfg != nil && len(job.MatrixCfg.Dimensions) > 0 {
		cfg["parallel"] = map[string]any{"matrix": []any{job.MatrixCfg.Dimensions}}
	}
}

// setEnvironment renders the deployment environment (name + optional URL).
func setEnvironment(cfg map[string]any, job *pisyn.Job) {
	if job.EnvironmentCfg == nil {
		return
	}
	env := map[string]any{"name": job.EnvironmentCfg.Name}
	if job.EnvironmentCfg.URL != "" {
		env["url"] = job.EnvironmentCfg.URL
	}
	cfg["environment"] = env
}

// setRules renders conditional execution rules (if, changes, when).
func setRules(cfg map[string]any, job *pisyn.Job) {
	if len(job.Rules) > 0 {
		cfg["rules"] = renderRuleList(job.Rules)
	}
}

// setFailurePolicy renders allow_failure with optional exit code filtering.
func setFailurePolicy(cfg map[string]any, job *pisyn.Job) {
	if job.AllowFailureCfg != nil && len(job.AllowFailureCfg.ExitCodes) > 0 {
		cfg["allow_failure"] = map[string]any{"exit_codes": job.AllowFailureCfg.ExitCodes}
	} else if job.IsAllowFailure {
		cfg["allow_failure"] = true
	}
}

// setTags renders runner selection tags.
func setTags(cfg map[string]any, job *pisyn.Job) {
	if len(job.Tags) > 0 {
		cfg["tags"] = job.Tags
	}
}

// setTimeout renders the job timeout in minutes.
func setTimeout(cfg map[string]any, job *pisyn.Job) {
	if job.TimeoutMin > 0 {
		cfg["timeout"] = fmt.Sprintf("%dm", job.TimeoutMin)
	}
}

// setRetry renders retry configuration. Supports both simple count and
// advanced config with failure condition filters and exit codes.
func setRetry(cfg map[string]any, job *pisyn.Job) {
	if job.RetryCfg != nil {
		retry := map[string]any{"max": job.RetryCfg.Max}
		if len(job.RetryCfg.When) > 0 {
			retry["when"] = job.RetryCfg.When
		}
		if len(job.RetryCfg.ExitCodes) > 0 {
			retry["exit_codes"] = job.RetryCfg.ExitCodes
		}
		cfg["retry"] = retry
	} else if job.RetryCount > 0 {
		cfg["retry"] = job.RetryCount
	}
}

// setWhen renders the execution policy (manual, always, on_failure).
// Defaults to on_success which is GitLab's implicit default, so we skip it.
func setWhen(cfg map[string]any, job *pisyn.Job) {
	switch job.When {
	case pisyn.Manual:
		cfg["when"] = "manual"
	case pisyn.Always:
		cfg["when"] = "always"
	case pisyn.OnFailure:
		cfg["when"] = "on_failure"
	}
}

// setInterruptible marks whether newer pipelines can cancel this job.
func setInterruptible(cfg map[string]any, job *pisyn.Job) {
	if job.Interruptible != nil {
		cfg["interruptible"] = *job.Interruptible
	}
}

// pisynToGitLab maps pisyn platform-neutral variables to GitLab CI equivalents.
var pisynToGitLab = pisyn.GitLabVars

func translateVars(str string) string {
	for pisynVar, gitlabVar := range pisynToGitLab {
		str = strings.ReplaceAll(str, "${"+pisynVar+"}", "${"+gitlabVar+"}")
		str = strings.ReplaceAll(str, "$"+pisynVar, "$"+gitlabVar)
	}
	// Translate output refs: $PISYN_OUTPUT_JOBNAME_VARNAME → $VARNAME
	// GitLab injects dotenv vars directly by name
	str = translateOutputRefs(str, func(_, varName string) string {
		return "$" + varName
	})
	return str
}

// translateOutputRefs finds $PISYN_OUTPUT_<JOB>_<VAR> patterns and replaces them.
func translateOutputRefs(str string, replacer func(jobName, varName string) string) string {
	const prefix = "$PISYN_OUTPUT_"
	for {
		idx := strings.Index(str, prefix)
		if idx == -1 {
			break
		}
		// Find the end of the variable name
		end := idx + len(prefix)
		for end < len(str) && (str[end] == '_' || (str[end] >= 'A' && str[end] <= 'Z') || (str[end] >= '0' && str[end] <= '9')) {
			end++
		}
		token := str[idx+len(prefix) : end]
		// Split into JOB__VAR using double underscore separator
		parts := strings.SplitN(token, "__", 2)
		if len(parts) == 2 {
			replacement := replacer(parts[0], parts[1])
			str = str[:idx] + replacement + str[end:]
		} else {
			break // malformed, stop
		}
	}
	return str
}

func translateSlice(lines []string) []string {
	out := make([]string, len(lines))
	for idx, line := range lines {
		out[idx] = translateVars(line)
	}
	return out
}
