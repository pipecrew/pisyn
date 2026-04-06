package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pipecrew/pisyn/pkg/runner"
)

const jobPanelWidth = 28

// JobState tracks a job's display state.
type JobState struct {
	Name         string
	Status       runner.JobStatus
	Elapsed      time.Duration
	AllowFailure bool
}

// Model is the bubbletea model for the pisyn run TUI.
type Model struct {
	jobs      []JobState
	jobIndex  map[string]int // job name → index in jobs slice
	selected  int
	logs      map[string][]string
	viewport  viewport.Model
	events    chan runner.Event
	cancel    context.CancelFunc // cancels the runner context on quit
	done      bool
	failed    bool
	startTime time.Time
	endTime   time.Time
	width     int
	height    int
}

// JobInfo carries job metadata needed by the TUI.
type JobInfo struct {
	Name         string
	AllowFailure bool
}

// NewModel creates a TUI model that consumes runner events.
func NewModel(jobInfos []JobInfo, events chan runner.Event, cancel context.CancelFunc) Model {
	jobs := make([]JobState, len(jobInfos))
	idx := make(map[string]int, len(jobInfos))
	for i, info := range jobInfos {
		jobs[i] = JobState{Name: info.Name, Status: runner.StatusPending, AllowFailure: info.AllowFailure}
		idx[info.Name] = i
	}
	return Model{
		jobs:      jobs,
		jobIndex:  idx,
		logs:      make(map[string][]string),
		viewport:  viewport.New(viewport.WithWidth(80), viewport.WithHeight(20)),
		events:    events,
		cancel:    cancel,
		startTime: time.Now(),
	}
}

// eventMsg wraps a runner event as a tea.Msg.
type eventMsg runner.Event

// channelClosedMsg signals the event channel was closed unexpectedly.
type channelClosedMsg struct{}

// tickMsg triggers periodic UI refresh.
type tickMsg time.Time

func waitForEvent(events chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return channelClosedMsg{}
		}
		return eventMsg(ev)
	}
}

func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.events), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel() // stop running containers
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.syncViewport()
			}
		case "down", "j":
			if m.selected < len(m.jobs)-1 {
				m.selected++
				m.syncViewport()
			}
		default:
			// Forward scroll keys (pgup, pgdown, home, end, etc.) to viewport
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		logH := m.height - 7
		if logH < 1 {
			logH = 1
		}
		logW := m.width - jobPanelWidth - 6
		if logW < 10 {
			logW = 10
		}
		m.viewport = viewport.New(viewport.WithWidth(logW), viewport.WithHeight(logH))
		m.syncViewport()

	case channelClosedMsg:
		if !m.done {
			m.done = true
			m.endTime = time.Now()
			m.failed = true
		}
		return m, nil

	case eventMsg:
		ev := runner.Event(msg)
		switch ev.Type {
		case runner.EventJobStarted:
			m.setJobStatus(ev.JobName, runner.StatusRunning)
			if i, ok := m.jobIndex[ev.JobName]; ok {
				m.selected = i
			}
		case runner.EventJobLog:
			m.logs[ev.JobName] = append(m.logs[ev.JobName], ev.Log)
			if m.selected < len(m.jobs) && m.jobs[m.selected].Name == ev.JobName {
				m.syncViewport()
			}
		case runner.EventJobFinished:
			m.setJobStatus(ev.JobName, ev.Status)
			m.setJobElapsed(ev.JobName, ev.Elapsed)
			if ev.Status == runner.StatusFailed {
				if i, ok := m.jobIndex[ev.JobName]; ok && !m.jobs[i].AllowFailure {
					m.failed = true
				}
			}
		case runner.EventRunComplete:
			m.done = true
			m.endTime = time.Now()
			if ev.Err != nil {
				m.failed = true
			}
			return m, nil
		}
		return m, waitForEvent(m.events)

	case tickMsg:
		if m.done {
			return m, nil
		}
		return m, tick()

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) setJobStatus(name string, status runner.JobStatus) {
	if i, ok := m.jobIndex[name]; ok {
		m.jobs[i].Status = status
	}
}

func (m *Model) setJobElapsed(name string, elapsed time.Duration) {
	if i, ok := m.jobIndex[name]; ok {
		m.jobs[i].Elapsed = elapsed
	}
}

