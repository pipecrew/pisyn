package runner

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pipecrew/pisyn/pkg/pisyn"
)

// DockerRunner manages Docker container lifecycle for local pipeline execution.
type DockerRunner struct {
	cli          client.APIClient
	networkID    string
	workDir      string
	workspaceVol string // Docker volume holding the workspace copy
	localVars    map[string]string
}

// NewDockerRunner creates a runner connected to the local Docker daemon.
// Verifies connectivity before returning.
func NewDockerRunner(workDir string) (*DockerRunner, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	// Resolve Docker host: prefer DOCKER_HOST env, fall back to active docker context
	if os.Getenv("DOCKER_HOST") == "" {
		if host := dockerHostFromContext(); host != "" {
			opts = append(opts, client.WithHost(host))
		}
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}

	// Verify Docker is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close() // best-effort cleanup on connection failure
		return nil, fmt.Errorf("cannot connect to Docker: %w (is Docker running?)", err)
	}

	return &DockerRunner{
		cli:       cli,
		workDir:   workDir,
		localVars: ResolveLocalVars(workDir),
	}, nil
}

// dockerHostFromContext reads the Docker host from the active docker context.
func dockerHostFromContext() string {
	out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CreateNetwork creates a bridge network for this run.
func (d *DockerRunner) CreateNetwork(ctx context.Context, name string) error {
	resp, err := d.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	d.networkID = resp.ID
	return nil
}

// RemoveNetwork removes the run network.
func (d *DockerRunner) RemoveNetwork(ctx context.Context) error {
	if d.networkID == "" {
		return nil
	}
	return d.cli.NetworkRemove(ctx, d.networkID)
}

const helperImage = "busybox:latest"

// ensureHelperImage makes sure the helper image used for workspace copy is available.
func (d *DockerRunner) ensureHelperImage(ctx context.Context) error {
	_, err := d.cli.ImageInspect(ctx, helperImage)
	if err == nil {
		return nil
	}
	reader, err := d.cli.ImagePull(ctx, helperImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull helper image %s: %w", helperImage, err)
	}
	_, _ = io.Copy(io.Discard, reader) // drain pull response
	_ = reader.Close()
	return nil
}

// CreateWorkspace copies the project directory into a Docker volume.
// Jobs mount this volume instead of the host directory, keeping local files untouched.
func (d *DockerRunner) CreateWorkspace(ctx context.Context, name string) error {
	_, err := d.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("create workspace volume: %w", err)
	}
	d.workspaceVol = name

	if err := d.ensureHelperImage(ctx); err != nil {
		return err
	}

	// Create a stopped container with the volume, then use CopyToContainer
	resp, err := d.cli.ContainerCreate(ctx,
		&container.Config{Image: helperImage, Cmd: []string{"true"}},
		&container.HostConfig{Binds: []string{name + ":/workspace"}},
		nil, nil, "")
	if err != nil {
		return fmt.Errorf("create workspace helper: %w", err)
	}
	defer func() { _ = d.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }() // best-effort cleanup

	// Tar the project directory and copy into the container's /workspace
	tarBuf, err := tarDir(d.workDir)
	if err != nil {
		return fmt.Errorf("tar workspace: %w", err)
	}
	if err := d.cli.CopyToContainer(ctx, resp.ID, "/workspace", tarBuf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("copy to workspace volume: %w", err)
	}
	return nil
}

// RemoveWorkspace removes the workspace volume.
func (d *DockerRunner) RemoveWorkspace(ctx context.Context) error {
	if d.workspaceVol == "" {
		return nil
	}
	return d.cli.VolumeRemove(ctx, d.workspaceVol, true)
}

