package pisyn

import (
	"fmt"
	"strings"
)

// Job represents a single CI/CD job within a stage.
type Job struct {
	Construct
	JobName         string
	ImageName       string
	ImageEP         []string
	ImageUsr        string
	Actions         []Action
	NeedsList       []string
	Runner          string
	ServiceList     []Service
	EnvVars         map[string]string
	ArtifactsCfg    *Artifacts
	CacheCfg        *Cache
	MatrixCfg       *Matrix
	EnvironmentCfg  *Environment
	IsAllowFailure  bool
	AllowFailureCfg *AllowFailureConfig
	TimeoutMin      int
	RetryCount      int
	RetryCfg        *RetryConfig
	When            WhenPolicy
	Tags            []string
	Rules           []Rule
	Interruptible   *bool
	DependencyList  []string
	EmptyNeeds      bool
	OutputList      []JobOutput
	FetchDepth      int // 0 = full history, -1 = default (unset)
}

// NewJob creates a new job in the given stage.
func NewJob(scope *Stage, name string) *Job {
	if err := checkDuplicateJobName(scope, name); err != nil {
		panic(err.Error())
	}
	j := &Job{
		JobName:    name,
		Runner:     "ubuntu-latest",
		EnvVars:    map[string]string{},
		FetchDepth: -1,
	}
	j.Construct = newConstruct(&scope.Construct, name, j)
	return j
}

// JobTemplate creates a Job without a scope — for use as a reusable template.
func JobTemplate(name string) *Job {
	return &Job{
		JobName:    name,
		Runner:     "ubuntu-latest",
		EnvVars:    map[string]string{},
		FetchDepth: -1,
	}
}

// Clone deep-copies this job and attaches it to the given stage.
func (j *Job) Clone(scope *Stage, name string) *Job {
	if err := checkDuplicateJobName(scope, name); err != nil {
		panic(err.Error())
	}
	c := &Job{
		JobName:         name,
		ImageName:       j.ImageName,
		ImageEP:         cloneStrings(j.ImageEP),
		ImageUsr:        j.ImageUsr,
		Actions:         cloneActions(j.Actions),
		NeedsList:       cloneStrings(j.NeedsList),
		Runner:          j.Runner,
		ServiceList:     cloneSlice(j.ServiceList),
		EnvVars:         cloneMap(j.EnvVars),
		IsAllowFailure:  j.IsAllowFailure,
		TimeoutMin:      j.TimeoutMin,
		RetryCount:      j.RetryCount,
		When:            j.When,
		Tags:            cloneStrings(j.Tags),
		Rules:           cloneSlice(j.Rules),
		DependencyList:  cloneStrings(j.DependencyList),
		EmptyNeeds:      j.EmptyNeeds,
		OutputList:      cloneSlice(j.OutputList),
		FetchDepth:      j.FetchDepth,
	}
	if j.ArtifactsCfg != nil {
		a := *j.ArtifactsCfg
		a.Paths = cloneStrings(a.Paths)
		c.ArtifactsCfg = &a
	}
	if j.CacheCfg != nil {
		ca := *j.CacheCfg
		ca.Paths = cloneStrings(ca.Paths)
		c.CacheCfg = &ca
	}
	if j.MatrixCfg != nil {
		m := *j.MatrixCfg
		m.Dimensions = cloneMapSlice(m.Dimensions)
		c.MatrixCfg = &m
	}
	if j.EnvironmentCfg != nil {
		e := *j.EnvironmentCfg
		c.EnvironmentCfg = &e
	}
	// Deep-copy fields that were previously aliased by pointer so a clone
	// can't mutate the template via RetryCfg.When, AllowFailureCfg.ExitCodes,
	// or *Interruptible (#7).
	if j.RetryCfg != nil {
		r := *j.RetryCfg
		r.When = cloneStrings(r.When)
		r.ExitCodes = cloneSlice(r.ExitCodes)
		c.RetryCfg = &r
	}
	if j.AllowFailureCfg != nil {
		a := *j.AllowFailureCfg
		a.ExitCodes = cloneSlice(a.ExitCodes)
		c.AllowFailureCfg = &a
	}
	if j.Interruptible != nil {
		v := *j.Interruptible
		c.Interruptible = &v
	}
	c.Construct = newConstruct(&scope.Construct, name, c)
	return c
}

// Image sets the container image for the job.
func (j *Job) Image(img string) *Job { j.ImageName = img; return j }

// ImageEntrypoint overrides the image's entrypoint.
func (j *Job) ImageEntrypoint(cmds ...string) *Job { j.ImageEP = cmds; return j }

