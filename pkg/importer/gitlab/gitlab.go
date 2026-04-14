// Package gitlab parses GitLab CI YAML files into pisyn IR structs.
package gitlab

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pipecrew/pisyn/pkg/pisyn"
	"gopkg.in/yaml.v3"
)

// knownTopLevel lists GitLab CI top-level keys that are not job definitions.
var knownTopLevel = map[string]bool{
	"stages": true, "variables": true, "workflow": true,
	"default": true, "include": true, "image": true,
}

// ParseResult holds the parsed pipeline and any hidden job templates.
type ParseResult struct {
	Pipeline  *pisyn.IRPipeline
	Templates []pisyn.IRJob
}

// Parse parses a GitLab CI YAML document and returns a ParseResult.
func Parse(data []byte) (*ParseResult, error) {
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	p := &pisyn.IRPipeline{Name: "CI"}
	result := &ParseResult{Pipeline: p}

	if node, ok := raw["stages"]; ok {
		var stages []string
		if err := node.Decode(&stages); err != nil {
			return nil, fmt.Errorf("decode stages: %w", err)
		}
		for _, s := range stages {
			p.Stages = append(p.Stages, pisyn.IRStage{Name: s})
		}
	}

	if node, ok := raw["variables"]; ok {
		p.Env = decodeStringMap(&node)
	}

	if node, ok := raw["workflow"]; ok {
		var wf struct {
			Rules []ruleYAML `yaml:"rules"`
		}
		if err := node.Decode(&wf); err == nil {
			for _, r := range wf.Rules {
				p.WorkflowRules = append(p.WorkflowRules, r.toRule())
			}
		}
	}

	if node, ok := raw["default"]; ok {
		var d pisyn.JobDefaults
		if err := node.Decode(&d); err == nil {
			p.Defaults = &d
		}
	}

	if node, ok := raw["include"]; ok {
		p.Includes = parseIncludes(&node)
	}

	// Everything else is a job definition
	stageIndex := map[string]int{}
	for i, s := range p.Stages {
		stageIndex[s.Name] = i
	}

	for key, node := range raw {
		if knownTopLevel[key] {
			continue
		}
		if strings.HasPrefix(key, ".") {
			job, err := parseJob(key, &node)
			if err != nil {
				return nil, fmt.Errorf("parse template %q: %w", key, err)
			}
			result.Templates = append(result.Templates, job.IRJob)
			continue
		}
		job, err := parseJob(key, &node)
		if err != nil {
			return nil, fmt.Errorf("parse job %q: %w", key, err)
		}
		idx, ok := stageIndex[job.stage]
		if !ok {
			// Stage not declared — create it
			idx = len(p.Stages)
			p.Stages = append(p.Stages, pisyn.IRStage{Name: job.stage})
			stageIndex[job.stage] = idx
		}
		p.Stages[idx].Jobs = append(p.Stages[idx].Jobs, job.IRJob)
	}

	return result, nil
}

type parsedJob struct {
	pisyn.IRJob
	stage string
}

// ruleYAML matches GitLab CI rule YAML structure.
type ruleYAML struct {
	If           string            `yaml:"if"`
	When         string            `yaml:"when"`
	AllowFailure bool              `yaml:"allow_failure"`
	Changes      []string          `yaml:"changes"`
	Exists       []string          `yaml:"exists"`
	Variables    map[string]string `yaml:"variables"`
}

func (r ruleYAML) toRule() pisyn.Rule {
	return pisyn.Rule{
		If: r.If, When: r.When, AllowFailure: r.AllowFailure,
		Changes: r.Changes, Exists: r.Exists, Variables: r.Variables,
	}
}

