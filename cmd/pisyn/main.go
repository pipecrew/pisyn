// Package main provides the pisyn CLI for synthesizing CI/CD pipelines.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pipecrew/pisyn/pkg/importer"
	importgitlab "github.com/pipecrew/pisyn/pkg/importer/gitlab"
	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/runner"
	"github.com/pipecrew/pisyn/pkg/tui"
	"github.com/pipecrew/pisyn/pkg/validate"
	"github.com/spf13/cobra"
)

// pisynVersion is set via -ldflags at build time. Falls back to "dev".
var pisynVersion = "dev"

func main() {
	root := &cobra.Command{
		Use:           "pisyn",
		Short:         "⚗️ pisyn — the pipeline synthesizer. Define CI/CD pipelines in Go, synthesize to GitLab CI, GitHub Actions, or Tekton.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	build := &cobra.Command{
		Use:   "build",
		Short: "Build pipeline.json from your Go pipeline definition",
		RunE:  runBuild,
	}
	build.Flags().StringP("app", "a", ".", "path to the Go package containing the pipeline definition")
	build.Flags().StringP("out-dir", "o", "pisyn.out", "output directory")
	root.AddCommand(build)

	synth := &cobra.Command{
		Use:   "synth",
		Short: "Synthesize pipeline output for the specified platform(s)",
		RunE:  runSynth,
	}
	synth.Flags().StringP("platform", "p", "", "comma-separated platforms: gitlab,github,tekton (default: all)")
	synth.Flags().StringP("app", "a", ".", "path to the Go package containing the pipeline definition")
	synth.Flags().StringP("out-dir", "o", "", "override output directory (default: pisyn.out)")
	synth.Flags().Bool("validate", false, "validate generated output against platform schemas after synthesis")
	root.AddCommand(synth)

	val := &cobra.Command{
		Use:   "validate",
		Short: "Validate generated YAML against official CI platform schemas",
		RunE:  runValidate,
	}
	val.Flags().StringP("platform", "p", "", "comma-separated platforms to validate: gitlab,github")
	val.Flags().StringP("out-dir", "o", "pisyn.out", "directory containing generated output")
	root.AddCommand(val)

	graph := &cobra.Command{
		Use:   "graph",
		Short: "Output a Mermaid diagram of the pipeline's job dependency graph",
		RunE:  runGraph,
	}
	graph.Flags().StringP("app", "a", "", "path to the Go package containing the pipeline definition (triggers rebuild)")
	graph.Flags().StringP("out-dir", "o", "pisyn.out", "directory containing pipeline.json")
	graph.Flags().StringP("name", "n", "", "pipeline name from existing pipeline.json (skips rebuild)")
	graph.Flags().StringP("output", "O", "", "render to image file via mmdc (e.g. graph.png, graph.svg)")
	root.AddCommand(graph)

	run := &cobra.Command{
		Use:   "run",
		Short: "Execute pipeline locally in Docker containers with a live TUI",
		RunE:  runLocal,
	}
	run.Flags().StringP("app", "a", "", "path to the Go package containing the pipeline definition (triggers rebuild)")
	run.Flags().StringP("out-dir", "o", "pisyn.out", "directory containing pipeline.json")
	run.Flags().StringP("name", "n", "", "pipeline name to run from existing pipeline.json (skips rebuild)")
	run.Flags().String("job", "", "run only this job")
	run.Flags().String("stage", "", "run only jobs in this stage")
	run.Flags().Bool("parallel", false, "run independent jobs concurrently")
	run.Flags().Bool("no-tui", false, "disable TUI, print logs to stdout")
	root.AddCommand(run)

	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what would change vs. existing output without writing files",
		RunE:  runDiff,
	}
	diff.Flags().StringP("platform", "p", "", "comma-separated platforms: gitlab,github,tekton (default: all)")
	diff.Flags().StringP("app", "a", ".", "path to the Go package containing the pipeline definition")
	diff.Flags().StringP("out-dir", "o", "", "override output directory (default: pisyn.out)")
	root.AddCommand(diff)

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a pisyn pipeline from an existing CI config file",
		RunE:  runInit,
	}
	initCmd.Flags().String("from", "", "path to existing CI config (e.g. .gitlab-ci.yml)")
	initCmd.Flags().StringP("output", "o", "", "output file path (default: stdout)")
	_ = initCmd.MarkFlagRequired("from")
	root.AddCommand(initCmd)

	version := &cobra.Command{
		Use:   "version",
		Short: "Print pisyn version and schema version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pisyn %s (schema v%d)\n", pisynVersion, pisyn.IRSchemaVersion)
		},
	}
	root.AddCommand(version)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// buildPipeline runs `go run <app>` with PISYN_MODE=build to produce pipeline.json.
