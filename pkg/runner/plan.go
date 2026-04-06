package runner

import (
	"fmt"

	"github.com/pipecrew/pisyn/pkg/pisyn"
)

// ExecutionGroup is a set of jobs that can run in parallel.
type ExecutionGroup struct {
	Jobs []*pisyn.Job
}

// ExecutionPlan is an ordered list of groups to execute sequentially.
type ExecutionPlan struct {
	Groups   []ExecutionGroup
	Deps     map[string][]string // resolved dependencies: job name → job names it depends on
	Warnings []string            // non-fatal issues detected during planning
}

// AllJobs returns all jobs in the plan in order.
func (p *ExecutionPlan) AllJobs() []*pisyn.Job {
	var jobs []*pisyn.Job
	for _, group := range p.Groups {
		jobs = append(jobs, group.Jobs...)
	}
	return jobs
}

// RunOpts controls what to execute.
type RunOpts struct {
	Job      string
	Stage    string
	Parallel bool
}

// Plan builds an execution plan from the app's construct tree.
func Plan(app *pisyn.App, opts RunOpts) (*ExecutionPlan, error) {
	var allJobs []*pisyn.Job
	jobByName := map[string]*pisyn.Job{}
	jobStage := map[string]int{} // job name → stage index

	for _, pipeline := range app.Pipelines() {
		for si, stage := range pipeline.Stages() {
			if opts.Stage != "" && stage.Name != opts.Stage {
				continue
			}
			for _, job := range stage.Jobs() {
				if opts.Job != "" && job.JobName != opts.Job {
					continue
				}
				allJobs = append(allJobs, job)
				jobByName[job.JobName] = job
				jobStage[job.JobName] = si
			}
		}
	}

	if len(allJobs) == 0 {
		if opts.Job != "" {
			return nil, fmt.Errorf("job %q not found", opts.Job)
		}
		if opts.Stage != "" {
			return nil, fmt.Errorf("stage %q not found", opts.Stage)
		}
		return nil, fmt.Errorf("no jobs found")
	}

	// Build dependency edges: stage ordering + explicit needs
	deps := map[string][]string{}
	var warnings []string
	for _, job := range allJobs {
		var jobDeps []string
		if !job.EmptyNeeds && len(job.NeedsList) > 0 {
			for _, need := range job.NeedsList {
				if _, ok := jobByName[need]; ok {
					jobDeps = append(jobDeps, need)
				} else {
					warnings = append(warnings, fmt.Sprintf("job %q needs %q which is not in the execution plan — dependency ignored", job.JobName, need))
				}
			}
		} else if !job.EmptyNeeds {
			// Implicit: depends on all jobs in previous stages
			si := jobStage[job.JobName]
			for _, other := range allJobs {
				if jobStage[other.JobName] < si {
					jobDeps = append(jobDeps, other.JobName)
				}
			}
		}
		deps[job.JobName] = jobDeps
	}

	// Topological sort into groups
	done := map[string]bool{}
	var groups []ExecutionGroup

	for len(done) < len(allJobs) {
		var group ExecutionGroup
		for _, job := range allJobs {
			if done[job.JobName] {
				continue
			}
			ready := true
			for _, dep := range deps[job.JobName] {
				if !done[dep] {
					ready = false
					break
				}
			}
			if ready {
				group.Jobs = append(group.Jobs, job)
			}
		}
		if len(group.Jobs) == 0 {
			return nil, fmt.Errorf("circular dependency detected")
		}
		for _, job := range group.Jobs {
			done[job.JobName] = true
		}
		groups = append(groups, group)
	}

	return &ExecutionPlan{Groups: groups, Deps: deps, Warnings: warnings}, nil
}