// jobYAML matches the GitLab CI job YAML structure.
type jobYAML struct {
	Stage        string         `yaml:"stage"`
	Image        yaml.Node      `yaml:"image"`
	Script       yaml.Node      `yaml:"script"`
	BeforeScript yaml.Node      `yaml:"before_script"`
	AfterScript  yaml.Node      `yaml:"after_script"`
	Needs        yaml.Node  `yaml:"needs"`
	Dependencies []string       `yaml:"dependencies"`
	Variables    map[string]string `yaml:"variables"`
	Artifacts    *artifactsYAML `yaml:"artifacts"`
	Cache        *cacheYAML     `yaml:"cache"`
	Services     []yaml.Node    `yaml:"services"`
	Rules        yaml.Node     `yaml:"rules"`
	When         string         `yaml:"when"`
	AllowFailure yaml.Node      `yaml:"allow_failure"`
	Retry        yaml.Node      `yaml:"retry"`
	Timeout      string         `yaml:"timeout"`
	Tags         []string       `yaml:"tags"`
	Interruptible *bool         `yaml:"interruptible"`
	Environment  yaml.Node      `yaml:"environment"`
	Parallel     *parallelYAML  `yaml:"parallel"`
}

type artifactsYAML struct {
	Paths    []string        `yaml:"paths"`
	Name     string          `yaml:"name"`
	ExpireIn string          `yaml:"expire_in"`
	When     string          `yaml:"when"`
	Reports  yaml.Node       `yaml:"reports"`
}

type cacheYAML struct {
	Key   string   `yaml:"key"`
	Paths []string `yaml:"paths"`
}

type parallelYAML struct {
	Matrix []map[string][]string `yaml:"matrix"`
}

func parseJob(name string, node *yaml.Node) (*parsedJob, error) {
	var j jobYAML
	if err := node.Decode(&j); err != nil {
		return nil, err
	}

	ir := pisyn.IRJob{
		Name:          name,
		Needs:         parseNeeds(&j.Needs),
		Dependencies:  j.Dependencies,
		Env:           j.Variables,
		Tags:          j.Tags,
		Interruptible: j.Interruptible,
		Runner:        "ubuntu-latest",
	}

	// needs: [] means "start immediately" — distinct from no needs key
	if j.Needs.Kind == yaml.SequenceNode && len(j.Needs.Content) == 0 {
		ir.EmptyNeeds = true
	}

	// Image
	parseImage(&j.Image, &ir)

	// Scripts
	ir.Actions = parseActions(&j.BeforeScript, &j.Script, &j.AfterScript)

	// Rules
	ir.Rules = parseRules(&j.Rules)

	// When
	switch j.When {
	case "manual":
		ir.When = pisyn.Manual
	case "always":
		ir.When = pisyn.Always
	case "on_failure":
		ir.When = pisyn.OnFailure
	}

	// Allow failure
	parseAllowFailure(&j.AllowFailure, &ir)

	// Retry
	parseRetry(&j.Retry, &ir)

	// Timeout
	if j.Timeout != "" {
		ir.TimeoutMin = parseTimeout(j.Timeout)
	}

	// Artifacts
	if j.Artifacts != nil {
		reports := parseReports(&j.Artifacts.Reports)
		a := &pisyn.Artifacts{
			Paths: j.Artifacts.Paths, Name: j.Artifacts.Name,
			ExpireIn: j.Artifacts.ExpireIn, Reports: reports,
		}
		// Extract dotenv outputs
		if dotenvFiles, ok := a.Reports["dotenv"]; ok {
			for _, f := range dotenvFiles {
				ir.Outputs = append(ir.Outputs, pisyn.JobOutput{Name: strings.TrimSuffix(strings.ToUpper(f), ".ENV"), DotenvFile: f})
			}
			delete(a.Reports, "dotenv")
			if len(a.Reports) == 0 {
				a.Reports = nil
			}
		}
		if len(a.Paths) > 0 || a.Name != "" || a.ExpireIn != "" || len(a.Reports) > 0 {
			ir.Artifacts = a
		}
	}

	// Cache
	if j.Cache != nil {
		ir.Cache = &pisyn.Cache{Key: j.Cache.Key, Paths: j.Cache.Paths}
	}

	// Services
	for _, svcNode := range j.Services {
		ir.Services = append(ir.Services, parseService(&svcNode))
	}

	// Environment
	parseEnvironment(&j.Environment, &ir)

	// Matrix
	if j.Parallel != nil && len(j.Parallel.Matrix) > 0 {
		ir.Matrix = &pisyn.Matrix{Dimensions: j.Parallel.Matrix[0]}
	}

	stage := j.Stage
	if stage == "" {
		stage = "test" // GitLab default
	}

	return &parsedJob{IRJob: ir, stage: stage}, nil
}

