package pisyn

// WhenPolicy controls when a job runs.
type WhenPolicy int

const (
	// OnSuccess runs the job only when all previous jobs succeed (default).
	OnSuccess WhenPolicy = iota
	// Manual requires manual trigger to run the job.
	Manual
	// Always runs the job regardless of previous job status.
	Always
	// OnFailure runs the job only when a previous job fails.
	OnFailure
)

// CachePolicy controls cache behavior.
type CachePolicy int

const (
	// PullPush downloads cache at start and uploads at end (default).
	PullPush CachePolicy = iota
	// Pull only downloads cache, does not upload.
	Pull
	// Push only uploads cache, does not download.
	Push
)

// Artifacts configures artifact collection for a job.
type Artifacts struct {
	Paths    []string            `json:"paths,omitempty"`
	Name     string              `json:"name,omitempty"`
	ExpireIn string              `json:"expire_in,omitempty"`
	When     WhenPolicy          `json:"when,omitempty"`
	Reports  map[string][]string `json:"reports,omitempty"`
}

// Cache configures caching for a job.
type Cache struct {
	Key    string      `json:"key"`
	Paths  []string    `json:"paths"`
	Policy CachePolicy `json:"policy,omitempty"`
}

// Matrix configures matrix builds with multiple variable dimensions.
type Matrix struct {
	Dimensions map[string][]string `json:"dimensions,omitempty"`
	Include    []map[string]string `json:"include,omitempty"`
	Exclude    []map[string]string `json:"exclude,omitempty"`
}

// Service defines a service container for a job.
type Service struct {
	Image     string            `json:"image"`
	Alias     string            `json:"alias"`
	Variables map[string]string `json:"variables,omitempty"`
}

// Environment defines a deployment environment.
type Environment struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// JobOutput declares a named output variable produced by a job.
type JobOutput struct {
	Name       string `json:"name"`
	DotenvFile string `json:"dotenv_file"`
}

// ActionPhase indicates when an action runs within a job.
type ActionPhase int

const (
	// PhaseMain is the default phase — the main script.
	PhaseMain ActionPhase = iota
	// PhaseBefore runs before the main script (before_script in GitLab).
	PhaseBefore
	// PhaseAfter runs after the main script, even on failure (after_script in GitLab).
	PhaseAfter
)

// Step represents a pre-built action step (e.g. GitHub Actions `uses:`).
type Step struct {
	Uses string            `json:"uses"`
	With map[string]string `json:"with,omitempty"`
	If   string            `json:"if,omitempty"`
	Name string            `json:"name,omitempty"`
	ID   string            `json:"id,omitempty"`
}

// Action is a single unit of work in a job — either a script block or a step.
type Action struct {
	Script *string     `json:"script,omitempty"`
	Step   *Step       `json:"step,omitempty"`
	Phase  ActionPhase `json:"phase,omitempty"`
}

// Rule represents a conditional rule for job or workflow execution.
type Rule struct {
	If           string            `json:"if,omitempty"`
	When         string            `json:"when,omitempty"`
	AllowFailure bool              `json:"allow_failure,omitempty"`
	Changes      []string          `json:"changes,omitempty"`
	Exists       []string          `json:"exists,omitempty"`
	Variables    map[string]string `json:"variables,omitempty"`
}

// RetryConfig holds retry settings including optional failure conditions.
type RetryConfig struct {
	Max       int      `json:"max"`
	When      []string `json:"when,omitempty"`
	ExitCodes []int    `json:"exit_codes,omitempty"`
}

// AllowFailureConfig holds allow_failure settings including optional exit codes.
type AllowFailureConfig struct {
	Enabled   bool  `json:"enabled"`
	ExitCodes []int `json:"exit_codes,omitempty"`
}

// PushTrigger configures a push event trigger.
type PushTrigger struct {
	Branches  []string `json:"branches,omitempty"`
	Protected bool     `json:"protected,omitempty"`
}

// PRTrigger configures a pull/merge request event trigger.
type PRTrigger struct {
	Branches []string `json:"branches,omitempty"`
}

// ScheduleTrigger configures a cron schedule trigger.
type ScheduleTrigger struct {
	Cron string `json:"cron"`
}

// Triggers holds all pipeline trigger configurations.
type Triggers struct {
	Push        *PushTrigger      `json:"push,omitempty"`
	PullRequest *PRTrigger        `json:"pull_request,omitempty"`
	Schedule    []ScheduleTrigger `json:"schedule,omitempty"`
}

// Include represents a GitLab CI include directive.
// Exactly one of Local, Remote, Project, or Template should be set.
type Include struct {
	Local    string `json:"local,omitempty"`
	Remote   string `json:"remote,omitempty"`
	Project  string `json:"project,omitempty"`
	File     string `json:"file,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Template string `json:"template,omitempty"`
}
