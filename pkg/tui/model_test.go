package tui

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pipecrew/pisyn/pkg/runner"
)

func testModel(names ...string) Model {
	infos := make([]JobInfo, len(names))
	for i, n := range names {
		infos[i] = JobInfo{Name: n}
	}
	events := make(chan runner.Event, 10)
	return NewModel(infos, events, func() {})
}

func TestModel_InitialState(t *testing.T) {
	m := testModel("a", "b", "c")

	if len(m.jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(m.jobs))
	}
	for _, j := range m.jobs {
		if j.Status != runner.StatusPending {
			t.Errorf("job %s should be pending, got %d", j.Name, j.Status)
		}
	}
	if m.done || m.failed {
		t.Error("should not be done or failed initially")
	}
}

func TestModel_JobStarted_SelectsJob(t *testing.T) {
	m := testModel("a", "b")

	ev := eventMsg(runner.Event{Type: runner.EventJobStarted, JobName: "b", Status: runner.StatusRunning})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if m.selected != 1 {
		t.Errorf("expected selected=1, got %d", m.selected)
	}
	if m.jobs[1].Status != runner.StatusRunning {
		t.Errorf("expected b running, got %d", m.jobs[1].Status)
	}
}

func TestModel_JobLog_AppendsToCorrectJob(t *testing.T) {
	m := testModel("a", "b")

	ev := eventMsg(runner.Event{Type: runner.EventJobLog, JobName: "a", Log: "hello"})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if len(m.logs["a"]) != 1 || m.logs["a"][0] != "hello" {
		t.Errorf("expected log 'hello' for a, got %v", m.logs["a"])
	}
	if len(m.logs["b"]) != 0 {
		t.Errorf("expected no logs for b, got %v", m.logs["b"])
	}
}

func TestModel_JobFinished_SetsStatusAndElapsed(t *testing.T) {
	m := testModel("a")

	ev := eventMsg(runner.Event{
		Type: runner.EventJobFinished, JobName: "a",
		Status: runner.StatusPassed, Elapsed: 5 * time.Second,
	})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if m.jobs[0].Status != runner.StatusPassed {
		t.Errorf("expected passed, got %d", m.jobs[0].Status)
	}
	if m.jobs[0].Elapsed != 5*time.Second {
		t.Errorf("expected 5s, got %s", m.jobs[0].Elapsed)
	}
}

func TestModel_FailedJob_SetsFailed(t *testing.T) {
	m := testModel("a")

	ev := eventMsg(runner.Event{Type: runner.EventJobFinished, JobName: "a", Status: runner.StatusFailed})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if !m.failed {
		t.Error("expected failed=true")
	}
}

func TestModel_AllowFailureJob_DoesNotSetFailed(t *testing.T) {
	infos := []JobInfo{{Name: "a", AllowFailure: true}}
	events := make(chan runner.Event, 10)
	m := NewModel(infos, events, func() {})

	ev := eventMsg(runner.Event{Type: runner.EventJobFinished, JobName: "a", Status: runner.StatusFailed})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if m.failed {
		t.Error("expected failed=false for allow-failure job")
	}
}

func TestModel_RunComplete_SetsDone(t *testing.T) {
	m := testModel("a")

	ev := eventMsg(runner.Event{Type: runner.EventRunComplete})
	updated, _ := m.Update(ev)
	m = updated.(Model)

	if !m.done {
		t.Error("expected done=true")
	}
	if m.endTime.IsZero() {
		t.Error("expected endTime to be set")
	}
}

func TestModel_ChannelClosed_SetsDoneAndFailed(t *testing.T) {
	m := testModel("a")

	updated, _ := m.Update(channelClosedMsg{})
	m = updated.(Model)

	if !m.done || !m.failed {
		t.Error("expected done=true and failed=true on channel close")
	}
}

func TestModel_FailedCount_ExcludesAllowFailure(t *testing.T) {
	infos := []JobInfo{
		{Name: "a", AllowFailure: false},
		{Name: "b", AllowFailure: true},
	}
	events := make(chan runner.Event, 10)
	m := NewModel(infos, events, func() {})

	// Fail both
	updated, _ := m.Update(eventMsg(runner.Event{Type: runner.EventJobFinished, JobName: "a", Status: runner.StatusFailed}))
	m = updated.(Model)
	updated, _ = m.Update(eventMsg(runner.Event{Type: runner.EventJobFinished, JobName: "b", Status: runner.StatusFailed}))
	m = updated.(Model)

	if m.FailedCount() != 1 {
		t.Errorf("expected FailedCount=1 (excluding allow-failure), got %d", m.FailedCount())
	}
}

func TestModel_Cancel_CalledOnQuit(t *testing.T) {
	infos := []JobInfo{{Name: "a"}}
	events := make(chan runner.Event, 10)
	ctx, cancel := context.WithCancel(context.Background())
	m := NewModel(infos, events, cancel)
	m.width = 80
	m.height = 24

	msg := tea.KeyPressMsg{Code: tea.KeyEscape, Text: "q"}
	// Use string-based approach: simulate "q" key
	m.Update(tea.KeyPressMsg{Text: "q"})

	_ = msg
	select {
	case <-ctx.Done():
		// good
	default:
		t.Error("expected context to be cancelled on quit")
	}
}
