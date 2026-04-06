// Package tekton synthesizes pisyn pipelines to Tekton YAML manifests.
package tekton

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"gopkg.in/yaml.v3"
)

// Synthesizer generates Tekton Task, Pipeline, and PipelineRun manifests.
type Synthesizer struct{}

// NewSynthesizer creates a new Tekton synthesizer.
func NewSynthesizer() *Synthesizer { return &Synthesizer{} }

func init() {
	pisyn.RegisterPlatform("tekton", func() pisyn.Synthesizer { return NewSynthesizer() })
}

// Synth generates Tekton YAML from the app's construct tree.
func (s *Synthesizer) Synth(app *pisyn.App, outDir string) error {
	for _, p := range app.Pipelines() {
		dir := filepath.Join(outDir, "tekton")
		tasks, pipeline, run := s.render(p)

		if err := writeYAMLFile(dir, "tasks.yaml", tasks); err != nil {
			return err
		}
		if err := writeYAMLFile(dir, "pipeline.yaml", []map[string]any{pipeline}); err != nil {
			return err
		}
		if err := writeYAMLFile(dir, "pipeline-run.yaml", []map[string]any{run}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Synthesizer) render(p *pisyn.Pipeline) (tasks []map[string]any, pipeline map[string]any, run map[string]any) {
	pipelineName := sanitize(p.Name)
	var pipelineTasks []map[string]any
	var prevStageTaskNames []string

	for _, st := range p.Stages() {
		var currentTaskNames []string
		for _, j := range st.Jobs() {
			taskName := sanitize(j.JobName)
			tasks = append(tasks, renderTask(taskName, j))

			pt := map[string]any{
				"name": taskName,
				"taskRef": map[string]any{
					"name": taskName,
				},
			}

			// runAfter: explicit needs or implicit stage ordering
			runAfter := make([]string, len(j.NeedsList))
			for i, n := range j.NeedsList {
				runAfter[i] = sanitize(n)
			}
			if len(runAfter) == 0 && len(prevStageTaskNames) > 0 {
				runAfter = prevStageTaskNames
			}
			if len(runAfter) > 0 {
				pt["runAfter"] = runAfter
			}

			pipelineTasks = append(pipelineTasks, pt)
			currentTaskNames = append(currentTaskNames, taskName)
		}
		prevStageTaskNames = currentTaskNames
	}

	pipeline = map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "Pipeline",
		"metadata":   map[string]any{"name": pipelineName},
		"spec": map[string]any{
			"tasks": pipelineTasks,
		},
	}

	run = map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "PipelineRun",
		"metadata":   map[string]any{"generateName": pipelineName + "-run-"},
		"spec": map[string]any{
			"pipelineRef": map[string]any{"name": pipelineName},
		},
	}

	return
}

func renderTask(name string, j *pisyn.Job) map[string]any {
	image := j.ImageName
	if image == "" {
		image = "alpine:latest"
	}

	var steps []map[string]any
	allScripts := concat(j.BeforeScriptLines(), j.ScriptLines(), j.AfterScriptLines())
	if len(allScripts) > 0 {
		steps = append(steps, map[string]any{
			"name":   "run",
			"image":  image,
			"script": "#!/usr/bin/env sh\nset -e\n" + translateVars(strings.Join(allScripts, "\n")),
		})
	}

	task := map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "Task",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"steps": steps,
		},
	}

	return task
}

func sanitize(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}

// translateVars replaces pisyn variables with Tekton param references.
func translateVars(s string) string {
	for k, v := range pisyn.TektonVars {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
		s = strings.ReplaceAll(s, "$"+k, v)
	}
	return s
}

func concat(slices ...[]string) []string {
	var result []string
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

func writeYAMLFile(dir, name string, docs []map[string]any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return err
		}
	}
	return enc.Close()
}