func buildPipeline(appPath, outDir string) error {
	goRun := exec.Command("go", "run", appPath)
	goRun.Stdout = os.Stdout
	goRun.Stderr = os.Stderr
	goRun.Env = append(os.Environ(), "PISYN_MODE=build", "PISYN_OUT_DIR="+outDir)
	if err := goRun.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

// loadApp loads a pipeline construct tree. Two modes:
//   - appPath set: rebuild pipeline.json from Go code, then load
//   - appPath empty: load existing pipeline.json (optionally filter by name)
func loadApp(appPath, outDir, name string) (*pisyn.App, error) {
	jsonPath := filepath.Join(outDir, "pipeline.json")

	if appPath != "" {
		// Rebuild from source
		if err := buildPipeline(appPath, outDir); err != nil {
			return nil, err
		}
	} else if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no pipeline.json found in %s — run 'pisyn build' first or pass --app", outDir)
	}

	ir, err := pisyn.LoadIR(jsonPath)
	if err != nil {
		return nil, err
	}

	if name != "" {
		found := false
		for _, pipeline := range ir.Pipelines {
			if pipeline.Name == name {
				found = true
				break
			}
		}
		if !found {
			var names []string
			for _, pipeline := range ir.Pipelines {
				names = append(names, pipeline.Name)
			}
			return nil, fmt.Errorf("pipeline %q not found in pipeline.json (available: %s)", name, strings.Join(names, ", "))
		}
	}

	app := ir.ToApp()
	app.OutDir = outDir
	return app, nil
}

func runBuild(cmd *cobra.Command, _ []string) error {
	appPath, _ := cmd.Flags().GetString("app")
	outDir, _ := cmd.Flags().GetString("out-dir")
	return buildPipeline(appPath, outDir)
}

func runSynth(cmd *cobra.Command, _ []string) error {
	platform, _ := cmd.Flags().GetString("platform")
	appPath, _ := cmd.Flags().GetString("app")
	outDir, _ := cmd.Flags().GetString("out-dir")
	doValidate, _ := cmd.Flags().GetBool("validate")

	if outDir == "" {
		outDir = "pisyn.out"
	}

	// Synth still runs user code directly (it needs platform registration via init imports)
	goRun := exec.Command("go", "run", appPath)
	goRun.Stdout = os.Stdout
	goRun.Stderr = os.Stderr
	var env []string
	if platform != "" {
		env = append(env, "PISYN_PLATFORM="+strings.TrimSpace(platform))
	}
	env = append(env, "PISYN_OUT_DIR="+outDir)
	goRun.Env = append(os.Environ(), env...)

	if err := goRun.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", appPath, err)
	}

	if doValidate {
		return validateDir(outDir, platform)
	}
	return nil
}

func runGraph(cmd *cobra.Command, _ []string) error {
	appPath, _ := cmd.Flags().GetString("app")
	outDir, _ := cmd.Flags().GetString("out-dir")
	name, _ := cmd.Flags().GetString("name")
	output, _ := cmd.Flags().GetString("output")

	app, err := loadApp(appPath, outDir, name)
	if err != nil {
		return err
	}

	mermaid := app.Graph()

	if output == "" {
		fmt.Print(mermaid)
		return nil
	}

	if _, err := exec.LookPath("mmdc"); err != nil {
		return fmt.Errorf("mmdc not found — install with: npm install -g @mermaid-js/mermaid-cli")
	}
	mmdc := exec.Command("mmdc", "-i", "/dev/stdin", "-o", output)
	mmdc.Stdin = strings.NewReader(mermaid)
	mmdc.Stdout = os.Stdout
	mmdc.Stderr = os.Stderr
	if err := mmdc.Run(); err != nil {
		return fmt.Errorf("mmdc failed: %w", err)
	}
	fmt.Printf("✅ graph rendered → %s\n", output)
	return nil
}

func runLocal(cmd *cobra.Command, _ []string) error {
	appPath, _ := cmd.Flags().GetString("app")
	outDir, _ := cmd.Flags().GetString("out-dir")
	name, _ := cmd.Flags().GetString("name")
	job, _ := cmd.Flags().GetString("job")
	stage, _ := cmd.Flags().GetString("stage")
	parallel, _ := cmd.Flags().GetBool("parallel")
	noTUI, _ := cmd.Flags().GetBool("no-tui")

	app, err := loadApp(appPath, outDir, name)
	if err != nil {
		return err
	}

	opts := runner.RunOpts{Job: job, Stage: stage, Parallel: parallel}
	ctx := context.Background()

	if noTUI {
		return tui.RunPlain(ctx, app, opts)
	}
	return tui.RunPipeline(ctx, app, opts)
}

var platformFiles = map[string]string{
	"gitlab": ".gitlab-ci.yml",
	"github": ".github/workflows",
}

func runValidate(cmd *cobra.Command, _ []string) error {
	outDir, _ := cmd.Flags().GetString("out-dir")
	platformFlag, _ := cmd.Flags().GetString("platform")
	return validateDir(outDir, platformFlag)
}

