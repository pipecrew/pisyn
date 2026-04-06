package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/github"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func main() {
	app := ps.NewApp()

	pipeline := ps.NewPipeline(app, "pisyn buildpipeline").
		OnPush("main").
		OnPR("main").
		OnPushProtected().
		SetEnv("CGO_ENABLED", "0").
		SetEnv("GOFLAGS", "-trimpath")

	// --- Reusable templates ---
	goBase := ps.JobTemplate("go-base").
		Image("golang:1.26").
		SetCache(ps.Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
		AddTag("non-prod-workload").
		SetInterruptible(true).
		SetRetry(ps.RetryConfig{
			Max:  2,
			When: []string{"runner_system_failure", "stuck_or_timeout_failure"},
		})

	// --- Lint stage ---
	lint := ps.NewStage(pipeline, "lint")

	goBase.Clone(lint, "lint-go").
		Image("golangci/golangci-lint:v2.11.3-alpine").
		ImageEntrypoint("").
		AddRule(ps.Rule{If: ps.VarMRID, When: "always"}).
		AddRule(ps.Rule{When: "never"}).
		AllowFailure().
		Script("golangci-lint run --timeout 5m ./...").
		SetArtifacts(ps.Artifacts{
			Paths: []string{"gl-code-quality.json"},
			Reports: map[string][]string{
				"codequality": {"gl-code-quality.json"},
			},
		})

	goBase.Clone(lint, "vulncheck").
		AddRule(ps.Rule{If: ps.VarMRID, When: "always"}).
		AddRule(ps.Rule{When: "never"}).
		AllowFailure().
		Script(
			"go install golang.org/x/vuln/cmd/govulncheck@latest",
			"govulncheck ./...",
		)

	// --- Test stage ---
	test := ps.NewStage(pipeline, "test")

	goBase.Clone(test, "unit-tests").
		Needs("lint-go", "vulncheck").
		Script("go test -race -coverprofile=coverage.out -covermode=atomic ./...").
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"coverage.out"},
			ExpireIn: "7 days",
			Reports: map[string][]string{
				"junit": {"report.xml"},
			},
		})

	// --- Build stage (uses version output from gen-version) ---
	build := ps.NewStage(pipeline, "build")

	genVersion := goBase.Clone(build, "gen-version").
		Needs("unit-tests").
		Script("git config --global --add safe.directory "+ps.VarProjectDir).
		Script(`echo "VERSION=$(git describe --tags --always)" > version.env`).
		Output("VERSION", "version.env")

	goBase.Clone(build, "build-binary").
		Needs("gen-version").
		Script(`echo "Version is: "` + genVersion.OutputRef("VERSION")).
		Script(
			"go build -buildvcs=false  -ldflags \"-s -w -X main.version=" + genVersion.OutputRef("VERSION") + "\" -o bin/pisyn ./cmd/pisyn",
		).
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"bin/"},
			ExpireIn: "30 days",
		})

	// --- Release stage (only on main branch, uses version from gen-version) ---
	release := ps.NewStage(pipeline, "release")

	goBase.Clone(release, "release").
		Needs("build-binary").
		AddTag("prod-workload").
		SetInterruptible(false).
		If(ps.VarCommitBranch+` == "main"`).
		AddStep(ps.Step{
			Uses: "actions/upload-artifact@v4",
			With: map[string]string{"name": "release-binary", "path": "bin/"},
		}).
		Script(
			"echo releasing "+ps.VarProjectPath+" version "+genVersion.OutputRef("VERSION"),
		).
		SetEnvironment("production", "https://app.example.com")

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
