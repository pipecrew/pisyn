package pisyn

// Platform-neutral CI/CD variables. Each synthesizer translates these to
// platform-specific equivalents (e.g. $CI_COMMIT_BRANCH, ${{ github.ref_name }}).
const (
	VarCommitBranch   = "$PISYN_COMMIT_BRANCH"
	VarCommitSHA      = "$PISYN_COMMIT_SHA"
	VarCommitTag      = "$PISYN_COMMIT_TAG"
	VarCommitMessage  = "$PISYN_COMMIT_MESSAGE"
	VarDefaultBranch  = "$PISYN_DEFAULT_BRANCH"
	VarPipelineID     = "$PISYN_PIPELINE_ID"
	VarJobID          = "$PISYN_JOB_ID"
	VarJobName        = "$PISYN_JOB_NAME"
	VarJobToken       = "$PISYN_JOB_TOKEN"
	VarProjectPath    = "$PISYN_PROJECT_PATH"
	VarProjectURL     = "$PISYN_PROJECT_URL"
	VarProjectDir     = "$PISYN_PROJECT_DIR"
	VarProjectName    = "$PISYN_PROJECT_NAME"
	VarProjectNS      = "$PISYN_PROJECT_NAMESPACE"
	VarMRID           = "$PISYN_MR_ID"
	VarMRSourceBranch = "$PISYN_MR_SOURCE_BRANCH"
	VarMRTargetBranch = "$PISYN_MR_TARGET_BRANCH"
	VarRefProtected   = "$PISYN_REF_PROTECTED"
)

// GitLabVars maps pisyn variable names (without $) to GitLab CI equivalents.
var GitLabVars = map[string]string{
	"PISYN_COMMIT_BRANCH":     "CI_COMMIT_BRANCH",
	"PISYN_COMMIT_SHA":        "CI_COMMIT_SHA",
	"PISYN_COMMIT_TAG":        "CI_COMMIT_TAG",
	"PISYN_COMMIT_MESSAGE":    "CI_COMMIT_MESSAGE",
	"PISYN_DEFAULT_BRANCH":    "CI_DEFAULT_BRANCH",
	"PISYN_PIPELINE_ID":       "CI_PIPELINE_ID",
	"PISYN_JOB_ID":            "CI_JOB_ID",
	"PISYN_JOB_NAME":          "CI_JOB_NAME",
	"PISYN_JOB_TOKEN":         "CI_JOB_TOKEN",
	"PISYN_PROJECT_PATH":      "CI_PROJECT_PATH",
	"PISYN_PROJECT_URL":       "CI_PROJECT_URL",
	"PISYN_PROJECT_DIR":       "CI_PROJECT_DIR",
	"PISYN_PROJECT_NAME":      "CI_PROJECT_NAME",
	"PISYN_PROJECT_NAMESPACE": "CI_PROJECT_NAMESPACE",
	"PISYN_MR_ID":             "CI_MERGE_REQUEST_ID",
	"PISYN_MR_SOURCE_BRANCH":  "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
	"PISYN_MR_TARGET_BRANCH":  "CI_MERGE_REQUEST_TARGET_BRANCH_NAME",
	"PISYN_REF_PROTECTED":     "CI_COMMIT_REF_PROTECTED",
}

// GitHubVars maps pisyn variable names (without $) to GitHub Actions equivalents.
var GitHubVars = map[string]string{
	"PISYN_COMMIT_BRANCH":     "${{ github.ref_name }}",
	"PISYN_COMMIT_SHA":        "${{ github.sha }}",
	"PISYN_COMMIT_TAG":        "${{ github.ref_name }}",
	"PISYN_COMMIT_MESSAGE":    "${{ github.event.head_commit.message }}",
	"PISYN_DEFAULT_BRANCH":    "${{ github.event.repository.default_branch }}",
	"PISYN_PIPELINE_ID":       "${{ github.run_id }}",
	"PISYN_JOB_ID":            "${{ github.run_id }}",
	"PISYN_JOB_NAME":          "${{ github.job }}",
	"PISYN_JOB_TOKEN":         "${{ secrets.GITHUB_TOKEN }}",
	"PISYN_PROJECT_PATH":      "${{ github.repository }}",
	"PISYN_PROJECT_URL":       "${{ github.server_url }}/${{ github.repository }}",
	"PISYN_PROJECT_DIR":       "${GITHUB_WORKSPACE}",
	"PISYN_PROJECT_NAME":      "${{ github.event.repository.name }}",
	"PISYN_PROJECT_NAMESPACE": "${{ github.repository_owner }}",
	"PISYN_MR_ID":             "${{ github.event.pull_request.number }}",
	"PISYN_MR_SOURCE_BRANCH":  "${{ github.head_ref }}",
	"PISYN_MR_TARGET_BRANCH":  "${{ github.base_ref }}",
	"PISYN_REF_PROTECTED":     "${{ github.ref_protected }}",
}

// TektonVars maps pisyn variable names (without $) to Tekton param references.
var TektonVars = map[string]string{
	"PISYN_COMMIT_BRANCH":     "$(params.commit-branch)",
	"PISYN_COMMIT_SHA":        "$(params.commit-sha)",
	"PISYN_COMMIT_TAG":        "$(params.commit-tag)",
	"PISYN_COMMIT_MESSAGE":    "$(params.commit-message)",
	"PISYN_DEFAULT_BRANCH":    "$(params.default-branch)",
	"PISYN_PIPELINE_ID":       "$(params.pipeline-id)",
	"PISYN_JOB_ID":            "$(params.job-id)",
	"PISYN_JOB_NAME":          "$(params.job-name)",
	"PISYN_JOB_TOKEN":         "$(params.job-token)",
	"PISYN_PROJECT_PATH":      "$(params.project-path)",
	"PISYN_PROJECT_URL":       "$(params.project-url)",
	"PISYN_PROJECT_DIR":       "$(workspaces.source.path)",
	"PISYN_PROJECT_NAME":      "$(params.project-name)",
	"PISYN_PROJECT_NAMESPACE": "$(params.project-namespace)",
	"PISYN_MR_ID":             "$(params.mr-id)",
	"PISYN_MR_SOURCE_BRANCH":  "$(params.mr-source-branch)",
	"PISYN_MR_TARGET_BRANCH":  "$(params.mr-target-branch)",
	"PISYN_REF_PROTECTED":     "$(params.ref-protected)",
}