func validateDir(outDir, platformFlag string) error {
	platforms := []string{"gitlab", "github"}
	if platformFlag != "" {
		platforms = nil
		for _, plat := range strings.Split(platformFlag, ",") {
			platforms = append(platforms, strings.TrimSpace(plat))
		}
	}

	hasErrors := false
	for _, platform := range platforms {
		pattern, ok := platformFiles[platform]
		if !ok {
			fmt.Fprintf(os.Stderr, "⚠️  no schema available for %s, skipping\n", platform)
			continue
		}

		var files []string
		if platform == "github" {
			dir := filepath.Join(outDir, pattern)
			entries, err := os.ReadDir(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  %s: no output found at %s\n", platform, dir)
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
					files = append(files, filepath.Join(dir, entry.Name()))
				}
			}
		} else {
			filePath := filepath.Join(outDir, pattern)
			if _, err := os.Stat(filePath); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  %s: no output found at %s\n", platform, filePath)
				continue
			}
			files = []string{filePath}
		}

		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read %s: %w", file, err)
			}
			if err := validate.Validate(platform, data); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %s: %s\n", file, err)
				hasErrors = true
			} else {
				fmt.Printf("✅ %s: valid\n", file)
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}

func runDiff(cmd *cobra.Command, _ []string) error {
	platform, _ := cmd.Flags().GetString("platform")
	appPath, _ := cmd.Flags().GetString("app")
	outDir, _ := cmd.Flags().GetString("out-dir")
	if outDir == "" {
		outDir = "pisyn.out"
	}

	// Synth to temp dir
	tmpDir, err := os.MkdirTemp("", "pisyn-diff-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // best-effort cleanup

	goRun := exec.Command("go", "run", appPath)
	goRun.Stderr = os.Stderr
	var env []string
	if platform != "" {
		env = append(env, "PISYN_PLATFORM="+strings.TrimSpace(platform))
	}
	env = append(env, "PISYN_OUT_DIR="+tmpDir)
	goRun.Env = append(os.Environ(), env...)
	if err := goRun.Run(); err != nil {
		return fmt.Errorf("synth failed: %w", err)
	}

	// Walk temp dir and diff against existing output
	hasChanges := false
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmpDir, path)
		existing := filepath.Join(outDir, rel)

		newData, _ := os.ReadFile(path)
		oldData, readErr := os.ReadFile(existing)

		if readErr != nil {
			// New file
			fmt.Printf("\033[32m+ %s (new file)\033[0m\n", rel)
			hasChanges = true
			return nil
		}

		if string(oldData) == string(newData) {
			return nil // no change
		}

		hasChanges = true
		fmt.Printf("\033[33m~ %s\033[0m\n", rel)

		// Line-by-line diff
		oldLines := strings.Split(string(oldData), "\n")
		newLines := strings.Split(string(newData), "\n")
		maxLines := len(oldLines)
		if len(newLines) > maxLines {
			maxLines = len(newLines)
		}
		for i := 0; i < maxLines; i++ {
			old, new_ := "", ""
			if i < len(oldLines) {
				old = oldLines[i]
			}
			if i < len(newLines) {
				new_ = newLines[i]
			}
			if old != new_ {
				if old != "" {
					fmt.Printf("  \033[31m- %s\033[0m\n", old)
				}
				if new_ != "" {
					fmt.Printf("  \033[32m+ %s\033[0m\n", new_)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !hasChanges {
		fmt.Println("✅ no changes")
		return nil
	}
	os.Exit(1)
	return nil
}

// maxInitInputSize is the maximum file size accepted by pisyn init (10 MB).
const maxInitInputSize = 10 * 1024 * 1024

func runInit(cmd *cobra.Command, _ []string) error {
	from, _ := cmd.Flags().GetString("from")
	output, _ := cmd.Flags().GetString("output")

	platform, err := importer.DetectPlatform(from)
	if err != nil {
		return err
	}

	// #7: check file size before reading into memory
	info, err := os.Stat(from)
	if err != nil {
		return fmt.Errorf("stat %s: %w", from, err)
	}
	if info.Size() > maxInitInputSize {
		return fmt.Errorf("input file %s is too large (%d bytes, max %d)", from, info.Size(), maxInitInputSize)
	}

	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("read %s: %w", from, err)
	}

	// Fall back to content-based detection if path wasn't conclusive
	if platform == "" {
		platform = importer.DetectPlatformFromContent(data)
	}

	var code string
	switch platform {
	case "gitlab":
		pipeline, err := importgitlab.Parse(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", from, err)
		}
		code = importgitlab.GenerateGo(pipeline)
	default:
		return fmt.Errorf("platform %q is not yet supported for init", platform)
	}

	if output == "" {
		fmt.Print(code)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(output, []byte(code), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("⚗️ scaffolded pipeline → %s\n", output)
	fmt.Println("⚠️  This is a best-effort import — review the generated code and adjust as needed.")
	return nil
}
