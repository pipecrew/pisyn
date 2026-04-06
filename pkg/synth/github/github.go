// Package github synthesizes pisyn pipelines to GitHub Actions YAML.
package github

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth"
)

// Synthesizer generates GitHub Actions workflow files.
type Synthesizer struct{}

// NewSynthesizer creates a new GitHub Actions synthesizer.
func NewSynthesizer() *Synthesizer { return &Synthesizer{} }

func init() {
	pisyn.RegisterPlatform("github", func() pisyn.Synthesizer { return NewSynthesizer() })
}

// Synth generates GitHub Actions YAML from the app's construct tree.
func (syn *Synthesizer) Synth(app *pisyn.App, outDir string) error {
	for _, pipeline := range app.Pipelines() {
		out := syn.renderPipeline(pipeline)
		dir := filepath.Join(outDir, ".github", "workflows")
		name := strings.ReplaceAll(strings.ToLower(pipeline.Name), " ", "-")
		name = strings.ReplaceAll(name, "/", "-") + ".yml"
		if err := synth.WriteYAML(dir, name, out); err != nil {
			return err
		}
	}
	return nil
}

func (syn *Synthesizer) renderPipeline(pipeline *pisyn.Pipeline) *synth.OrderedMap {
	workflow := synth.NewOrderedMap()
	workflow.Set("name", pipeline.Name)

	on := renderTriggers(pipeline.On)
	on["workflow_dispatch"] = map[string]any{}
	workflow.Set("on", on)

	if len(pipeline.Env) > 0 {
		translated := make(map[string]string, len(pipeline.Env))
		for key, val := range pipeline.Env {
			translated[key] = translateVars(val)
		}
		workflow.Set("env", translated)
	}

	jobs := synth.NewOrderedMap()
	var prevStageJobs []string
	for _, stage := range pipeline.Stages() {
		var currentJobs []string
		for _, job := range stage.Jobs() {
			jobs.Set(job.JobName, renderJob(job, prevStageJobs))
			currentJobs = append(currentJobs, job.JobName)
		}
		prevStageJobs = currentJobs
	}
	workflow.Set("jobs", jobs)

	return workflow
}

func renderTriggers(triggers pisyn.Triggers) map[string]any {
	on := map[string]any{}
	if triggers.Push != nil {
		on["push"] = map[string]any{"branches": triggers.Push.Branches}
	}
	if triggers.PullRequest != nil {
		on["pull_request"] = map[string]any{"branches": triggers.PullRequest.Branches}
	}
	if len(triggers.Schedule) > 0 {
		scheds := make([]map[string]any, len(triggers.Schedule))
		for idx, sched := range triggers.Schedule {
			scheds[idx] = map[string]any{"cron": sched.Cron}
		}
		on["schedule"] = scheds
	}
	return on
}

// renderJob converts a pisyn Job into a GitHub Actions job configuration map.
func renderJob(job *pisyn.Job, prevStageJobs []string) map[string]any {
	cfg := map[string]any{"runs-on": job.Runner}

	// Job ordering
	setNeeds(cfg, job, prevStageJobs)
	setOutputs(cfg, job)

	// Job-level configuration
	setEnv(cfg, job)
	setServices(cfg, job)
	setMatrix(cfg, job)
	setEnvironment(cfg, job)
	setCondition(cfg, job)
	setTimeout(cfg, job)
	setFailurePolicy(cfg, job)
	setContainer(cfg, job)

	// Build the ordered list of workflow steps
	cfg["steps"] = buildSteps(job)

	return cfg
}

// setNeeds renders job dependencies. Falls back to previous stage jobs
// if no explicit needs are declared (implicit stage ordering).
func setNeeds(cfg map[string]any, job *pisyn.Job, prevStageJobs []string) {
	needs := append([]string{}, job.NeedsList...)
	if len(needs) == 0 && len(prevStageJobs) > 0 {
		needs = prevStageJobs
	}
	if len(needs) > 0 {
		cfg["needs"] = needs
	}
}

