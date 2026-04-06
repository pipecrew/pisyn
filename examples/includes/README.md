# Includes Example

Compose pisyn-generated pipelines with existing shared GitLab CI templates.

## What This Shows

Many organizations have existing shared CI templates (security scanning, compliance checks, base configurations). This example shows how to include them alongside pisyn-generated jobs using all four GitLab CI `include:` types.

### pisyn Features Used

- **`IncludeLocal()`** — include a file from the same repository (`/templates/base.yml`)
- **`IncludeRemote()`** — include a file from any URL (`https://example.com/ci.yml`)
- **`IncludeProject()`** — include a file from another GitLab project with a specific ref (`shared/templates`, `ci/lint.yml`, `main`)
- **`IncludeTemplate()`** — include a GitLab-provided template (`Auto-DevOps.gitlab-ci.yml`)
- **Direct synthesizer call** — targets GitLab CI only since includes are a GitLab-specific feature

### Generated Output

The `include:` block is rendered at the top of `.gitlab-ci.yml`, before `stages:`:

```yaml
include:
    - local: /templates/base.yml
    - remote: https://example.com/ci.yml
    - file: ci/lint.yml
      project: shared/templates
      ref: main
    - template: Auto-DevOps.gitlab-ci.yml
stages:
    - test
# ... jobs follow
```

## Run It

```sh
go run .                    # synthesizes GitLab CI only
```

Output: `pisyn.out/.gitlab-ci.yml`
