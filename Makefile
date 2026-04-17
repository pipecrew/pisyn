# Variables
SCHEMAS_DIR   := pkg/validate/schemas
GITLAB_URL    := https://gitlab.com/gitlab-org/gitlab/-/raw/master/app/assets/javascripts/editor/schema/ci.json
GITHUB_URL    := https://json.schemastore.org/github-workflow.json

.PHONY: help build test update-schemas

help:
	@echo "Available targets:"
	@echo "  build            Build the pisyn binary"
	@echo "  test             Run unit tests"
	@echo "  update-schemas   Refresh bundled CI schemas from upstream (GitLab + GitHub)"

build:
	go build ./...

test:
	go test ./...

update-schemas:
	@echo "Fetching GitLab CI schema from $(GITLAB_URL)"
	curl -fsSL $(GITLAB_URL) -o $(SCHEMAS_DIR)/gitlab-ci.json
	@echo "Fetching GitHub Actions workflow schema from $(GITHUB_URL)"
	curl -fsSL $(GITHUB_URL) -o $(SCHEMAS_DIR)/github-workflow.json
	@echo "Schemas updated. Review with: git diff $(SCHEMAS_DIR)"