// ImageUser sets the Docker user for the image.
func (j *Job) ImageUser(user string) *Job { j.ImageUsr = user; return j }

// SetFetchDepth sets the checkout fetch depth (0 = full history).
func (j *Job) SetFetchDepth(depth int) *Job { j.FetchDepth = depth; return j }

// Script appends a script block to the job's actions.
func (j *Job) Script(cmds ...string) *Job {
	s := strings.Join(cmds, "\n")
	j.Actions = append(j.Actions, Action{Script: &s})
	return j
}

// PrependScript inserts a script block before all existing main-phase actions.
func (j *Job) PrependScript(cmds ...string) *Job {
	s := strings.Join(cmds, "\n")
	a := Action{Script: &s}
	insertAt := 0
	for i, existing := range j.Actions {
		if existing.Phase == PhaseBefore {
			insertAt = i + 1
		}
	}
	j.Actions = append(j.Actions[:insertAt], append([]Action{a}, j.Actions[insertAt:]...)...)
	return j
}

// SetScript removes all existing main-phase script actions and adds a new one.
func (j *Job) SetScript(cmds ...string) *Job {
	j.removePhase(PhaseMain)
	return j.Script(cmds...)
}

// AddStep appends a pre-built action step (e.g. GitHub Actions `uses:`).
func (j *Job) AddStep(step Step) *Job {
	j.Actions = append(j.Actions, Action{Step: &step})
	return j
}

// ScriptLines returns all script strings from the job's actions (ignoring steps).
func (j *Job) ScriptLines() []string {
	return j.scriptsByPhase(PhaseMain)
}

// BeforeScriptLines returns before-phase script strings.
func (j *Job) BeforeScriptLines() []string {
	return j.scriptsByPhase(PhaseBefore)
}

// AfterScriptLines returns after-phase script strings.
func (j *Job) AfterScriptLines() []string {
	return j.scriptsByPhase(PhaseAfter)
}

func (j *Job) scriptsByPhase(phase ActionPhase) []string {
	var lines []string
	for _, a := range j.Actions {
		if a.Script != nil && a.Phase == phase {
			lines = append(lines, *a.Script)
		}
	}
	return lines
}

// BeforeScript sets commands to run before the main script (replaces any previous before script).
func (j *Job) BeforeScript(cmds ...string) *Job {
	s := strings.Join(cmds, "\n")
	j.removePhase(PhaseBefore)
	j.Actions = append([]Action{{Script: &s, Phase: PhaseBefore}}, j.Actions...)
	return j
}

// AfterScript sets commands to run after the main script, even on failure (replaces any previous after script).
func (j *Job) AfterScript(cmds ...string) *Job {
	s := strings.Join(cmds, "\n")
	j.removePhase(PhaseAfter)
	j.Actions = append(j.Actions, Action{Script: &s, Phase: PhaseAfter})
	return j
}

func (j *Job) removePhase(phase ActionPhase) {
	filtered := make([]Action, 0, len(j.Actions))
	for _, a := range j.Actions {
		if a.Phase != phase {
			filtered = append(filtered, a)
		}
	}
	j.Actions = filtered
}

// Needs declares job dependencies (jobs that must complete before this one).
func (j *Job) Needs(jobs ...string) *Job { j.NeedsList = append(j.NeedsList, jobs...); return j }

// RunsOn sets the runner label (e.g. "ubuntu-latest" for GitHub Actions).
func (j *Job) RunsOn(runner string) *Job { j.Runner = runner; return j }

// Env sets an environment variable on the job.
func (j *Job) Env(key, value string) *Job { j.EnvVars[key] = value; return j }

// AllowFailure marks the job as allowed to fail without failing the pipeline.
func (j *Job) AllowFailure() *Job {
	j.IsAllowFailure = true
	j.AllowFailureCfg = nil // clear exit-code-specific config
	return j
}

// AllowFailureOnExitCodes allows failure only on specific exit codes.
func (j *Job) AllowFailureOnExitCodes(codes ...int) *Job {
	j.AllowFailureCfg = &AllowFailureConfig{Enabled: true, ExitCodes: codes}
	return j
}

// IsAllowedToFail returns true if the job is configured to not fail the pipeline on error.
func (j *Job) IsAllowedToFail() bool {
	return j.IsAllowFailure || (j.AllowFailureCfg != nil && j.AllowFailureCfg.Enabled)
}

// Timeout sets the job timeout in minutes.
func (j *Job) Timeout(minutes int) *Job { j.TimeoutMin = minutes; return j }

