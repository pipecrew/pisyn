// Package main demonstrates a real-world pisyn pipeline (docs-as-code linting).
// This is a pisyn representation of examplegitlab-ci.yaml, showcasing:
// service-level variables, rules, tags, interruptible, empty needs,
// allow_failure with exit codes, artifact reports, retry with conditions,
// image entrypoint overrides, and job templates with Clone().
package main

import (
	"log"

	ps "github.com/pipecrew/pisyn/pkg/pisyn"
	_ "github.com/pipecrew/pisyn/pkg/synth/github"
	_ "github.com/pipecrew/pisyn/pkg/synth/gitlab"
)

const (
	pipelineUtilsImage           = "pipeship-docker-release-local.bahnhub.tech.rz.db.de/pipeline-utils:2.6.1@sha256:68d77e05451ba4049ede270202b98319076707cfae2a4f83d31e18208fdbaa7e"
	createModuleAnnotationsImage = "pipeship-docker-release-local.bahnhub.tech.rz.db.de/create-module-annotations:1.8.24@sha256:a30cd81745972c7c5e311db3951226a69063de04c44a32aeea4c7a51c9432517"
)

var svcVars = map[string]string{
	"KUBERNETES_SERVICE_CPU_REQUEST":    "1m",
	"KUBERNETES_SERVICE_MEMORY_REQUEST": "1Mi",
}

