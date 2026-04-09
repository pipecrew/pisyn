package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/github"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func main() {
	app := ps.NewApp()

	buildPipeline(app)
	releasePleasePipeline(app)
	releasePipeline(app)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildPipeline(app *ps.App) {
	pipeline := ps.NewPipeline(app, "Build").
		OnPR("main").
		OnPushProtected().
		SetEnv("CGO_ENABLED", "0").
		SetEnv("GOFLAGS", "-trimpath")

	goBase := ps.JobTemplate("go-base").
		Image("golang:1.26").
		SetCache(ps.Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
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
		Script("golangci-lint run --timeout 5m ./...")

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
		Script("go test -v -cover -coverprofile=coverage.out -covermode=atomic ./...").
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"coverage.out"},
			ExpireIn: "7 days",
			Reports: map[string][]string{
				"junit": {"report.xml"},
			},
		})

	// --- Build stage ---
	build := ps.NewStage(pipeline, "build")

	genVersion := goBase.Clone(build, "gen-version").
		Needs("unit-tests").
		SetFetchDepth(0).
		Script(`echo "VERSION=$(git describe --tags --always)" > version.env`).
		Output("VERSION", "version.env")

	goBase.Clone(build, "build-binary").
		Needs("gen-version").
		Script(`echo "Version is: "` + genVersion.OutputRef("VERSION")).
		Script(
			"go build -buildvcs=false -ldflags \"-s -w -X main.pisynVersion=" + genVersion.OutputRef("VERSION") + "\" -o bin/pisyn ./cmd/pisyn",
		).
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"bin/"},
			ExpireIn: "30 days",
		})
}

func releasePleasePipeline(app *ps.App) {
	pipeline := ps.NewPipeline(app, "Release Please").
		OnPush("main")

	stage := ps.NewStage(pipeline, "release-please")

	ps.NewJob(stage, "release-please").
		Image("node:22-alpine").
		Env("RELEASE_PAT", "${{ secrets.RELEASE_PAT }}").
		Script(
			"npx release-please release-pr --repo-url="+ps.VarProjectURL+" --token=$RELEASE_PAT --release-type=go --pull-request-header=\":robot: new release\" --pull-request-footer=\" \"",
			"npx release-please github-release --repo-url="+ps.VarProjectURL+" --token=$RELEASE_PAT --release-type=go",
		)
}

func releasePipeline(app *ps.App) {
	pipeline := ps.NewPipeline(app, "Release").
		OnPushTag("v*")

	stage := ps.NewStage(pipeline, "goreleaser")

	ps.NewJob(stage, "goreleaser").
		Image("goreleaser/goreleaser:v2.15.2").
		SetFetchDepth(0).
		Script("goreleaser release --clean").
		Env("GITHUB_TOKEN", "${{ secrets.GITHUB_TOKEN }}")
}
