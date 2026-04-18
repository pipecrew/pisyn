package pisyn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// IR types — plain data structs for JSON serialization.
// These mirror the construct tree but without Construct embedding or methods.

// IRSchemaVersion is the current schema version of pipeline.json.
// Bump only on breaking changes (field renames, structural changes).
// Adding new optional fields is NOT a version bump.
const IRSchemaVersion = 1

// IRApp is the JSON representation of an App.
type IRApp struct {
	Version   int          `json:"version"`
	Pipelines []IRPipeline `json:"pipelines"`
}

// IRPipeline is the JSON representation of a Pipeline.
type IRPipeline struct {
	Name          string            `json:"name"`
	Env           map[string]string `json:"env,omitempty"`
	Triggers      Triggers          `json:"triggers,omitempty"`
	WorkflowRules []Rule            `json:"workflow_rules,omitempty"`
	Includes      []Include         `json:"includes,omitempty"`
	Defaults      *JobDefaults      `json:"defaults,omitempty"`
	Stages        []IRStage         `json:"stages"`
}

// IRStage is the JSON representation of a Stage.
type IRStage struct {
	Name string  `json:"name"`
	Jobs []IRJob `json:"jobs"`
}

// IRJob is the JSON representation of a Job.
type IRJob struct {
	Name            string              `json:"name"`
	Image           string              `json:"image,omitempty"`
	ImageEntrypoint []string            `json:"image_entrypoint,omitempty"`
	ImageUser       string              `json:"image_user,omitempty"`
	Actions         []Action            `json:"actions,omitempty"`
	Needs           []string            `json:"needs,omitempty"`
	EmptyNeeds      bool                `json:"empty_needs,omitempty"`
	Dependencies    []string            `json:"dependencies,omitempty"`
	Runner          string              `json:"runner,omitempty"`
	Services        []Service           `json:"services,omitempty"`
	Env             map[string]string   `json:"env,omitempty"`
	Artifacts       *Artifacts          `json:"artifacts,omitempty"`
	Cache           *Cache              `json:"cache,omitempty"`
	Matrix          *Matrix             `json:"matrix,omitempty"`
	Environment     *Environment        `json:"environment,omitempty"`
	AllowFailure    bool                `json:"allow_failure,omitempty"`
	AllowFailureCfg *AllowFailureConfig `json:"allow_failure_config,omitempty"`
	TimeoutMin      int                 `json:"timeout_min,omitempty"`
	RetryCount      int                 `json:"retry_count,omitempty"`
	RetryConfig     *RetryConfig        `json:"retry_config,omitempty"`
	When            WhenPolicy          `json:"when,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	Rules           []Rule              `json:"rules,omitempty"`
	Interruptible   *bool               `json:"interruptible,omitempty"`
	Outputs         []JobOutput         `json:"outputs,omitempty"`
	FetchDepth      int                 `json:"fetch_depth"`
}

// Build serializes the App's construct tree to pipeline.json in the given directory.
func (a *App) Build(outDir string) error {
	if err := a.checkDuplicateJobNames(); err != nil {
		return err
	}
	ir := a.ToIR()
	data, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pipeline: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	path := filepath.Join(outDir, "pipeline.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	for _, pipeline := range ir.Pipelines {
		fmt.Printf("✅ Built pipeline %q → %s\n", pipeline.Name, path)
	}
	return nil
}

// ToIR converts the live construct tree to IR structs.
func (a *App) ToIR() *IRApp {
	ir := &IRApp{Version: IRSchemaVersion}
	for _, pipeline := range a.Pipelines() {
		irp := IRPipeline{
			Name:          pipeline.Name,
			Env:           pipeline.Env,
			Triggers:      pipeline.On,
			WorkflowRules: pipeline.WorkflowRules,
			Includes:      pipeline.IncludeList,
			Defaults:      pipeline.Defaults,
		}
		for _, stage := range pipeline.Stages() {
			irs := IRStage{Name: stage.Name}
			for _, job := range stage.Jobs() {
				irs.Jobs = append(irs.Jobs, jobToIR(job))
			}
			irp.Stages = append(irp.Stages, irs)
		}
		ir.Pipelines = append(ir.Pipelines, irp)
	}
	return ir
}

func jobToIR(job *Job) IRJob {
	return IRJob{
		Name:            job.JobName,
		Image:           job.ImageName,
		ImageEntrypoint: job.ImageEP,
		ImageUser:       job.ImageUsr,
		Actions:         job.Actions,
		Needs:           job.NeedsList,
		EmptyNeeds:      job.EmptyNeeds,
		Dependencies:    job.DependencyList,
		Runner:          job.Runner,
		Services:        job.ServiceList,
		Env:             job.EnvVars,
		Artifacts:       job.ArtifactsCfg,
		Cache:           job.CacheCfg,
		Matrix:          job.MatrixCfg,
		Environment:     job.EnvironmentCfg,
		AllowFailure:    job.IsAllowFailure,
		AllowFailureCfg: job.AllowFailureCfg,
		TimeoutMin:      job.TimeoutMin,
		RetryCount:      job.RetryCount,
		RetryConfig:     job.RetryCfg,
		When:            job.When,
		Tags:            job.Tags,
		Rules:           job.Rules,
		Interruptible:   job.Interruptible,
		Outputs:         job.OutputList,
		FetchDepth:      job.FetchDepth,
	}
}

// LoadIR reads pipeline.json and returns IR structs.
func LoadIR(path string) (*IRApp, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var ir IRApp
	if err := json.Unmarshal(data, &ir); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Version 0 means the field was absent — treat as version 1 (pre-versioning files)
	if ir.Version == 0 {
		ir.Version = 1
	}
	if ir.Version > IRSchemaVersion {
		return nil, fmt.Errorf("%s was created by a newer pisyn version (schema v%d, this CLI supports v%d) — please upgrade pisyn", path, ir.Version, IRSchemaVersion)
	}
	return &ir, nil
}

// ToApp converts IR structs back to a live construct tree.
func (ir *IRApp) ToApp() *App {
	app := &App{OutDir: "pisyn.out"}
	app.Construct = newConstruct(nil, "App", app)

	for _, irp := range ir.Pipelines {
		pipeline := &Pipeline{
			Name:          irp.Name,
			Env:           irp.Env,
			On:            irp.Triggers,
			WorkflowRules: irp.WorkflowRules,
			IncludeList:   irp.Includes,
			Defaults:      irp.Defaults,
		}
		if pipeline.Env == nil {
			pipeline.Env = map[string]string{}
		}
		pipeline.Construct = newConstruct(&app.Construct, irp.Name, pipeline)

		for _, irs := range irp.Stages {
			stage := &Stage{Name: irs.Name}
			stage.Construct = newConstruct(&pipeline.Construct, irs.Name, stage)

			for _, irj := range irs.Jobs {
				job := irJobToJob(irj)
				job.Construct = newConstruct(&stage.Construct, irj.Name, job)
			}
		}
	}
	return app
}

func irJobToJob(irj IRJob) *Job {
	env := irj.Env
	if env == nil {
		env = map[string]string{}
	}
	return &Job{
		JobName:         irj.Name,
		ImageName:       irj.Image,
		ImageEP:         irj.ImageEntrypoint,
		ImageUsr:        irj.ImageUser,
		Actions:         irj.Actions,
		NeedsList:       irj.Needs,
		EmptyNeeds:      irj.EmptyNeeds,
		DependencyList:  irj.Dependencies,
		Runner:          irj.Runner,
		ServiceList:     irj.Services,
		EnvVars:         env,
		ArtifactsCfg:    irj.Artifacts,
		CacheCfg:        irj.Cache,
		MatrixCfg:       irj.Matrix,
		EnvironmentCfg:  irj.Environment,
		IsAllowFailure:  irj.AllowFailure,
		AllowFailureCfg: irj.AllowFailureCfg,
		TimeoutMin:      irj.TimeoutMin,
		RetryCount:      irj.RetryCount,
		RetryCfg:        irj.RetryConfig,
		When:            irj.When,
		Tags:            irj.Tags,
		Rules:           irj.Rules,
		Interruptible:   irj.Interruptible,
		OutputList:      irj.Outputs,
		FetchDepth:      irj.FetchDepth,
	}
}
