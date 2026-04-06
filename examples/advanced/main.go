// Package main demonstrates advanced pisyn features (rules, tags, retry, allow failure).
package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func main() {
	app := ps.NewApp()
	pipeline := ps.NewPipeline(app, "Advanced CI")

	test := ps.NewStage(pipeline, "test")

	ps.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test -race -coverprofile=coverage.out ./...").
		AddTag("non-prod-workload", "docker").
		AddRule(ps.Rule{If: ps.VarMRID, When: "always"}).
		AddRule(ps.Rule{If: ps.VarCommitBranch + ` == "main"`, When: "always"}).
		AddRule(ps.Rule{When: "never"}).
		SetRetry(ps.RetryConfig{
			Max:  2,
			When: []string{"runner_system_failure", "stuck_or_timeout_failure"},
		}).
		SetArtifacts(ps.Artifacts{
			Paths:    []string{"coverage.out"},
			ExpireIn: "7 days",
			Reports: map[string][]string{
				"junit":           {"report.xml"},
				"coverage_report": {"coverage.out"},
			},
		})

	ps.NewJob(test, "lint").
		Image("golangci/golangci-lint:latest").
		ImageEntrypoint("").
		Script("golangci-lint run --out-format=code-climate > gl-code-quality.json || true").
		AddTag("non-prod-workload").
		AddRule(ps.Rule{Changes: []string{"**/*.go", "go.mod"}, When: "always"}).
		AddRule(ps.Rule{When: "never"}).
		AllowFailureOnExitCodes(1).
		SetArtifacts(ps.Artifacts{
			Paths: []string{"gl-code-quality.json"},
			Reports: map[string][]string{
				"codequality": {"gl-code-quality.json"},
			},
		})

	deploy := ps.NewStage(pipeline, "deploy")

	ps.NewJob(deploy, "deploy-prod").
		Image("alpine:latest").
		ImageUser("deployer").
		Needs("unit-tests", "lint").
		Script("kubectl apply -f deploy/").
		AddTag("prod-workload").
		AddRule(ps.Rule{If: ps.VarCommitBranch + ` == "main"`, When: "manual"}).
		AddRule(ps.Rule{When: "never"}).
		SetRetry(ps.RetryConfig{
			Max:       1,
			When:      []string{"script_failure"},
			ExitCodes: []int{137},
		}).
		SetEnvironment("production", "https://app.example.com")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		log.Fatalf("synth: %v", err)
	}
}