// RunJob executes a single job in a Docker container, streaming logs to the events channel.
// extraEnv contains output variables from upstream jobs.
func (d *DockerRunner) RunJob(ctx context.Context, job *pisyn.Job, extraEnv map[string]string, events chan<- Event) error {
	img := job.ImageName
	if img == "" {
		img = "alpine:latest"
	}

	script := d.buildScript(job)
	if script == "" {
		events <- Event{Type: EventJobLog, JobName: job.JobName, Log: "⚠️  no scripts to execute"}
		return nil
	}

	// Pull image (skip if already available locally)
	if err := d.ensureImage(ctx, img, job.JobName, events); err != nil {
		return err
	}

	// Start services
	serviceIDs, err := d.startServices(ctx, job, events)
	if err != nil {
		return err
	}
	defer d.stopContainers(context.Background(), serviceIDs)

	// Apply timeout
	if job.TimeoutMin > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(job.TimeoutMin)*time.Minute)
		defer cancel()
	}

	// Create container
	cfg := &container.Config{
		Image:      img,
		Cmd:        []string{"sh", "-e", "-c", script},
		Env:        d.buildEnv(job, extraEnv),
		WorkingDir: "/workspace",
	}
	if job.ImageEP != nil {
		cfg.Entrypoint = job.ImageEP
	}

	binds := []string{d.workspaceVol + ":/workspace"}
	if job.CacheCfg != nil && job.CacheCfg.Key != "" {
		for _, cachePath := range job.CacheCfg.Paths {
			volName := "pisyn-cache-" + job.CacheCfg.Key
			binds = append(binds, volName+":"+cachePath)
		}
	}

	hostCfg := &container.HostConfig{
		Binds: binds,
	}

	netCfg := &network.NetworkingConfig{}
	if d.networkID != "" {
		netCfg.EndpointsConfig = map[string]*network.EndpointSettings{
			d.networkID: {},
		}
	}

	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "")
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	containerID := resp.ID
	defer func() { _ = d.cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}) }() // best-effort cleanup

	// Start container
	if err := d.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	// Attach logs after starting
	logReader, err := d.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true,
	})
	if err != nil {
		return fmt.Errorf("attach logs: %w", err)
	}
	defer func() { _ = logReader.Close() }() // best-effort cleanup

	// Stream logs in background
	logDone := make(chan struct{})
	go func() {
		d.streamLogs(ctx, logReader, job.JobName, events)
		close(logDone)
	}()

	// Wait for exit
	waitCh, errCh := d.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case result := <-waitCh:
		<-logDone // drain remaining logs
		if result.StatusCode != 0 {
			return fmt.Errorf("exit code %d", result.StatusCode)
		}
		return nil
	case err := <-errCh:
		return err
	case <-ctx.Done():
		_ = d.cli.ContainerStop(context.Background(), containerID, container.StopOptions{}) // best-effort on timeout
		return ctx.Err()
	}
}

// ensureImage pulls the image only if not available locally.
func (d *DockerRunner) ensureImage(ctx context.Context, img, jobName string, events chan<- Event) error {
	_, err := d.cli.ImageInspect(ctx, img)
	if err == nil {
		events <- Event{Type: EventJobLog, JobName: jobName, Log: fmt.Sprintf("Using cached image %s", img)}
		return nil // already available
	}
	events <- Event{Type: EventJobLog, JobName: jobName, Log: fmt.Sprintf("Pulling %s...", img)}
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull %s: %w", img, err)
	}
	_, _ = io.Copy(io.Discard, reader) // drain pull response
	_ = reader.Close()
	return nil
}

func (d *DockerRunner) startServices(ctx context.Context, job *pisyn.Job, events chan<- Event) ([]string, error) {
	var ids []string
	for _, svc := range job.ServiceList {
		events <- Event{Type: EventJobLog, JobName: job.JobName, Log: fmt.Sprintf("Starting service %s...", svc.Alias)}

		if err := d.ensureImage(ctx, svc.Image, job.JobName, events); err != nil {
			d.stopContainers(ctx, ids)
			return nil, err
		}

		var env []string
		for key, val := range svc.Variables {
			env = append(env, key+"="+val)
		}

		netCfg := &network.NetworkingConfig{}
		if d.networkID != "" {
			netCfg.EndpointsConfig = map[string]*network.EndpointSettings{
				d.networkID: {Aliases: []string{svc.Alias}},
			}
		}

		resp, err := d.cli.ContainerCreate(ctx,
			&container.Config{Image: svc.Image, Env: env},
			nil, netCfg, nil, "")
		if err != nil {
			d.stopContainers(ctx, ids)
			return nil, fmt.Errorf("create service %s: %w", svc.Alias, err)
		}
		if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			d.stopContainers(ctx, ids)
			return nil, fmt.Errorf("start service %s: %w", svc.Alias, err)
		}
		ids = append(ids, resp.ID)
	}
	return ids, nil
}

func (d *DockerRunner) stopContainers(ctx context.Context, ids []string) {
	for _, id := range ids {
		_ = d.cli.ContainerStop(ctx, id, container.StopOptions{})       // best-effort cleanup
		_ = d.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}) // best-effort cleanup
	}
}