// Retry sets a simple retry count for the job.
func (j *Job) Retry(times int) *Job { j.RetryCount = times; return j }

// SetRetry sets advanced retry configuration with failure conditions.
func (j *Job) SetRetry(cfg RetryConfig) *Job { j.RetryCfg = &cfg; return j }

// SetWhen sets the job's execution policy (e.g. Manual, Always).
func (j *Job) SetWhen(policy WhenPolicy) *Job { j.When = policy; return j }

// If sets a simple condition for job execution (shorthand for AddRule with If).
func (j *Job) If(condition string) *Job {
	j.Rules = append([]Rule{{If: condition}}, j.Rules...)
	return j
}

// AddTag adds runner tags for job scheduling.
func (j *Job) AddTag(tags ...string) *Job { j.Tags = append(j.Tags, tags...); return j }

// AddRule adds a conditional rule for job execution.
func (j *Job) AddRule(r Rule) *Job { j.Rules = append(j.Rules, r); return j }

// SetInterruptible marks whether the job can be cancelled by newer pipelines.
func (j *Job) SetInterruptible(v bool) *Job { j.Interruptible = &v; return j }

// Dependencies controls which jobs' artifacts to download (separate from needs).
func (j *Job) Dependencies(deps ...string) *Job {
	j.DependencyList = append(j.DependencyList, deps...)
	return j
}

// EmptyNeedsList sets an empty needs list so the job starts immediately.
func (j *Job) EmptyNeedsList() *Job { j.EmptyNeeds = true; return j }

// AddService adds a service container to the job.
func (j *Job) AddService(image, alias string) *Job {
	j.ServiceList = append(j.ServiceList, Service{Image: image, Alias: alias})
	return j
}

// AddServiceWithVars adds a service container with environment variables.
func (j *Job) AddServiceWithVars(image, alias string, vars map[string]string) *Job {
	j.ServiceList = append(j.ServiceList, Service{Image: image, Alias: alias, Variables: vars})
	return j
}

// SetArtifacts configures artifact collection for the job.
func (j *Job) SetArtifacts(a Artifacts) *Job { j.ArtifactsCfg = &a; return j }

// SetCache configures caching for the job.
func (j *Job) SetCache(c Cache) *Job { j.CacheCfg = &c; return j }

// SetEnvironment sets the deployment environment for the job.
func (j *Job) SetEnvironment(name, url string) *Job {
	j.EnvironmentCfg = &Environment{Name: name, URL: url}
	return j
}

// SetMatrix configures matrix builds with multiple variable dimensions.
func (j *Job) SetMatrix(dimensions map[string][]string) *Job {
	j.MatrixCfg = &Matrix{Dimensions: dimensions}
	return j
}

// Output declares a named output variable produced by this job.
// The dotenvFile is the path to the file containing KEY=VALUE pairs (used by GitLab dotenv artifacts).
func (j *Job) Output(name, dotenvFile string) *Job {
	j.OutputList = append(j.OutputList, JobOutput{Name: name, DotenvFile: dotenvFile})
	return j
}

// OutputRef returns a platform-neutral placeholder for referencing this job's output.
// Each synthesizer translates it to the platform-specific syntax.
func (j *Job) OutputRef(name string) string {
	return fmt.Sprintf("$PISYN_OUTPUT_%s__%s", strings.ToUpper(strings.ReplaceAll(j.JobName, "-", "_")), strings.ToUpper(name))
}

// helpers

func checkDuplicateJobName(scope *Stage, name string) error {
	pipeline := scope.scope
	if pipeline == nil {
		return nil
	}
	for _, child := range pipeline.children {
		if st, ok := child.node.(*Stage); ok {
			for _, jc := range st.children {
				if j, ok := jc.node.(*Job); ok && j.JobName == name {
					return fmt.Errorf("pisyn: duplicate job name %q in pipeline", name)
				}
			}
		}
	}
	return nil
}

func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}

func cloneMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}

func cloneMapSlice(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	c := make(map[string][]string, len(m))
	for k, v := range m {
		c[k] = cloneStrings(v)
	}
	return c
}

func cloneActions(actions []Action) []Action {
	if actions == nil {
		return nil
	}
	c := make([]Action, len(actions))
	for i, a := range actions {
		c[i] = Action{Phase: a.Phase}
		if a.Script != nil {
			s := *a.Script
			c[i].Script = &s
		}
		if a.Step != nil {
			step := *a.Step
			if step.With != nil {
				step.With = cloneMap(step.With)
			}
			c[i].Step = &step
		}
	}
	return c
}