// setOutputs declares job outputs for cross-job data passing.
// Values reference the "outputs" step which writes to $GITHUB_OUTPUT.
func setOutputs(cfg map[string]any, job *pisyn.Job) {
	if len(job.OutputList) == 0 {
		return
	}
	outputs := map[string]string{}
	for _, output := range job.OutputList {
		outputs[strings.ToLower(output.Name)] = fmt.Sprintf("${{ steps.outputs.outputs.%s }}", strings.ToLower(output.Name))
	}
	cfg["outputs"] = outputs
}

// setEnv renders job-level environment variables with variable translation.
func setEnv(cfg map[string]any, job *pisyn.Job) {
	if len(job.EnvVars) == 0 {
		return
	}
	translated := make(map[string]string, len(job.EnvVars))
	for key, val := range job.EnvVars {
		translated[key] = translateVars(val)
	}
	cfg["env"] = translated
}

// setServices renders sidecar service containers (e.g. postgres, redis).
func setServices(cfg map[string]any, job *pisyn.Job) {
	if len(job.ServiceList) == 0 {
		return
	}
	svcs := map[string]any{}
	for _, svc := range job.ServiceList {
		key := svc.Alias
		if key == "" {
			key = svc.Image
		}
		entry := map[string]any{"image": svc.Image}
		if len(svc.Variables) > 0 {
			entry["env"] = svc.Variables
		}
		svcs[key] = entry
	}
	cfg["services"] = svcs
}