func parseImage(node *yaml.Node, ir *pisyn.IRJob) {
	if node.Kind == 0 {
		return
	}
	if node.Kind == yaml.ScalarNode {
		ir.Image = node.Value
		return
	}
	// Object form: {name, entrypoint, docker: {user}}
	var img struct {
		Name       string   `yaml:"name"`
		Entrypoint []string `yaml:"entrypoint"`
		Docker     struct {
			User string `yaml:"user"`
		} `yaml:"docker"`
	}
	if err := node.Decode(&img); err == nil {
		ir.Image = img.Name
		ir.ImageEntrypoint = img.Entrypoint
		ir.ImageUser = img.Docker.User
	}
}

func parseActions(before, script, after *yaml.Node) []pisyn.Action {
	var actions []pisyn.Action
	if lines := decodeStringSlice(before); len(lines) > 0 {
		s := strings.Join(lines, "\n")
		actions = append(actions, pisyn.Action{Script: &s, Phase: pisyn.PhaseBefore})
	}
	if lines := decodeStringSlice(script); len(lines) > 0 {
		for _, line := range lines {
			l := line
			actions = append(actions, pisyn.Action{Script: &l})
		}
	}
	if lines := decodeStringSlice(after); len(lines) > 0 {
		s := strings.Join(lines, "\n")
		actions = append(actions, pisyn.Action{Script: &s, Phase: pisyn.PhaseAfter})
	}
	return actions
}

func parseAllowFailure(node *yaml.Node, ir *pisyn.IRJob) {
	if node.Kind == 0 {
		return
	}
	if node.Kind == yaml.ScalarNode {
		if node.Value == "true" {
			ir.AllowFailure = true
		}
		return
	}
	var af struct {
		ExitCodes yaml.Node `yaml:"exit_codes"`
	}
	if err := node.Decode(&af); err == nil && af.ExitCodes.Kind != 0 {
		codes := decodeIntSlice(&af.ExitCodes)
		ir.AllowFailureCfg = &pisyn.AllowFailureConfig{Enabled: true, ExitCodes: codes}
	}
}

func parseRetry(node *yaml.Node, ir *pisyn.IRJob) {
	if node.Kind == 0 {
		return
	}
	if node.Kind == yaml.ScalarNode {
		if n, err := strconv.Atoi(node.Value); err == nil {
			ir.RetryCount = n
		}
		return
	}
	var r struct {
		Max  int      `yaml:"max"`
		When []string `yaml:"when"`
	}
	if err := node.Decode(&r); err == nil {
		ir.RetryConfig = &pisyn.RetryConfig{Max: r.Max, When: r.When}
	}
}

func parseTimeout(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "m")); err == nil {
			return n
		}
	}
	if strings.HasSuffix(s, "h") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "h")); err == nil {
			return n * 60
		}
	}
	return 0
}

func parseRules(node *yaml.Node) []pisyn.Rule {
	if node.Kind == 0 {
		return nil
	}
	// Try flat list of rules
	var flat []ruleYAML
	if err := node.Decode(&flat); err == nil {
		rules := make([]pisyn.Rule, len(flat))
		for i, r := range flat {
			rules[i] = r.toRule()
		}
		return rules
	}
	// Nested rules (list of lists) — flatten
	var nested []yaml.Node
	if err := node.Decode(&nested); err == nil {
		var rules []pisyn.Rule
		for _, n := range nested {
			var r ruleYAML
			if n.Decode(&r) == nil && (r.If != "" || r.When != "") {
				rules = append(rules, r.toRule())
			}
			// Try as nested list
			var sub []ruleYAML
			if n.Decode(&sub) == nil {
				for _, s := range sub {
					rules = append(rules, s.toRule())
				}
			}
		}
		return rules
	}
	return nil
}

