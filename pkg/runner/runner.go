package runner

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pipecrew/pisyn/pkg/pisyn"
)

// JobStatus represents the current state of a job.
type JobStatus int

const (
	StatusPending JobStatus = iota
	StatusRunning
	StatusPassed
	StatusFailed
	StatusSkipped
)

// EventType identifies the kind of runner event.
type EventType int

const (
	EventJobStarted EventType = iota
	EventJobLog
	EventJobFinished
	EventRunComplete
)

// Event is emitted by the runner to the TUI.
type Event struct {
	Type    EventType
	JobName string
	Status  JobStatus
	Log     string
	Err     error
	Elapsed time.Duration
}

// Run executes the pipeline locally in Docker containers.
// Accepts a pre-computed plan to avoid duplicate computation.
// The caller should cancel ctx to stop execution (e.g. on ctrl+c).
func Run(ctx context.Context, plan *ExecutionPlan, opts RunOpts) (chan Event, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	docker, err := NewDockerRunner(wd)
	if err != nil {
		return nil, err
	}

	events := make(chan Event, 100)

	go func() {
		defer close(events)
		defer docker.Close()

		netName := fmt.Sprintf("pisyn-run-%d", time.Now().UnixNano())
		if err := docker.CreateNetwork(ctx, netName); err != nil {
			events <- Event{Type: EventRunComplete, Err: err}
			return
		}
		defer docker.RemoveNetwork(context.Background())

		// Copy project into isolated workspace volume
		wsName := fmt.Sprintf("pisyn-workspace-%d", time.Now().UnixNano())
		events <- Event{Type: EventJobLog, JobName: "", Log: "Preparing workspace..."}
		if err := docker.CreateWorkspace(ctx, wsName); err != nil {
			events <- Event{Type: EventRunComplete, Err: err}
			return
		}
		defer docker.RemoveWorkspace(context.Background())

		// Emit plan warnings
		for _, warning := range plan.Warnings {
			events <- Event{Type: EventJobLog, JobName: "", Log: "⚠️  " + warning}
		}

		var mu sync.Mutex
		failed := map[string]bool{}
		outputs := map[string]string{} // collected job outputs: PISYN_OUTPUT_X__Y → value

		for _, group := range plan.Groups {
			if ctx.Err() != nil {
				break
			}
			if opts.Parallel && len(group.Jobs) > 1 {
				runParallel(ctx, docker, plan, group.Jobs, events, failed, outputs, &mu)
			} else {
				for _, job := range group.Jobs {
					if ctx.Err() != nil {
						break
					}
					runSingleJob(ctx, docker, plan, job, events, failed, outputs, &mu)
				}
			}
		}

		events <- Event{Type: EventRunComplete}
	}()

	return events, nil
}

func runSingleJob(ctx context.Context, docker *DockerRunner, plan *ExecutionPlan, job *pisyn.Job, events chan<- Event, failed map[string]bool, outputs map[string]string, mu *sync.Mutex) {
	// Check if any resolved dependency failed
	mu.Lock()
	skip := false
	for _, dep := range plan.Deps[job.JobName] {
		if failed[dep] {
			skip = true
			break
		}
	}
	mu.Unlock()

	if skip {
		events <- Event{Type: EventJobFinished, JobName: job.JobName, Status: StatusSkipped}
		mu.Lock()
		failed[job.JobName] = true
		mu.Unlock()
		return
	}

	// Warn about unsupported features
	for _, action := range job.Actions {
		if action.Step != nil {
			events <- Event{Type: EventJobLog, JobName: job.JobName, Log: fmt.Sprintf("⚠️  skipping step: %s (uses: not supported locally)", action.Step.Uses)}
		}
	}
	if job.MatrixCfg != nil {
		events <- Event{Type: EventJobLog, JobName: job.JobName, Log: "⚠️  matrix builds not supported locally, running default configuration"}
	}

	start := time.Now()
	events <- Event{Type: EventJobStarted, JobName: job.JobName, Status: StatusRunning}

	mu.Lock()
	extraEnv := make(map[string]string)
	for key, val := range outputs {
		extraEnv[key] = val
	}
	mu.Unlock()

	err := docker.RunJob(ctx, job, extraEnv, events)
	elapsed := time.Since(start)

	if err != nil {
		events <- Event{Type: EventJobFinished, JobName: job.JobName, Status: StatusFailed, Err: err, Elapsed: elapsed}
		if !job.IsAllowedToFail() {
			mu.Lock()
			failed[job.JobName] = true
			mu.Unlock()
		}
	} else {
		// Collect outputs from dotenv files
		if len(job.OutputList) > 0 {
			collected := docker.CollectOutputs(ctx, job)
			mu.Lock()
			for key, val := range collected {
				outputs[key] = val
			}
			mu.Unlock()
		}
		events <- Event{Type: EventJobFinished, JobName: job.JobName, Status: StatusPassed, Elapsed: elapsed}
	}
}

func runParallel(ctx context.Context, docker *DockerRunner, plan *ExecutionPlan, jobs []*pisyn.Job, events chan<- Event, failed map[string]bool, outputs map[string]string, mu *sync.Mutex) {
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j *pisyn.Job) {
			defer wg.Done()
			runSingleJob(ctx, docker, plan, j, events, failed, outputs, mu)
		}(job)
	}
	wg.Wait()
}

// RunOptsFromEnv reads run options from environment variables.
func RunOptsFromEnv() RunOpts {
	return RunOpts{
		Job:      os.Getenv("PISYN_RUN_JOB"),
		Stage:    os.Getenv("PISYN_RUN_STAGE"),
		Parallel: os.Getenv("PISYN_RUN_PARALLEL") == "true",
	}
}
