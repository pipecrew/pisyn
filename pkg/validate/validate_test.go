package validate_test

import (
	"testing"

	"github.com/pipecrew/pisyn/pkg/validate"
)

func TestValidateGitLabValid(t *testing.T) {
	yml := []byte(`
stages:
  - test
unit-tests:
  stage: test
  image: golang:1.26
  script:
    - go test ./...
`)
	if err := validate.Validate("gitlab", yml); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateGitHubValid(t *testing.T) {
	yml := []byte(`
name: CI
on:
  push:
    branches: [main]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - run: echo hello
`)
	if err := validate.Validate("github", yml); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateGitHubInvalid(t *testing.T) {
	yml := []byte(`
name: CI
on:
  push:
    branches: [main]
jobs:
  test:
    steps:
      - run: echo hello
`)
	err := validate.Validate("github", yml)
	if err == nil {
		t.Fatal("expected validation error for missing runs-on")
	}
}

func TestValidateUnknownPlatform(t *testing.T) {
	if err := validate.Validate("unknown", []byte(`{}`)); err == nil {
		t.Fatal("expected error for unknown platform")
	}
}
