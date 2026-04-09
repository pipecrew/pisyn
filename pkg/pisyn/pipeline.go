package pisyn

// Pipeline is a named CI/CD pipeline within an App.
type Pipeline struct {
	Construct
	Name          string
	Env           map[string]string
	On            Triggers
	WorkflowRules []Rule
	IncludeList   []Include
	Defaults      *JobDefaults
}

// JobDefaults defines shared defaults for all jobs in a pipeline.
type JobDefaults struct {
	Image        string   `json:"image,omitempty"`
	BeforeScript []string `json:"before_script,omitempty"`
	AfterScript  []string `json:"after_script,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// NewPipeline creates a new pipeline in the given app.
func NewPipeline(scope *App, name string) *Pipeline {
	p := &Pipeline{Name: name, Env: map[string]string{}}
	p.Construct = newConstruct(&scope.Construct, name, p)
	return p
}

// SetEnv sets a pipeline-level environment variable.
func (p *Pipeline) SetEnv(key, value string) *Pipeline {
	p.Env[key] = value
	return p
}

// OnPush adds a push trigger for the given branches.
func (p *Pipeline) OnPush(branches ...string) *Pipeline {
	if p.On.Push == nil {
		p.On.Push = &PushTrigger{}
	}
	p.On.Push.Branches = branches
	return p
}

// OnPushProtected adds a push trigger restricted to protected branches.
func (p *Pipeline) OnPushProtected() *Pipeline {
	if p.On.Push == nil {
		p.On.Push = &PushTrigger{}
	}
	p.On.Push.Protected = true
	return p
}

// OnPushTag adds a push trigger for the given tag patterns (e.g. "v*").
func (p *Pipeline) OnPushTag(patterns ...string) *Pipeline {
	if p.On.Push == nil {
		p.On.Push = &PushTrigger{}
	}
	p.On.Push.Tags = patterns
	return p
}

// OnPR adds a pull/merge request trigger for the given target branches.
func (p *Pipeline) OnPR(branches ...string) *Pipeline {
	p.On.PullRequest = &PRTrigger{Branches: branches}
	return p
}

// OnMR is an alias for OnPR (GitLab terminology).
func (p *Pipeline) OnMR(branches ...string) *Pipeline {
	p.On.PullRequest = &PRTrigger{Branches: branches}
	return p
}

// OnSchedule adds a cron schedule trigger.
func (p *Pipeline) OnSchedule(cron string) *Pipeline {
	p.On.Schedule = append(p.On.Schedule, ScheduleTrigger{Cron: cron})
	return p
}

// AddWorkflowRule adds a pipeline-level workflow rule (e.g. GitLab workflow:rules).
func (p *Pipeline) AddWorkflowRule(r Rule) *Pipeline {
	p.WorkflowRules = append(p.WorkflowRules, r)
	return p
}

// IncludeLocal adds a local file include (path relative to repo root).
func (p *Pipeline) IncludeLocal(path string) *Pipeline {
	p.IncludeList = append(p.IncludeList, Include{Local: path})
	return p
}

// IncludeRemote adds a remote URL include.
func (p *Pipeline) IncludeRemote(url string) *Pipeline {
	p.IncludeList = append(p.IncludeList, Include{Remote: url})
	return p
}

// IncludeProject adds a project file include with optional ref.
func (p *Pipeline) IncludeProject(project, file, ref string) *Pipeline {
	p.IncludeList = append(p.IncludeList, Include{Project: project, File: file, Ref: ref})
	return p
}

// IncludeTemplate adds a GitLab-provided template include.
func (p *Pipeline) IncludeTemplate(name string) *Pipeline {
	p.IncludeList = append(p.IncludeList, Include{Template: name})
	return p
}

// SetDefault sets shared defaults for all jobs in this pipeline.
func (p *Pipeline) SetDefault(d JobDefaults) *Pipeline {
	p.Defaults = &d
	return p
}

// Stages returns all stages in this pipeline.
func (p *Pipeline) Stages() []*Stage {
	return childrenOfType[Stage](&p.Construct)
}
