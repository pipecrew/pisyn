// Package main demonstrates deploying a Docusaurus site to GitHub Pages.
// Stages: install → build → deploy, using GitHub Actions steps and caching.
package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/github"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

func main() {
	app := ps.NewApp()

	pipeline := ps.NewPipeline(app, "Deploy Docusaurus").
		OnPush("main")

	// --- Build stage ---
	build := ps.NewStage(pipeline, "build")

	ps.NewJob(build, "build-docusaurus").
		Image("node:22-alpine").
		SetCache(ps.Cache{Key: "npm", Paths: []string{"node_modules"}}).
		AddStep(ps.Step{Uses: "actions/configure-pages@v5"}).
		Script(
			"npm ci",
			"npm run build",
		).
		AddStep(ps.Step{
			Uses: "actions/upload-pages-artifact@v3",
			With: map[string]string{"path": "build"},
		})

	// --- Deploy stage ---
	deploy := ps.NewStage(pipeline, "deploy")

	ps.NewJob(deploy, "deploy-pages").
		Needs("build-docusaurus").
		If(ps.VarCommitBranch + ` == "main"`).
		AddStep(ps.Step{
			Uses: "actions/deploy-pages@v4",
			ID:   "deployment",
		}).
		SetEnvironment("github-pages", "")

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
