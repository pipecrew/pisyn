// Package main demonstrates reusable pipeline components with typed parameters.
//
// This is the pisyn equivalent of GitLab CI components or GitHub reusable workflows:
// define typed functions in a package, share them as a Go module.
// Consumers get compile-time validation, IDE autocomplete, and proper versioning.
package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"

	// In a real project, these would be external modules:
	//   import "github.com/yourorg/ci-components/golang"
	//   import "github.com/yourorg/ci-components/deploy"
	"github.com/pipecrew/pisyn/examples/components/deploy"
	"github.com/pipecrew/pisyn/examples/components/golang"
)

func main() {
	app := ps.NewApp()

	pipeline := ps.NewPipeline(app, "CI CD").
		OnPush("main").
		OnPR("main")

	// --- Test stage: one call, fully configured ---
	test := ps.NewStage(pipeline, "test")

	golang.Test(test, "unit-tests", golang.TestConfig{
		GoVersion:    "1.26",
		Race:         true,
		CoverProfile: "coverage.out",
		Tags:         []string{"non-prod-workload"},
	})

	// --- Build stage: two binaries from the same repo ---
	build := ps.NewStage(pipeline, "build")

	golang.Build(build, "build-api", golang.BuildConfig{
		GoVersion:  "1.26",
		BinaryName: "api-server",
		// BuildPath defaults to ./cmd/api-server
	}).Needs("unit-tests")

	golang.Build(build, "build-worker", golang.BuildConfig{
		GoVersion:  "1.26",
		BinaryName: "worker",
		LDFlags:    `-X main.version="dev"`,
	}).Needs("unit-tests")

	// --- Deploy stage: staging auto, production manual ---
	deployStage := ps.NewStage(pipeline, "deploy")

	deploy.Kubernetes(deployStage, "deploy-staging", deploy.KubernetesConfig{
		Environment: "staging",
		URL:         "https://staging.example.com",
		Namespace:   "app-staging",
		ManifestDir: "deploy/staging/",
		Tags:        []string{"prod-workload"},
	}).Needs("build-api", "build-worker").
		If(ps.VarCommitBranch + ` == "main"`)

	deploy.Kubernetes(deployStage, "deploy-production", deploy.KubernetesConfig{
		Environment: "production",
		URL:         "https://app.example.com",
		Namespace:   "app-prod",
		ManifestDir: "deploy/production/",
		Manual:      true,
		Tags:        []string{"prod-workload"},
	}).Needs("deploy-staging")

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