func (d *DockerRunner) buildScript(job *pisyn.Job) string {
	before := job.BeforeScriptLines()
	main := job.ScriptLines()
	after := job.AfterScriptLines()

	if len(before) == 0 && len(main) == 0 && len(after) == 0 {
		return ""
	}

	// If there's an after_script, wrap with trap so it runs even on failure
	if len(after) > 0 {
		var parts []string
		parts = append(parts, "_pisyn_after() {")
		parts = append(parts, after...)
		parts = append(parts, "}")
		parts = append(parts, "trap _pisyn_after EXIT")
		parts = append(parts, before...)
		parts = append(parts, main...)
		return strings.Join(parts, "\n")
	}

	var parts []string
	parts = append(parts, before...)
	parts = append(parts, main...)
	return strings.Join(parts, "\n")
}

func (d *DockerRunner) buildEnv(job *pisyn.Job, extraEnv map[string]string) []string {
	expanded := ExpandVars(job.EnvVars, d.localVars)

	// Merge: local vars + upstream outputs + job vars (later overrides earlier)
	merged := make(map[string]string, len(d.localVars)+len(extraEnv)+len(expanded)+1)
	for key, val := range d.localVars {
		merged[key] = val
	}
	for key, val := range extraEnv {
		merged[key] = val
	}
	for key, val := range expanded {
		merged[key] = val
	}
	merged["PISYN_JOB_NAME"] = job.JobName

	// Expand PISYN_OUTPUT_* references in all values
	replacerPairs := make([]string, 0, len(extraEnv)*2)
	for key, val := range extraEnv {
		replacerPairs = append(replacerPairs, "$"+key, val)
	}
	if len(replacerPairs) > 0 {
		replacer := strings.NewReplacer(replacerPairs...)
		for key, val := range merged {
			merged[key] = replacer.Replace(val)
		}
	}

	// Sort for deterministic output
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+merged[key])
	}
	return env
}

func (d *DockerRunner) streamLogs(ctx context.Context, reader io.Reader, jobName string, events chan<- Event) {
	// Docker multiplexed stream: 8-byte header + payload per frame.
	// stdcopy.StdCopy demuxes; we merge stdout+stderr into one writer.
	pr, pw := io.Pipe()
	go func() {
		_, _ = stdcopy.StdCopy(pw, pw, reader) // demux docker stream; errors surface as EOF
		_ = pw.Close()                         // signal scanner to stop
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		select {
		case events <- Event{Type: EventJobLog, JobName: jobName, Log: scanner.Text()}:
		case <-ctx.Done():
			_ = pr.Close() // unblock stdcopy → pw.Close() → scanner exits
			return
		}
	}
}

// CollectOutputs reads dotenv files declared by the job from the workspace volume.
// Keys are in PISYN_OUTPUT_<JOB>__<NAME> format plus the raw variable name.
func (d *DockerRunner) CollectOutputs(ctx context.Context, job *pisyn.Job) map[string]string {
	outputs := map[string]string{}
	for _, out := range job.OutputList {
		data := d.readFileFromVolume(ctx, out.DotenvFile)
		if data == "" {
			continue
		}
		for _, line := range strings.Split(data, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				outputs[k] = v
				ref := fmt.Sprintf("PISYN_OUTPUT_%s__%s",
					strings.ToUpper(strings.ReplaceAll(job.JobName, "-", "_")),
					strings.ToUpper(k))
				outputs[ref] = v
			}
		}
	}
	return outputs
}

// readFileFromVolume reads a file from the workspace volume using CopyFromContainer.
func (d *DockerRunner) readFileFromVolume(ctx context.Context, filePath string) string {
	resp, err := d.cli.ContainerCreate(ctx,
		&container.Config{Image: helperImage, Cmd: []string{"true"}},
		&container.HostConfig{Binds: []string{d.workspaceVol + ":/workspace:ro"}},
		nil, nil, "")
	if err != nil {
		return ""
	}
	defer func() { _ = d.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }() // best-effort cleanup

	reader, _, err := d.cli.CopyFromContainer(ctx, resp.ID, "/workspace/"+filePath)
	if err != nil {
		return ""
	}
	defer func() { _ = reader.Close() }() // best-effort cleanup

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		return ""
	}
	data, err := io.ReadAll(tr)
	if err != nil {
		return ""
	}
	return string(data)
}

// tarDir creates a tar archive of the directory contents.
func tarDir(dir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }() // read-only, safe to ignore
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("finalize tar: %w", err)
	}
	return &buf, nil
}

// Close releases the Docker client.
func (d *DockerRunner) Close() error {
	return d.cli.Close()
}
