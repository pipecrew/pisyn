# Advanced Example

Deep dive into GitLab CI-specific features — rules, tags, retry, allow_failure with exit codes, image entrypoint overrides, and artifact reports.

## What This Shows

This example targets GitLab CI only (using `app.Synth(gitlab.NewSynthesizer())` directly) and exercises the advanced job configuration options that map to GitLab's richer feature set.

### pisyn Features Used

- **Direct synthesizer call** — `app.Synth(gitlab.NewSynthesizer())` instead of `app.Run()` for single-platform output
- **Rules with conditions** — `AddRule()` with `If`, `When`, and `Changes` for fine-grained job execution control
- **Runner tags** — `AddTag()` for scheduling jobs on specific runners
- **Retry with failure conditions** — `SetRetry()` with `When` filters (`runner_system_failure`, `stuck_or_timeout_failure`) and exit code matching
- **Allow failure on specific exit codes** — `AllowFailureOnExitCodes()` so only certain failures are tolerated
- **Image entrypoint override** — `ImageEntrypoint("")` for images that have non-shell entrypoints (like ruff)
- **Image docker user** — `ImageUser()` to run as a specific user inside the container
- **Artifact reports** — `SetArtifacts()` with `Reports` for junit, coverage, and codequality integration
- **Deployment environments** — `SetEnvironment()` with manual deploy gate via rules

## Run It

```sh
go run .                    # synthesizes GitLab CI only
```

Output: `pisyn.out/.gitlab-ci.yml`
