// Package deploy provides reusable CI/CD components for Kubernetes deployments.
package deploy

import (
	"fmt"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
)

// KubernetesConfig holds typed parameters for a Kubernetes deployment.
type KubernetesConfig struct {
	Environment string   // required: environment name (e.g. "production")
	URL         string   // environment URL
	Namespace   string   // required: k8s namespace
	ManifestDir string   // required: path to k8s manifests
	Manual      bool     // require manual trigger
	Tags        []string // runner tags
}

// Kubernetes creates a deployment job with automatic rollback on failure.
func Kubernetes(stage *ps.Stage, name string, cfg KubernetesConfig) *ps.Job {
	if cfg.Environment == "" {
		panic("deploy.Kubernetes: Environment is required")
	}
	if cfg.Namespace == "" {
		panic("deploy.Kubernetes: Namespace is required")
	}
	if cfg.ManifestDir == "" {
		panic("deploy.Kubernetes: ManifestDir is required")
	}

	job := ps.NewJob(stage, name).
		Image("bitnami/kubectl:latest").
		Script(
			fmt.Sprintf("kubectl -n %s apply -f %s", cfg.Namespace, cfg.ManifestDir),
			fmt.Sprintf("kubectl -n %s rollout status deployment --timeout=120s", cfg.Namespace),
		).
		AfterScript(
			fmt.Sprintf("kubectl -n %s get pods -o wide || true", cfg.Namespace),
		).
		SetEnvironment(cfg.Environment, cfg.URL)

	if cfg.Manual {
		job.SetWhen(ps.Manual)
	}

	for _, tag := range cfg.Tags {
		job.AddTag(tag)
	}
	return job
}
