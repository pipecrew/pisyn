package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/runner"
)

// RunPipeline executes the pipeline locally with a TUI display.
func RunPipeline(parentCtx context.Context, app *pisyn.App, opts runner.RunOpts) error {
	plan, err := runner.Plan(app, opts)
	if err != nil {
		return err
	}

	// Cancellable context: cancelled when TUI exits (ctrl+c / q)
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	events, err := runner.Run(ctx, plan, opts)
	if err != nil {
		return err
	}

	var jobInfos []JobInfo
	for _, job := range plan.AllJobs() {
		jobInfos = append(jobInfos, JobInfo{
			Name:         job.JobName,
			AllowFailure: job.IsAllowedToFail(),
		})
	}

	model := NewModel(jobInfos, events, cancel)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	if m, ok := finalModel.(Model); ok && m.HasFailures() {
		return fmt.Errorf("%d job(s) failed", m.FailedCount())
	}

	return nil
}

// RunPlain executes the pipeline without TUI, printing logs to stdout.
func RunPlain(parentCtx context.Context, app *pisyn.App, opts runner.RunOpts) error {
	plan, err := runner.Plan(app, opts)
	if err != nil {
		return err
	}

	// Build allow-failure lookup
	allowFailure := map[string]bool{}
	for _, job := range plan.AllJobs() {
		if job.IsAllowedToFail() {
			allowFailure[job.JobName] = true
		}
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	events, err := runner.Run(ctx, plan, opts)
	if err != nil {
		return err
	}

	failed := 0
	for ev := range events {
		switch ev.Type {
		case runner.EventJobStarted:
			fmt.Printf("▶ %s\n", ev.JobName)
		case runner.EventJobLog:
			fmt.Printf("  %s\n", ev.Log)
		case runner.EventJobFinished:
			switch ev.Status {
			case runner.StatusPassed:
				fmt.Printf("✅ %s (%s)\n", ev.JobName, ev.Elapsed.Truncate(100*time.Millisecond))
			case runner.StatusFailed:
				if allowFailure[ev.JobName] {
					fmt.Printf("⚠️  %s (%s): %v (allowed to fail)\n", ev.JobName, ev.Elapsed.Truncate(100*time.Millisecond), ev.Err)
				} else {
					fmt.Printf("❌ %s (%s): %v\n", ev.JobName, ev.Elapsed.Truncate(100*time.Millisecond), ev.Err)
					failed++
				}
			case runner.StatusSkipped:
				fmt.Printf("⏭  %s (skipped)\n", ev.JobName)
			}
		case runner.EventRunComplete:
			if ev.Err != nil {
				return ev.Err
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d job(s) failed", failed)
	}
	return nil
}
