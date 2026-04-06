// Package main demonstrates GitLab CI include directives with pisyn.
package main

import (
	"log"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func main() {
	app := pisyn.NewApp()

	p := pisyn.NewPipeline(app, "CI").
		IncludeLocal("/templates/base.yml").
		IncludeRemote("https://example.com/ci.yml").
		IncludeProject("shared/templates", "ci/lint.yml", "main").
		IncludeTemplate("Auto-DevOps.gitlab-ci.yml")

	test := pisyn.NewStage(p, "test")
	pisyn.NewJob(test, "unit-tests").
		Image("golang:1.26").
		Script("go test ./...")

	if err := app.Synth(gitlab.NewSynthesizer()); err != nil {
		log.Fatal(err)
	}
}