// setMatrix renders the build matrix strategy.
func setMatrix(cfg map[string]any, job *pisyn.Job) {
	if job.MatrixCfg != nil && len(job.MatrixCfg.Dimensions) > 0 {
		cfg["strategy"] = map[string]any{"matrix": job.MatrixCfg.Dimensions}
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

// setCondition renders the job-level `if` condition from rules or when policy.
// Combines multiple rule conditions with OR. The "when: never" rules are
// excluded since they serve as GitLab-style fallthrough terminators.
func setCondition(cfg map[string]any, job *pisyn.Job) {
	if job.When == pisyn.OnFailure {
		cfg["if"] = "failure()"
		return
	}
	if len(job.Rules) == 0 {
		return
	}
	var conditions []string
	for _, rule := range job.Rules {
		if rule.If != "" && rule.When != "never" {
			conditions = append(conditions, translateVars(rule.If))
		}
	}
	if len(conditions) > 0 {
		cfg["if"] = strings.Join(conditions, " ||\n")
	}
}

// setTimeout renders the job timeout in minutes.
func setTimeout(cfg map[string]any, job *pisyn.Job) {
	if job.TimeoutMin > 0 {
		cfg["timeout-minutes"] = job.TimeoutMin
	}
}

// setFailurePolicy renders continue-on-error for allowed failures.
func setFailurePolicy(cfg map[string]any, job *pisyn.Job) {
	if (job.AllowFailureCfg != nil && job.AllowFailureCfg.Enabled) || job.IsAllowFailure {
		cfg["continue-on-error"] = true
	}
}

// setContainer renders the Docker container configuration.
// GitHub Actions requires both runs-on (the VM) and container (the Docker image).
func setContainer(cfg map[string]any, job *pisyn.Job) {
	if job.ImageName == "" {
		return
	}
	container := map[string]any{"image": job.ImageName}
	if len(job.ImageEP) > 0 {
		ep := job.ImageEP[0]
		if ep == "" {
			ep = `""`
		}
		container["options"] = "--entrypoint " + ep
	}
	cfg["container"] = container
}

// buildSteps assembles the ordered list of workflow steps:
// 1. Checkout
// 2. Cache (if configured)
// 3. User-defined actions (scripts + steps, in declaration order)
// 4. Output collection (if job has outputs)
// 5. Artifact upload (if configured)
func buildSteps(job *pisyn.Job) []map[string]any {
	var steps []map[string]any

	// Every job starts with a checkout
	steps = append(steps, map[string]any{"uses": "actions/checkout@v5"})

	// Dependency caching (rendered as an actions/cache step)
	if job.CacheCfg != nil {
		steps = append(steps, map[string]any{
			"name": "Cache dependencies",
			"uses": "actions/cache@v4",
			"with": map[string]any{
				"path": strings.Join(job.CacheCfg.Paths, "\n"),
				"key":  job.CacheCfg.Key,
			},
		})
	}

	// User-defined actions: scripts become run: steps, AddStep() becomes uses: steps.
	// Before/after scripts get descriptive names; after_script runs with if: always().
	for _, action := range job.Actions {
		if action.Script != nil {
			steps = append(steps, buildScriptStep(action))
		} else if action.Step != nil {
			steps = append(steps, buildActionStep(action.Step))
		}
	}

	// Collect job outputs by reading dotenv files into $GITHUB_OUTPUT
	if len(job.OutputList) > 0 {
		var lines []string
		for _, output := range job.OutputList {
			lines = append(lines, fmt.Sprintf("cat %s >> $GITHUB_OUTPUT", output.DotenvFile))
		}
		steps = append(steps, map[string]any{
			"name": "Set outputs",
			"id":   "outputs",
			"run":  strings.Join(lines, "\n"),
		})
	}

	// Upload artifacts via actions/upload-artifact
	if job.ArtifactsCfg != nil && len(job.ArtifactsCfg.Paths) > 0 {
		artifactName := job.ArtifactsCfg.Name
		if artifactName == "" {
			artifactName = fmt.Sprintf("%s-artifacts", job.JobName)
		}
		steps = append(steps, map[string]any{
			"name": "Upload artifacts",
			"if":   "always()",
			"uses": "actions/upload-artifact@v4",
			"with": map[string]any{
				"name": artifactName,
				"path": strings.Join(job.ArtifactsCfg.Paths, "\n"),
			},
		})
	}

	return steps
}

// buildScriptStep converts a script action into a run: step.
func buildScriptStep(action pisyn.Action) map[string]any {
	step := map[string]any{"run": translateVars(*action.Script)}
	switch action.Phase {
	case pisyn.PhaseBefore:
		step["name"] = "Before script"
	case pisyn.PhaseAfter:
		step["name"] = "After script"
		step["if"] = "always()"
	default:
		step["name"] = "Run"
	}
	return step
}

// buildActionStep converts a pisyn Step (uses: action) into a GitHub step.
func buildActionStep(actionStep *pisyn.Step) map[string]any {
	step := map[string]any{}
	if actionStep.ID != "" {
		step["id"] = actionStep.ID
	}
	if actionStep.Name != "" {
		step["name"] = actionStep.Name
	}
	step["uses"] = actionStep.Uses
	if actionStep.If != "" {
		step["if"] = translateVars(actionStep.If)
	}
	if len(actionStep.With) > 0 {
		step["with"] = actionStep.With
	}
	return step
}

// pisynToGitHub maps pisyn platform-neutral variables to GitHub Actions equivalents.
var pisynToGitHub = pisyn.GitHubVars

// translateVars replaces pisyn variables with GitHub Actions equivalents.
func translateVars(str string) string {
	for pisynVar, ghVar := range pisynToGitHub {
		str = strings.ReplaceAll(str, "${"+pisynVar+"}", ghVar)
		str = strings.ReplaceAll(str, "$"+pisynVar, ghVar)
	}
	// Translate output refs: $PISYN_OUTPUT_JOBNAME_VARNAME → ${{ needs.job-name.outputs.varname }}
	str = translateOutputRefs(str)
	return str
}

func translateOutputRefs(str string) string {
	const prefix = "$PISYN_OUTPUT_"
	for {
		idx := strings.Index(str, prefix)
		if idx == -1 {
			break
		}
		end := idx + len(prefix)
		for end < len(str) && (str[end] == '_' || (str[end] >= 'A' && str[end] <= 'Z') || (str[end] >= '0' && str[end] <= '9')) {
			end++
		}
		token := str[idx+len(prefix) : end]
		parts := strings.SplitN(token, "__", 2)
		if len(parts) == 2 {
			jobName := strings.ToLower(strings.ReplaceAll(parts[0], "_", "-"))
			varName := strings.ToLower(parts[1])
			str = str[:idx] + fmt.Sprintf("${{ needs.%s.outputs.%s }}", jobName, varName) + str[end:]
		} else {
			break
		}
	}
	return str
}
