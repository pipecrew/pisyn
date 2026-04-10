# Docusaurus → GitHub Pages

Deploy a [Docusaurus](https://docusaurus.io/) site to GitHub Pages — a real docs-as-code pipeline.

## What This Shows

A two-stage pipeline: build the Docusaurus site, then deploy to GitHub Pages. Uses GitHub Actions steps (`configure-pages`, `upload-pages-artifact`, `deploy-pages`) alongside regular scripts, caching, environments, and conditional deployment.

### pisyn Features Used

- **GitHub Actions steps** — `AddStep()` for `configure-pages`, `upload-pages-artifact`, and `deploy-pages`
- **Caching** — `SetCache()` for `node_modules`
- **Deployment environments** — `SetEnvironment("github-pages", ...)` for GitHub Pages
- **Conditional deployment** — `If()` with `VarCommitBranch` to deploy only from `main`
- **Job dependencies** — `Needs()` to chain build → deploy
- **Multi-platform output** — targets both GitLab CI and GitHub Actions

## Pipeline Graph

```mermaid
graph LR
    build-docusaurus --> deploy-pages
```

## Run It

```sh
go run .                    # synthesizes GitLab CI and GitHub Actions
```

Output:
- `pisyn.out/.gitlab-ci.yml`
- `pisyn.out/.github/workflows/deploy-docusaurus.yml`
