// Package main demonstrates a basic pisyn pipeline.
package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/github"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"
	_ "github.com/pipecrew/pisyn/pkg/synth/tekton"
)

func main() {
	app := ps.NewApp()

	pipeline := ps.NewPipeline(app, "CI CD").
		OnPush("main", "develop").
		OnPR("main").
		SetEnv("GO_VERSION", "1.26")

	// --- Test stage ---
	test := ps.NewStage(pipeline, "test")

	ps.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test ./...").
		SetCache(ps.Cache{Key: "go-mod", Paths: []string{"/go/pkg/mod"}}).
		SetArtifacts(ps.Artifacts{Paths: []string{"coverage.out"}, ExpireIn: "7 days"}).
		Timeout(15)

	ps.NewJob(test, "lint").
		Image("golangci/golangci-lint:latest").
		Script("golangci-lint run")

	// --- Build stage ---
	build := ps.NewStage(pipeline, "build")

	ps.NewJob(build, "build-binary").
		Image("golang:1.26").
		Needs("unit-tests", "lint").
		Script("go build -o app ./cmd/app").
		SetArtifacts(ps.Artifacts{Paths: []string{"app"}})

	// --- Deploy stage (uses platform-neutral variables) ---
	deploy := ps.NewStage(pipeline, "deploy")

	ps.NewJob(deploy, "deploy-prod").
		Image("alpine:latest").
		Needs("build-binary").
		If(ps.VarCommitBranch+` == "main"`).
		Script("echo deploying "+ps.VarProjectPath+" at "+ps.VarCommitSHA).
		SetEnvironment("production", "https://app.example.com").
		SetWhen(ps.Manual)

	// Synthesize all platforms
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
