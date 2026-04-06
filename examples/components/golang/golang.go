// Package golang provides reusable CI/CD components for Go projects.
//
// Import this package to get type-safe, pre-configured jobs for testing
// and building Go binaries. Share it as a Go module across repositories.
package golang

import (
	"fmt"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
)

// TestConfig holds typed parameters for the Go test component.
type TestConfig struct {
	GoVersion    string   // required: Go image tag (e.g. "1.26")
	Race         bool     // enable race detector (default: false)
	CoverProfile string   // coverage output file (empty = no coverage)
	Tags         []string // runner tags
}

// Test creates a Go test job. Automatically configures junit and coverage
// artifact reports when CoverProfile is set.
func Test(stage *ps.Stage, name string, cfg TestConfig) *ps.Job {
	if cfg.GoVersion == "" {
		panic("golang.Test: GoVersion is required")
	}

	args := "go test"
	if cfg.Race {
		args += " -race"
	}
	if cfg.CoverProfile != "" {
		args += fmt.Sprintf(" -coverprofile=%s -covermode=atomic", cfg.CoverProfile)
	}
	args += " ./..."

	job := ps.NewJob(stage, name).
		Image(fmt.Sprintf("golang:%s", cfg.GoVersion)).
		SetCache(ps.Cache{Key: "go-mod-" + cfg.GoVersion, Paths: []string{"/go/pkg/mod"}}).
		Script(args)

	// Automatically configure artifact reports when coverage is enabled
	if cfg.CoverProfile != "" {
		job.SetArtifacts(ps.Artifacts{
			Paths:    []string{cfg.CoverProfile},
			ExpireIn: "7 days",
			Reports: map[string][]string{
				"junit": {"report.xml"},
			},
		})
	}

	for _, tag := range cfg.Tags {
		job.AddTag(tag)
	}
	return job
}

// BuildConfig holds typed parameters for the Go build component.
type BuildConfig struct {
	GoVersion  string   // required: Go image tag (e.g. "1.26")
	BinaryName string   // required: output binary name
	LDFlags    string   // extra ldflags (prepended with "-s -w")
	BuildPath  string   // path to build (default: "./cmd/<BinaryName>")
	Tags       []string // runner tags
}

// Build creates a Go build job. Produces a stripped binary with artifacts
// and automatic rollback-safe AfterScript that reports the binary size.
func Build(stage *ps.Stage, name string, cfg BuildConfig) *ps.Job {
	if cfg.GoVersion == "" {
		panic("golang.Build: GoVersion is required")
	}
	if cfg.BinaryName == "" {
		panic("golang.Build: BinaryName is required")
	}

	buildPath := cfg.BuildPath
	if buildPath == "" {
		buildPath = fmt.Sprintf("./cmd/%s", cfg.BinaryName)
	}
	ldflags := "-s -w"
	if cfg.LDFlags != "" {
		ldflags += " " + cfg.LDFlags
	}

	return ps.NewJob(stage, name).
		Image(fmt.Sprintf("golang:%s", cfg.GoVersion)).
		SetCache(ps.Cache{Key: "go-mod-" + cfg.GoVersion, Paths: []string{"/go/pkg/mod"}}).
		Env("CGO_ENABLED", "0").
		Script(
			fmt.Sprintf("go build -ldflags %q -o bin/%s %s", ldflags, cfg.BinaryName, buildPath),
		).
		AfterScript(
			fmt.Sprintf("ls -lh bin/%s 2>/dev/null || echo 'build produced no binary'", cfg.BinaryName),
		).
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"bin/"},
			ExpireIn: "30 days",
		})
}