func (m *Model) syncViewport() {
	if m.selected >= 0 && m.selected < len(m.jobs) {
		name := m.jobs[m.selected].Name
		lines := m.logs[name]
		m.viewport.SetContent(strings.Join(lines, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m Model) View() tea.View {
	if m.width == 0 {
		view := tea.NewView("Initializing...")
		view.AltScreen = true
		return view
	}

	// Job list panel
	var jobLines []string
	for i, j := range m.jobs {
		line := formatJob(j)
		if i == m.selected {
			line = styleSelected.Width(jobPanelWidth - 2).Render(line)
		}
		jobLines = append(jobLines, line)
	}

	panelH := m.height - 4
	if panelH < 1 {
		panelH = 1
	}

	jobContent := strings.Join(jobLines, "\n")
	jobPanel := styleJobPanel.
		Width(jobPanelWidth).
		Height(panelH).
		Render(styleTitle.Render("Jobs") + "\n" + jobContent)

	// Log panel
	selectedName := ""
	if m.selected >= 0 && m.selected < len(m.jobs) {
		selectedName = m.jobs[m.selected].Name
	}
	logTitle := styleTitle.Render(fmt.Sprintf("Logs (%s)", selectedName))
	logW := m.width - jobPanelWidth - 4
	if logW < 10 {
		logW = 10
	}
	logPanel := styleLogPanel.
		Width(logW).
		Height(panelH).
		Render(logTitle + "\n" + m.viewport.View())

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, jobPanel, logPanel)

	// Status bar
	elapsed := time.Since(m.startTime).Truncate(time.Millisecond)
	if m.done {
		elapsed = m.endTime.Sub(m.startTime).Truncate(time.Millisecond)
	}
	passed, failed, skipped, running, pending := m.countStatuses()
	var status string
	if m.done {
		result := stylePassed.Render("✅ PASSED")
		if m.failed {
			result = styleFailed.Render("❌ FAILED")
		}
		status = fmt.Sprintf(" %s  %d passed  %d failed  %d skipped │ %s │ ↑↓ select  pgup/pgdn scroll  q quit",
			result, passed, failed, skipped, elapsed)
	} else {
		status = fmt.Sprintf(" %d passed  %d failed  %d skipped  %d running  %d pending │ %s │ ↑↓ select  pgup/pgdn scroll  q quit",
			passed, failed, skipped, running, pending, elapsed)
	}
	bar := styleStatusBar.Width(m.width).Render(status)

	view := tea.NewView(panels + "\n" + bar)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) countStatuses() (passed, failed, skipped, running, pending int) {
	for _, j := range m.jobs {
		switch j.Status {
		case runner.StatusPassed:
			passed++
		case runner.StatusFailed:
			failed++
		case runner.StatusSkipped:
			skipped++
		case runner.StatusRunning:
			running++
		case runner.StatusPending:
			pending++
		}
	}
	return
}

// HasFailures returns true if any job failed.
func (m Model) HasFailures() bool {
	return m.failed
}

// FailedCount returns the number of failed jobs (excluding allow-failure).
func (m Model) FailedCount() int {
	count := 0
	for _, j := range m.jobs {
		if j.Status == runner.StatusFailed && !j.AllowFailure {
			count++
		}
	}
	return count
}

func formatJob(j JobState) string {
	icon := statusIcon(j.Status, j.AllowFailure)
	elapsed := ""
	if j.Elapsed > 0 {
		elapsed = fmt.Sprintf(" %s", j.Elapsed.Truncate(100*time.Millisecond))
	}
	name := icon + " " + j.Name + elapsed
	return jobStyle(j).Render(name)
}

func statusIcon(s runner.JobStatus, allowFailure bool) string {
	switch s {
	case runner.StatusPassed:
		return "✅"
	case runner.StatusFailed:
		if allowFailure {
			return "⚠️"
		}
		return "❌"
	case runner.StatusRunning:
		return "🔄"
	case runner.StatusSkipped:
		return "⏭"
	default:
		return "⏳"
	}
}

func jobStyle(j JobState) lipgloss.Style {
	switch j.Status {
	case runner.StatusPassed:
		return stylePassed
	case runner.StatusFailed:
		if j.AllowFailure {
			return styleAllowFailure
		}
		return styleFailed
	case runner.StatusRunning:
		return styleRunning
	case runner.StatusSkipped:
		return styleSkipped
	default:
		return stylePending
	}
}