func main() {
	app := ps.NewApp()

	pipeline := ps.NewPipeline(app, "DAC Lint Pipeline").
		SetEnv("DAC_STAGE_INCLUDED", "true").
		SetEnv("LA_JOB_INCLUDED", "true").
		SetEnv("LSI_JOB_INCLUDED", "true").
		SetEnv("LP_JOB_INCLUDED", "true").
		SetEnv("LK_JOB_INCLUDED", "true").
		SetEnv("LX_JOB_INCLUDED", "true").
		SetEnv("LD_JOB_INCLUDED", "true").
		SetEnv("LCF_JOB_INCLUDED", "true")

	autoTest := ps.NewStage(pipeline, "automatic_test")

	// Base template: shared services (with variables), retry, tags, interruptible, empty needs
	baseLint := ps.JobTemplate("base-lint").
		AddServiceWithVars(pipelineUtilsImage, "", svcVars).
		AddServiceWithVars(createModuleAnnotationsImage, "", svcVars).
		AddRule(ps.Rule{When: "always"}).
		AddTag("non-prod-workload").
		SetInterruptible(true).
		EmptyNeedsList().
		SetRetry(ps.RetryConfig{
			Max:  2,
			When: []string{"unknown_failure", "runner_system_failure", "stuck_or_timeout_failure"},
		}).
		SetArtifacts(ps.Artifacts{
			Reports: map[string][]string{
				"annotations": {"${CI_PROJECT_DIR}/branding.json"},
			},
		}).
		AllowFailure()

	// ── lint_alex ──
	lintAlex := baseLint.Clone(autoTest, "lint_alex").
		Image("db-cd-lib-docker-release-local.bahnhub.tech.rz.db.de/alex@sha256:138eb9dffb766f8df65bbf530570a23798517bdbb49db730299a4a11cc3629f4").
		Env("LA_TARGET_PATH", "pass").
		BeforeScript("cd tests/lint_alex").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LA_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LA_TARGET_PATH "**"`,
			`log_config "LA_"`,
			`alex ${LA_TARGET_PATH} ${LA_ADDITIONAL_ARGS}`,
		)

	// ── lint_scm_info ──
	lintScmInfo := baseLint.Clone(autoTest, "lint_scm_info").
		Image("pipeship-docker-release-local.bahnhub.tech.rz.db.de/yaml-validator:1.4.0@sha256:f4a15ed6f6bc26794f0d465b84e605cb0fa74c44553f25ba1599656c6bc593f1").
		Env("LSI_SCHEMA_REPO", "https://bahnhub.tech.rz.db.de/artifactory/db-inner-source-generic-release-local/scm-info-json-schema/v3.schema.yaml").
		SetRetry(ps.RetryConfig{
			Max:  2,
			When: []string{"script_failure", "unknown_failure", "runner_system_failure", "stuck_or_timeout_failure"},
		}).
		BeforeScript("cd tests/lint_scm_info/pass").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LSI_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`checkEnv LSI_SCHEMA_REPO "need to know from where to download the most current schema"`,
			`export LSI_SCM_INFO_PATH=scm-info.yaml`,
			`checkFile LSI_SCM_INFO_PATH "Mandatory file (scm-info.yaml) is not present, what should I lint?"`,
			`filename="$(echo $LSI_SCHEMA_REPO | sed 's;.*/;;g')"`,
			`checkEnv filename`,
			`wget -Y off -q "${LSI_SCHEMA_REPO}"`,
			`mv ${filename} schema.yaml`,
			`export LSI_SCHEMA_PATH=schema.yaml`,
			`checkFile LSI_SCHEMA_PATH "schema is necessary for determining compliance of file"`,
			`yaml-validator ${LSI_ADDITIONAL_ARGS}`,
		)

	// ── lint_python (image with entrypoint override, allow_failure on exit codes, codequality report) ──
	lintPython := baseLint.Clone(autoTest, "lint_python").
		Image("ghcr-remote.bahnhub.tech.rz.db.de/astral-sh/ruff:0.15.8-alpine@sha256:4696568cdca700e0dbfb2c440a4a83287f4f2a5db9b852e627eb98147139066e").
		ImageEntrypoint("").
		Env("LP_CODE_QUALITY_REPORT_FILE", "gl-code-quality.json").
		Env("WORKON_HOME", "${CI_PROJECT_DIR}/.local").
		Env("PYTHONUSERBASE", "${WORKON_HOME}").
		Env("LP_LINT_PATH", "pass").
		AllowFailureOnExitCodes(1).
		SetArtifacts(ps.Artifacts{
			Paths: []string{"${LP_CODE_QUALITY_REPORT_FILE}"},
			Reports: map[string][]string{
				"codequality": {"${LP_CODE_QUALITY_REPORT_FILE}"},
				"annotations": {"${CI_PROJECT_DIR}/branding.json"},
			},
		}).
		BeforeScript("cd tests/lint_python").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LP_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LP_LINT_PATH "."`,
			`log_config "LP_"`,
			`ruff check --exit-zero ${LP_ADDITIONAL_ARGS} ${LP_LINT_PATH}`,
			`ruff check --output-format=gitlab --output-file=${LP_CODE_QUALITY_REPORT_FILE} ${LP_ADDITIONAL_ARGS} ${LP_LINT_PATH}`,
		)

	// ── lint_kotlin (allow_failure on exit codes) ──
	lintKotlin := baseLint.Clone(autoTest, "lint_kotlin").
		Image("db-cd-lib-docker-release-local.bahnhub.tech.rz.db.de/kotlin-detekt:0.0.2064@sha256:884385379f852d622a2340777fee5d1d0d0e09313d9362413fade2f2b3b6bdff").
		Env("LK_OUTPUT_FORMAT", "html").
		Env("LK_OUTPUT_PATH", "reports/detekt.${LK_OUTPUT_FORMAT}").
		Env("LK_ADDITIONAL_ARGS", `--config detekt.yml`).
		AllowFailureOnExitCodes(2).
		SetArtifacts(ps.Artifacts{
			Paths: []string{"${LK_OUTPUT_PATH}"},
			Reports: map[string][]string{
				"annotations": {"${CI_PROJECT_DIR}/branding.json"},
			},
		}).
		BeforeScript("cd tests/lint_kotlin/pass").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LK_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LK_ADDITIONAL_ARGS ""`,
			`log_config "LK_"`,
			`export PROJECT_PATH_WITHOUT_GROUP=$(echo ${CI_PROJECT_PATH} | sed 's/^[^\/]*\///g')`,
			`export LK_EXIT_CODE=0`,
			`detekt ${LK_ADDITIONAL_ARGS} -r "${LK_OUTPUT_FORMAT}:${LK_OUTPUT_PATH}" || export LK_EXIT_CODE=${?}`,
			`exit ${LK_EXIT_CODE}`,
		)

	// ── lint_xml ──
	lintXML := baseLint.Clone(autoTest, "lint_xml").
		Image("db-cd-lib-docker-release-local.bahnhub.tech.rz.db.de/xmllint@sha256:27fcd09dce07237a8945a253f325b5f797189aaa6d5a01d3384452601c7d3312").
		Env("LX_LINT_TARGET_DIRECTORY", "tests/lint_xml/pass").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LX_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LX_LINT_TARGET_DIRECTORY "${CI_PROJECT_DIR}"`,
			`checkEnv LX_LINT_TARGET_DIRECTORY "need to know where to lint"`,
			`log_config "LX_"`,
			"cd ${LX_LINT_TARGET_DIRECTORY}\nfind -type f -iname \"**.xml\" > LX_FILES.lst\ncat LX_FILES.lst | while read -r file; do\n\tlog_info \"Linting file $file\"\n\txmllint ${LX_ADDITIONAL_ARGS} $file\n\techo \" \"\ndone",
		)

	// ── lint_dockerfile (image with entrypoint override) ──
	lintDockerfile := baseLint.Clone(autoTest, "lint_dockerfile").
		Image("docker-hub-remote.bahnhub.tech.rz.db.de/hadolint/hadolint:latest-alpine@sha256:7aba693c1442eb31c0b015c129697cb3b6cb7da589d85c7562f9deb435a6657c").
		ImageEntrypoint("").
		SetArtifacts(ps.Artifacts{
			Paths: []string{"hadolint-results.json"},
			Reports: map[string][]string{
				"annotations": {"${CI_PROJECT_DIR}/branding.json"},
			},
		}).
		BeforeScript("cd tests/lint_docker/pass").
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LD_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LD_DOCKERFILE_PATH "Dockerfile"`,
			`log_config "LD_"`,
			"if [ -f \"${LD_DOCKERFILE_PATH}\" ]; then\n  log_info \"Linting Dockerfile at path ${LD_DOCKERFILE_PATH}\"\nelse\n  log_info \"No Dockerfile at path ${LD_DOCKERFILE_PATH} was found, nothing to do\"\n  exit 0\nfi",
			`hadolint -f json --no-fail ${LD_ADDITIONAL_REPORT_ARGS} ${LD_DOCKERFILE_PATH} > hadolint-results.json`,
			`hadolint ${LD_ADDITIONAL_ARGS} ${LD_DOCKERFILE_PATH}`,
		)

	// ── lint_cloudformation (allow_failure on multiple exit codes, junit report) ──
	lintCF := baseLint.Clone(autoTest, "lint_cloudformation").
		Image("db-cd-lib-docker-release-local.bahnhub.tech.rz.db.de/cfn-lint:1.0.1247@sha256:bd6b8bba6fae582c48d55f64001f26fc5b8a43360281f6c126f968172887fece").
		Env("LCF_PATH_TO_LINT", "tests/lint_cloudformation/pass/lint_cloudformation_pass_cf.yaml").
		AllowFailureOnExitCodes(2, 4, 6, 8, 10, 12, 14).
		SetArtifacts(ps.Artifacts{
			Reports: map[string][]string{
				"junit":       {"junit_report.xml"},
				"annotations": {"${CI_PROJECT_DIR}/branding.json"},
			},
		}).
		Script(
			`source "${CI_BUILDS_DIR}/.create_module_annotations.${CI_JOB_ID}.sh"`,
			`[ "${LCF_JOB_DISABLED}" = "true" ] && exit 0`,
			`source "${CI_BUILDS_DIR}/.pipeline_utils.${CI_JOB_ID}.sh"`,
			`default LCF_PATH_TO_LINT "**/**cf.**"`,
			`checkEnv LCF_PATH_TO_LINT "Path to lint shouldn't be empty."`,
			`checkFile LCF_PATH_TO_LINT "There was no cloud formation template found"`,
			`log_config "LCF_"`,
			`cfn-lint ${LCF_ADDITIONAL_ARGS} ${LCF_PATH_TO_LINT} -f pretty || true`,
			`set +e`,
			`cfn-lint ${LCF_ADDITIONAL_ARGS} ${LCF_PATH_TO_LINT} -f junit > junit_report.xml`,
		)

	// ── _fail variants: clone pass jobs, override test paths and allow_failure ──
	lintAlex.Clone(autoTest, "lint_alex_fail").
		Env("LA_TARGET_PATH", "fail")

	lintScmInfo.Clone(autoTest, "lint_scm_info_fail").
		BeforeScript("cd tests/lint_scm_info/fail").
		AllowFailure()

	lintPython.Clone(autoTest, "lint_python_fail").
		Env("LP_LINT_PATH", "fail").
		Env("LP_FORMAT", "true").
		AllowFailure()

	lintKotlin.Clone(autoTest, "lint_kotlin_fail").
		Env("LK_ADDITIONAL_ARGS", `--excludes '**/ignore/**'`).
		BeforeScript("cd tests/lint_kotlin/fail").
		AllowFailure()

	lintXML.Clone(autoTest, "lint_xml_fail").
		Env("LX_LINT_TARGET_DIRECTORY", "tests/lint_xml/fail").
		AllowFailure()

	lintDockerfile.Clone(autoTest, "lint_dockerfile_fail").
		BeforeScript("cd tests/lint_docker/fail").
		AllowFailure()

	lintCF.Clone(autoTest, "lint_cloudformation_fail").
		Env("LCF_PATH_TO_LINT", "tests/lint_cloudformation/fail/lint_cloudformation_fail_cf.yaml").
		AllowFailure()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
