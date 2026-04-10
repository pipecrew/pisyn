# AGENTS.md — Context for AI Coding Agents

This file provides context for AI agents (Copilot, Cursor, Kiro, Devin, etc.) working on pisyn.

## Build & Test

```sh
go build ./...       # compile everything
go test ./...        # run all tests
go test -cover ./... # with coverage
```

Go 1.26+. No external tools needed for build/test. Docker needed only for `pkg/runner/` and `pkg/tui/`.

## What pisyn Does

pisyn is a CDK for CI/CD: define pipelines in Go, synthesize to GitLab CI / GitHub Actions / Tekton YAML. The construct tree is `App → Pipeline → Stage → Job`. Synthesizers walk the tree and emit platform-specific files.

## Package Map

| Package | Purpose | Key files |
|---|---|---|
| `pkg/pisyn` | Core DSL — construct tree, job builder API, IR serialization | `job.go`, `pipeline.go`, `types.go`, `ir.go`, `vars.go` |
| `pkg/synth/gitlab` | GitLab CI YAML synthesizer | `gitlab.go` |
| `pkg/synth/github` | GitHub Actions YAML synthesizer | `github.go` |
| `pkg/synth/tekton` | Tekton YAML synthesizer | `tekton.go` |
| `pkg/importer/gitlab` | Reverse import: `.gitlab-ci.yml` → Go code | `gitlab.go` (parser), `codegen.go` (code gen) |
| `pkg/importer` | Platform detection from file path/content | `detect.go` |
| `pkg/runner` | Local Docker execution engine | `docker.go`, `plan.go`, `runner.go` |
| `pkg/tui` | Bubbletea TUI for `pisyn run` | `model.go`, `run.go` |
| `pkg/validate` | JSON schema validation | `validate.go` |
| `cmd/pisyn` | CLI entry point (Cobra) | `main.go` |

## Code Patterns

- **Builder pattern**: all Job/Pipeline methods return `*Job`/`*Pipeline` for chaining
- **Synthesizer helpers**: `setX(cfg map[string]any, job *pisyn.Job)` — one function per feature
- **Platform registration**: synthesizers self-register via `init()` + `pisyn.RegisterPlatform()`
- **Variable translation**: `$PISYN_*` constants in `vars.go`, per-platform maps (`GitLabVars`, `GitHubVars`, `TektonVars`)
- **IR round-trip**: construct tree ↔ `pipeline.json` via `ToIR()`/`ToApp()` — all fields must be mapped in both directions
- **Clone deep-copy**: `Clone()` in `job.go` must deep-copy all mutable fields (slices, maps, pointer structs)

## Adding a Job Feature (Checklist)

1. Add field to `Job` struct in `pkg/pisyn/job.go`
2. Add builder method (chainable, returns `*Job`)
3. Add field to `IRJob` in `pkg/pisyn/ir.go`
4. Update `jobToIR()` and `irJobToJob()` in `ir.go`
5. Update `Clone()` in `job.go` (deep-copy if mutable)
6. Update relevant synthesizers in `pkg/synth/*/`
7. Update feature matrix in `README.md`

## Known Technical Debt

Browse open issues by label:

- [All issues](https://github.com/pipecrew/pisyn/issues)
- [`good first issue`](https://github.com/pipecrew/pisyn/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) — small, self-contained tasks ideal for getting started
- [`help wanted`](https://github.com/pipecrew/pisyn/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22) — larger features that need contributors
- [`bug`](https://github.com/pipecrew/pisyn/issues?q=is%3Aissue+is%3Aopen+label%3Abug) — correctness issues
- [`enhancement`](https://github.com/pipecrew/pisyn/issues?q=is%3Aissue+is%3Aopen+label%3Aenhancement) — feature improvements

## Testing Expectations

- All existing tests must pass after changes: `go test ./...`
- Synthesizer tests compare generated YAML against expected output
- Core tests verify builder API behavior, clone isolation, and construct tree structure
- No test framework beyond stdlib `testing` — use table-driven tests where appropriate

## Files to Never Modify Without Understanding

- `pkg/pisyn/construct.go` — the tree node base; changes affect everything
- `pkg/pisyn/vars.go` — variable maps must stay in sync across all three platforms
- `pkg/validate/schemas/*.json` — bundled from upstream, don't hand-edit
