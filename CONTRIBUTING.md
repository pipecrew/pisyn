# Contributing to pisyn

Thanks for your interest in contributing to pisyn! This guide will get you up and running.

## Quick Setup

```sh
# Clone
git clone https://github.com/pipecrew/pisyn.git
cd pisyn

# Build
go build ./...

# Run tests
go test ./...

# Build the CLI
go build -o pisyn ./cmd/pisyn
```

Requires Go 1.26+. Docker is needed only for `pisyn run` (local execution).

## Project Structure

```
cmd/pisyn/main.go          CLI (Cobra commands: synth, build, validate, graph, run, diff, init)
pkg/pisyn/                  Core DSL: App → Pipeline → Stage → Job construct tree
pkg/synth/gitlab/           GitLab CI synthesizer
pkg/synth/github/           GitHub Actions synthesizer
pkg/synth/tekton/           Tekton synthesizer
pkg/importer/gitlab/        GitLab CI → Go reverse importer (pisyn init)
pkg/runner/                 Local Docker execution engine
pkg/tui/                    Terminal UI for pisyn run
pkg/validate/               JSON schema validation for generated YAML
examples/                   Example pipelines (basic, advanced, golang, includes, components, dac)
pipeline/                   pisyn's own CI pipeline (eats its own dogfood)
```

## How It Works

1. Users define pipelines in Go using the builder API (`NewApp → NewPipeline → NewStage → NewJob`)
2. `app.Run()` serializes the construct tree to `pipeline.json` (IR)
3. Synthesizers walk the tree and emit platform-specific YAML
4. Each synthesizer self-registers via `init()` — blank imports activate them

## Making Changes

### Adding a job feature

1. Add the field to `Job` struct in `pkg/pisyn/job.go`
2. Add a builder method (returns `*Job` for chaining)
3. Add the field to `IRJob` in `pkg/pisyn/ir.go` + update `jobToIR` and `irJobToJob`
4. Update each synthesizer that supports it (`pkg/synth/gitlab/`, `pkg/synth/github/`)
5. Update the feature matrix in `README.md`

### Adding a synthesizer

1. Create `pkg/synth/<platform>/<platform>.go`
2. Implement the `pisyn.Synthesizer` interface
3. Register via `init()`: `pisyn.RegisterPlatform("name", func() pisyn.Synthesizer { ... })`
4. Users activate it with a blank import: `_ "github.com/pipecrew/pisyn/pkg/synth/<platform>"`

### Adding an importer (pisyn init)

1. Create `pkg/importer/<platform>/` with a parser and code generator
2. Add the platform case in `runInit` in `cmd/pisyn/main.go`

## Code Conventions

- Builder methods return `*Job` / `*Pipeline` for chaining
- Synthesizer helper functions follow the pattern `setX(cfg map[string]any, job *pisyn.Job)`
- Platform-neutral variables use `$PISYN_*` prefix — each synthesizer translates them
- Use `synth.OrderedMap` for YAML output to preserve key order
- `Clone()` must deep-copy all mutable fields (slices, maps, pointers)

## Running Tests

```sh
go test ./...                    # all tests
go test ./pkg/pisyn/            # core only
go test ./pkg/synth/gitlab/     # gitlab synthesizer only
go test -cover ./...             # with coverage
```

## Updating bundled schemas

`pkg/validate/schemas/` holds the JSON schemas bundled into the binary via `//go:embed`. When upstream schemas change, refresh the local copies:

```sh
make update-schemas
```

That target downloads the latest `gitlab-ci.json` and `github-workflow.json` from their canonical sources (GitLab `gitlab-org/gitlab` and [json.schemastore.org](https://json.schemastore.org/)) into `pkg/validate/schemas/`. Review the diff with `git diff pkg/validate/schemas/` and commit the updated files.

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) for automated releases:

```
feat: add matrix exclude support
fix: correct timeout rendering for GitHub Actions
docs: update feature matrix
chore: bump Go version
```

`feat:` and `fix:` trigger releases via release-please.

## Pull Request Process

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Ensure `go build ./...` and `go test ./...` pass
4. Open a PR with a clear description of what and why
5. We'll review within a few days

## Good First Issues

Look for issues labeled [`good first issue`](https://github.com/pipecrew/pisyn/labels/good%20first%20issue) — these are scoped, self-contained tasks ideal for getting familiar with the codebase.