func parseReports(node *yaml.Node) map[string][]string {
	if node.Kind == 0 {
		return nil
	}
	// Try standard form: map[string][]string
	var standard map[string][]string
	if err := node.Decode(&standard); err == nil {
		return standard
	}
	// Mixed form: some values are []string, some are maps — extract only []string ones
	reports := map[string][]string{}
	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err == nil {
		for k, v := range raw {
			var ss []string
			if v.Decode(&ss) == nil {
				reports[k] = ss
			}
			// Skip map-form reports (coverage_report, etc.) — not supported by pisyn
		}
	}
	if len(reports) == 0 {
		return nil
	}
	return reports
}

func parseNeeds(node *yaml.Node) []string {
	if node.Kind == 0 {
		return nil
	}
	// Try simple string list first
	var ss []string
	if err := node.Decode(&ss); err == nil {
		return ss
	}
	// Object form: [{job: "name", ...}]
	var objs []struct {
		Job string `yaml:"job"`
	}
	if err := node.Decode(&objs); err == nil {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			if o.Job != "" {
				out = append(out, o.Job)
			}
		}
		return out
	}
	return nil
}

func parseService(node *yaml.Node) pisyn.Service {
	if node.Kind == yaml.ScalarNode {
		return pisyn.Service{Image: node.Value}
	}
	var svc struct {
		Name      string            `yaml:"name"`
		Alias     string            `yaml:"alias"`
		Variables map[string]string `yaml:"variables"`
	}
	if err := node.Decode(&svc); err == nil {
		return pisyn.Service{Image: svc.Name, Alias: svc.Alias, Variables: svc.Variables}
	}
	return pisyn.Service{}
}

func parseEnvironment(node *yaml.Node, ir *pisyn.IRJob) {
	if node.Kind == 0 {
		return
	}
	if node.Kind == yaml.ScalarNode {
		ir.Environment = &pisyn.Environment{Name: node.Value}
		return
	}
	var env struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	}
	if err := node.Decode(&env); err == nil {
		ir.Environment = &pisyn.Environment{Name: env.Name, URL: env.URL}
	}
}

func parseIncludes(node *yaml.Node) []pisyn.Include {
	// includes can be a single map or a list of maps
	var includes []pisyn.Include
	var list []struct {
		Local    string `yaml:"local"`
		Remote   string `yaml:"remote"`
		Project  string `yaml:"project"`
		File     string `yaml:"file"`
		Ref      string `yaml:"ref"`
		Template string `yaml:"template"`
	}
	if err := node.Decode(&list); err == nil {
		for _, inc := range list {
			includes = append(includes, pisyn.Include{
				Local: inc.Local, Remote: inc.Remote,
				Project: inc.Project, File: inc.File, Ref: inc.Ref,
				Template: inc.Template,
			})
		}
	}
	return includes
}

func decodeStringSlice(node *yaml.Node) []string {
	if node.Kind == 0 {
		return nil
	}
	var s []string
	_ = node.Decode(&s)
	return s
}

// decodeStringMap parses a YAML mapping into map[string]string, handling
// GitLab CI's variable formats:
//   - scalar values (string, int, bool) — used as-is via yaml.Node.Value
//   - expanded form ({value: "x", description: "..."}) — extracts the value field
func decodeStringMap(node *yaml.Node) map[string]string {
	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return nil
	}
	m := make(map[string]string, len(raw))
	for k, v := range raw {
		switch v.Kind {
		case yaml.ScalarNode:
			// Covers strings, integers, and booleans — Node.Value is the string representation
			m[k] = v.Value
		case yaml.MappingNode:
			// Expanded form: {value: "x", description: "..."}
			var expanded struct {
				Value string `yaml:"value"`
			}
			if v.Decode(&expanded) == nil && expanded.Value != "" {
				m[k] = expanded.Value
			}
		}
	}
	return m
}

func decodeIntSlice(node *yaml.Node) []int {
	if node.Kind == yaml.ScalarNode {
		if n, err := strconv.Atoi(node.Value); err == nil {
			return []int{n}
		}
		return nil
	}
	var s []int
	_ = node.Decode(&s)
	return s
}
