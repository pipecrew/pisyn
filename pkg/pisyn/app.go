// Package pisyn provides a Go DSL for defining CI/CD pipelines and synthesizing
// them to platform-specific YAML (GitLab CI, GitHub Actions, Tekton).
package pisyn

import (
	"fmt"
	"os"
	"strings"
)

// Synthesizer transforms a construct tree into platform-specific output files.
type Synthesizer interface {
	Synth(app *App, outDir string) error
}

// SynthesizerFactory creates a synthesizer for a given platform name.
type SynthesizerFactory func() Synthesizer

// registry maps platform names to synthesizer factories.
var registry = map[string]SynthesizerFactory{}

// RegisterPlatform registers a synthesizer factory for a platform name (e.g. "gitlab").
func RegisterPlatform(name string, factory SynthesizerFactory) {
	registry[strings.ToLower(name)] = factory
}

// App is the root of the construct tree.
type App struct {
	Construct
	OutDir string
}

// NewApp creates a new App with default settings. Reads PISYN_OUT_DIR env var if set.
func NewApp() *App {
	a := &App{OutDir: "pisyn.out"}
	if d := os.Getenv("PISYN_OUT_DIR"); d != "" {
		a.OutDir = d
	}
	a.Construct = newConstruct(nil, "App", a)
	return a
}

// Pipelines returns all pipelines in this app.
func (a *App) Pipelines() []*Pipeline {
	return childrenOfType[Pipeline](&a.Construct)
}

// Synth synthesizes the app using the given synthesizer.
func (a *App) Synth(s Synthesizer) error {
	return s.Synth(a, a.OutDir)
}

// Run synthesizes the app. If PISYN_PLATFORM is set, only those platforms are
// synthesized. Otherwise all registered platforms are used.
// If PISYN_MODE=build, writes pipeline.json and exits.
// If PISYN_MODE=graph, outputs a Mermaid diagram instead of synthesizing.
func (a *App) Run() error {
	if os.Getenv("PISYN_MODE") == "build" {
		outDir := a.OutDir
		if d := os.Getenv("PISYN_OUT_DIR"); d != "" {
			outDir = d
		}
		return a.Build(outDir)
	}

	if os.Getenv("PISYN_MODE") == "graph" {
		fmt.Print(a.Graph())
		return nil
	}

	// Default: build + synth
	if err := a.Build(a.OutDir); err != nil {
		return err
	}

	platforms := platformsFromEnv()
	if len(platforms) == 0 {
		for name := range registry {
			platforms = append(platforms, name)
		}
	}
	if len(platforms) == 0 {
		return fmt.Errorf("no platforms registered; call pisyn.RegisterPlatform() or set PISYN_PLATFORM")
	}
	for _, p := range platforms {
		factory, ok := registry[strings.ToLower(p)]
		if !ok {
			return fmt.Errorf("⚠️ unknown platform: %s", p)
		}
		if err := a.Synth(factory()); err != nil {
			return fmt.Errorf("❌ synth %s: %w", p, err)
		}
		fmt.Printf("⚗️ %s synthesized → %s\n", p, a.OutDir)
	}
	return nil
}

// Graph returns a Mermaid flowchart of the pipeline's job dependency graph.
func (a *App) Graph() string {
	var b strings.Builder
	var edges strings.Builder
	b.WriteString("graph LR\n")
	for _, pipeline := range a.Pipelines() {
		var prevStageJobs []string
		for _, stage := range pipeline.Stages() {
			var currentJobs []string
			fmt.Fprintf(&b, "    subgraph %s[%q]\n", sanitizeID(stage.Name), stage.Name)
			for _, job := range stage.Jobs() {
				id := sanitizeID(job.JobName)
				fmt.Fprintf(&b, "        %s[%q]\n", id, job.JobName)
				if len(job.NeedsList) > 0 {
					for _, need := range job.NeedsList {
						fmt.Fprintf(&edges, "    %s --> %s\n", sanitizeID(need), id)
					}
				} else if len(prevStageJobs) > 0 {
					for _, prev := range prevStageJobs {
						fmt.Fprintf(&edges, "    %s --> %s\n", prev, id)
					}
				}
				currentJobs = append(currentJobs, id)
			}
			b.WriteString("    end\n")
			prevStageJobs = currentJobs
		}
	}
	b.WriteString(edges.String())
	return b.String()
}

func sanitizeID(name string) string {
	r := strings.NewReplacer("-", "_", " ", "_", ".", "_")
	return r.Replace(name)
}

func platformsFromEnv() []string {
	v := os.Getenv("PISYN_PLATFORM")
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) checkDuplicateJobNames() error {
	for _, p := range a.Pipelines() {
		seen := map[string]bool{}
		for _, st := range p.Stages() {
			for _, j := range st.Jobs() {
				if seen[j.JobName] {
					return fmt.Errorf("pisyn: duplicate job name %q in pipeline %q", j.JobName, p.Name)
				}
				seen[j.JobName] = true
			}
		}
	}
	return nil
}
