package core

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type taskFixture struct {
	st     *Store
	path   string
	folder string
}

// realDir resolves symlinks, as the store does: on macOS the temporary
// directory lives behind /tmp or /var, which are links to /private.
func realDir(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func allowTempFolders(t *testing.T) {
	t.Helper()
	old := TemporaryFolder
	TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { TemporaryFolder = old })
}

func newTaskFixture(t *testing.T) taskFixture {
	t.Helper()
	allowTempFolders(t)
	root := t.TempDir()
	path := filepath.Join(root, "queue.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	folder := realDir(t, mkdir(t, filepath.Join(root, "game")))
	if err := st.CreateProject("Game", folder); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(root, "agent")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"claude", "codex"} {
		if err := st.SetComputerAgent(p, cli, "", "messenger-only"); err != nil {
			t.Fatal(err)
		}
	}
	return taskFixture{st: st, path: path, folder: folder}
}

var taskObs = platform.Observation{PID: 4242, PGID: 4242, StartToken: "start", BootID: "boot"}

func (f taskFixture) start(t *testing.T, op, provider, text string) string {
	t.Helper()
	id, created, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: op, Provider: provider, Text: text})
	if err != nil || !created {
		t.Fatalf("start %v %v", created, err)
	}
	return id
}

// launch takes the task's pending run through launch, process and identity.
func (f taskFixture) launch(t *testing.T, task, session string) TaskRun {
	t.Helper()
	runs, err := f.st.NextTaskRunIntents(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range runs {
		r, _ := f.st.TaskRun(id)
		if r.TaskID != task {
			continue
		}
		if _, _, err := f.st.BeginTaskRun(id, "res-"+id); err != nil {
			t.Fatal(err)
		}
		if err := f.st.RecordTaskRunProcess(id, taskObs); err != nil {
			t.Fatal(err)
		}
		if session != "" {
			if err := f.st.BindTaskSession(id, session); err != nil {
				t.Fatal(err)
			}
		}
		r, _ = f.st.TaskRun(id)
		return r
	}
	t.Fatalf("no pending run for %s", task)
	return TaskRun{}
}

func (f taskFixture) sessionOf(t *testing.T, task string) string {
	t.Helper()
	tk, err := f.st.TaskByID(task)
	if err != nil {
		t.Fatal(err)
	}
	return tk.SessionID
}

func (f taskFixture) dispose(t *testing.T, r TaskRun, kind, text string) {
	t.Helper()
	if _, err := f.st.ReadTask(r.ID, r.Receipt, false); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordDisposition(r.ID, r.Receipt, kind, text); err != nil {
		t.Fatal(err)
	}
}

var cleanExit = RunEvidence{ExitObserved: true, ExitCode: 0, GroupEmpty: true, ProviderOK: true}

func (f taskFixture) events(t *testing.T, audience string, after int64) []TaskEvent {
	t.Helper()
	events, _, err := f.st.TaskEvents("Game", audience, after, 100)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func kinds(events []TaskEvent) string {
	var out []string
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return strings.Join(out, ",")
}

func TestProjectNeedsAValidNameAndARealFolder(t *testing.T) {
	root := t.TempDir()
	st, err := Open(filepath.Join(root, "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	folder := realDir(t, mkdir(t, filepath.Join(root, "game")))
	// t.TempDir is a temporary folder, which is refused by default.
	if err := st.CreateProject("Game", folder); !errors.Is(err, ErrRejected) {
		t.Fatalf("temporary folder accepted: %v", err)
	}
	allowTempFolders(t)
	for _, name := range []string{"", " Game", "a/b", strings.Repeat("x", 65)} {
		if err := st.CreateProject(name, folder); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}
	for _, bad := range []string{"relative", filepath.Join(root, "missing")} {
		if err := st.CreateProject("Game", bad); err == nil {
			t.Errorf("folder %q accepted", bad)
		}
	}
	if err := st.CreateProject("Моя игра", folder); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject("Моя игра", folder); err != nil {
		t.Fatal("repeating the same project failed")
	}
	other := mkdir(t, filepath.Join(root, "other"))
	if err := st.CreateProject("Моя игра", other); err == nil {
		t.Fatal("duplicate project name accepted")
	}
	projects, err := st.Projects()
	if err != nil || len(projects) != 1 || projects[0].Folder != folder {
		t.Fatalf("projects %+v %v", projects, err)
	}
}

func TestStartTaskNeedsProjectFolderAndAgent(t *testing.T) {
	f := newTaskFixture(t)
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Nope", OpID: "o1", Provider: "claude", Text: "x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown project: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "o1", Provider: "cursor", Text: "x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("cursor accepted: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "o1", Provider: "claude", Text: "x", Computer: "computer-elsewhere"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("other computer accepted: %v", err)
	}
	// A project without a folder on this computer refuses tasks (D-025).
	if _, err := f.st.db.Exec(`INSERT INTO projects(name,created_at) VALUES ('Bare','x')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Bare", OpID: "o1", Provider: "claude", Text: "x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("project without folder accepted: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "", Provider: "claude", Text: "x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("missing operation id accepted: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "o1", Provider: "claude", Text: strings.Repeat("x", MaxTaskText+1)}); !errors.Is(err, ErrRejected) {
		t.Fatalf("oversized task accepted: %v", err)
	}
}

func TestStartTaskIsIdempotentPerOperation(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "claude", "Fix the menu")
	again, created, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-1", Provider: "claude", Text: "Fix the menu"})
	if err != nil || created || again != id {
		t.Fatalf("repeat: %s %v %v", again, created, err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-1", Provider: "claude", Text: "Something else"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("reused operation id accepted: %v", err)
	}
	runs, _ := f.st.NextTaskRunIntents(10)
	if len(runs) != 1 {
		t.Fatalf("runs %v", runs)
	}
	task, _ := f.st.TaskByID(id)
	if task.Folder != f.folder || task.State != "running" || len(task.SessionID) != 36 || task.Title != "Fix the menu" {
		t.Fatalf("task %+v", task)
	}
}

func TestStartTaskRecordsTheChosenModel(t *testing.T) {
	f := newTaskFixture(t)
	id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-m", Provider: "claude", Text: "Fix", Model: "opus"})
	if err != nil {
		t.Fatal(err)
	}
	task, _ := f.st.TaskByID(id)
	if task.Model != "opus" {
		t.Fatalf("model %q", task.Model)
	}
	// The same operation with another model is a different task.
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-m", Provider: "claude", Text: "Fix", Model: "sonnet"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("operation reused with another model: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-bad", Provider: "claude", Text: "Fix", Model: "--yolo"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("flag-like model accepted: %v", err)
	}
	view, err := f.st.TaskFeed(id)
	if err != nil || view.Task.Model != "opus" {
		t.Fatalf("feed model %+v %v", view.Task, err)
	}
}

func TestTaskReportIsPublishedOnlyAfterVerification(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "claude", "Do it")
	runs, _ := f.st.NextTaskRunIntents(10)
	r, _, err := f.st.BeginTaskRun(runs[0], "res")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordTaskRunProcess(r.ID, taskObs); err != nil {
		t.Fatal(err)
	}
	// Tools stay closed until the conversation is bound.
	if _, err := f.st.ReadTask(r.ID, r.Receipt, false); !errors.Is(err, ErrRejected) {
		t.Fatalf("read before identity: %v", err)
	}
	if err := f.st.BindTaskSession(r.ID, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("wrong Claude id bound: %v", err)
	}
	if err := f.st.BindTaskSession(r.ID, f.sessionOf(t, id)); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordDisposition(r.ID, r.Receipt, DispositionReport, "done"); !errors.Is(err, ErrRejected) {
		t.Fatalf("report before read_task: %v", err)
	}
	if _, err := f.st.ReadTask(r.ID, "wrong", false); !errors.Is(err, ErrRejected) {
		t.Fatalf("wrong receipt: %v", err)
	}
	brief, err := f.st.ReadTask(r.ID, r.Receipt, false)
	if err != nil || brief.Task != "Do it" || brief.Folder != f.folder || brief.Project != "Game" {
		t.Fatalf("brief %+v %v", brief, err)
	}
	if err := f.st.RecordDisposition(r.ID, r.Receipt, DispositionReport, "All done"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordDisposition(r.ID, r.Receipt, DispositionAsk, "and a question"); !errors.Is(err, ErrRejected) {
		t.Fatalf("second disposition: %v", err)
	}
	if got := kinds(f.events(t, AudienceCoordinator, 0)); got != "" {
		t.Fatalf("report visible before verification: %s", got)
	}
	final, err := f.st.FinishTaskRun(r.ID, cleanExit)
	if err != nil || final != "finished" {
		t.Fatalf("finish %s %v", final, err)
	}
	events := f.events(t, AudienceCoordinator, 0)
	if kinds(events) != "report" || events[0].Text != "All done" {
		t.Fatalf("events %+v", events)
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "done" || task.FinishedAt == "" {
		t.Fatalf("task %+v", task)
	}
	// The receipt is revoked with the run.
	if _, err := f.st.ReadTask(r.ID, r.Receipt, false); !errors.Is(err, ErrRejected) {
		t.Fatalf("read after finish: %v", err)
	}
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err == nil {
		t.Fatal("finished twice")
	}
	if _, err := f.st.CancelTask(id); !errors.Is(err, ErrRejected) {
		t.Fatalf("cancel of a done task: %v", err)
	}
}

func TestTaskRunOutcomes(t *testing.T) {
	cases := []struct {
		name, disposition string
		ev                RunEvidence
		final, task       string
	}{
		{"exit 1", DispositionReport, RunEvidence{ExitObserved: true, ExitCode: 1, GroupEmpty: true, ProviderOK: true}, "failed", "failed"},
		{"provider error", DispositionReport, RunEvidence{ExitObserved: true, GroupEmpty: true}, "failed", "failed"},
		{"no disposition", "", cleanExit, "failed", "failed"},
		{"group not empty", DispositionReport, RunEvidence{ExitObserved: true, GroupEmpty: false, ProviderOK: true}, "blocked", "blocked"},
		{"exit not observed", DispositionReport, RunEvidence{GroupEmpty: true, ProviderOK: true}, "blocked", "blocked"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newTaskFixture(t)
			id := f.start(t, "op", "claude", "x")
			r := f.launch(t, id, f.sessionOf(t, id))
			if c.disposition != "" {
				f.dispose(t, r, c.disposition, "partial work")
			}
			final, err := f.st.FinishTaskRun(r.ID, c.ev)
			if err != nil || final != c.final {
				t.Fatalf("final %s %v", final, err)
			}
			task, _ := f.st.TaskByID(id)
			if task.State != c.task {
				t.Fatalf("task %s", task.State)
			}
			events := f.events(t, AudienceCoordinator, 0)
			if kinds(events) != c.task {
				t.Fatalf("events %s", kinds(events))
			}
			if c.final == "failed" && c.disposition != "" && !strings.Contains(events[0].Text, "Unverified text from the worker:\npartial work") {
				t.Fatalf("failed event text %q", events[1].Text)
			}
			// Nothing is relaunched.
			if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 0 {
				t.Fatalf("relaunch intent %v", runs)
			}
		})
	}
}

func TestTaskQuestionAnswerResumesSameConversation(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Ask first")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionAsk, "Which level?")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "waiting_coordinator" || task.OpenQuestion == nil || task.OpenQuestion.Text != "Which level?" {
		t.Fatalf("task %+v", task)
	}
	q := task.OpenQuestion.ID
	events := f.events(t, AudienceCoordinator, 0)
	if kinds(events) != "question" || events[0].QuestionID != q {
		t.Fatalf("events %+v", events)
	}
	if err := f.st.AnswerTask(id, q, "a1", "Level 2", AudienceOwner); !errors.Is(err, ErrRejected) {
		t.Fatalf("owner answered a coordinator question: %v", err)
	}
	if err := f.st.AnswerTask(id, "q-unknown", "a1", "Level 2", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown question: %v", err)
	}
	if err := f.st.AnswerTask(id, q, "a1", "Level 2", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	// A repeated answer after a lost response opens no second run.
	if err := f.st.AnswerTask(id, q, "a1", "Level 2", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AnswerTask(id, q, "a2", "Level 3", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("second answer: %v", err)
	}
	if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 1 {
		t.Fatalf("runs %v", runs)
	}
	// The resumed run must report the same conversation.
	runs, _ := f.st.NextTaskRunIntents(10)
	r2, _, err := f.st.BeginTaskRun(runs[0], "res2")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordTaskRunProcess(r2.ID, taskObs); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BindTaskSession(r2.ID, "019a0000-0000-7000-8000-000000000002"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("other conversation accepted on resume: %v", err)
	}
	if err := f.st.BindTaskSession(r2.ID, "019a0000-0000-7000-8000-000000000001"); err != nil {
		t.Fatal(err)
	}
	brief, err := f.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || brief.Answer != "Level 2" || brief.Question != "Which level?" || brief.Run != 2 {
		t.Fatalf("brief %+v %v", brief, err)
	}
	// T-033: the resumed conversation already has the task text.
	if brief.Task != "" || brief.Note == "" {
		t.Fatalf("resumed brief repeats the task %+v", brief)
	}
	if full, err := f.st.ReadTask(r2.ID, r2.Receipt, true); err != nil || full.Task == "" || full.Note != "" || full.Answer != "Level 2" {
		t.Fatalf("include_task %+v %v", full, err)
	}
	if err := f.st.RecordDisposition(r2.ID, r2.Receipt, DispositionReport, "Done on level 2"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.FinishTaskRun(r2.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if got := kinds(f.events(t, AudienceCoordinator, 0)); got != "question,report" {
		t.Fatalf("events %s", got)
	}
	// A late answer after completion is refused.
	if err := f.st.AnswerTask(id, q, "a3", "late", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("late answer: %v", err)
	}
}

func TestOwnerQuestionGoesToTheOwnerOnly(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "x")
	r := f.launch(t, id, f.sessionOf(t, id))
	f.dispose(t, r, DispositionAskOwner, "May I delete the old saves?")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if got := kinds(f.events(t, AudienceCoordinator, 0)); got != "" {
		t.Fatalf("coordinator saw the owner question: %s", got)
	}
	owner := f.events(t, AudienceOwner, 0)
	if kinds(owner) != "question" {
		t.Fatalf("owner events %s", kinds(owner))
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "waiting_owner" {
		t.Fatalf("state %s", task.State)
	}
	if err := f.st.AnswerTask(id, owner[0].QuestionID, "a1", "yes", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("coordinator answered an owner question: %v", err)
	}
	if err := f.st.AnswerTask(id, owner[0].QuestionID, "a1", "yes", AudienceOwner); err != nil {
		t.Fatal(err)
	}
}

func TestCodexConversationIsBoundOnceAndNotShared(t *testing.T) {
	f := newTaskFixture(t)
	a := f.start(t, "op-a", "codex", "a")
	b := f.start(t, "op-b", "codex", "b")
	ra := f.launch(t, a, "019a0000-0000-7000-8000-00000000000a")
	if err := f.st.BindTaskSession(ra.ID, "019a0000-0000-7000-8000-00000000000b"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("second id in one run: %v", err)
	}
	rb := f.launch(t, b, "")
	if err := f.st.BindTaskSession(rb.ID, "019a0000-0000-7000-8000-00000000000a"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("conversation shared between tasks: %v", err)
	}
}

func TestCancelTask(t *testing.T) {
	f := newTaskFixture(t)
	// Before launch: cancelled at once, and the run never starts.
	a := f.start(t, "op-a", "claude", "a")
	if state, err := f.st.CancelTask(a); err != nil || state != "cancelled" {
		t.Fatalf("cancel before launch %s %v", state, err)
	}
	if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("cancelled run still pending %v", runs)
	}
	// Running: the supervisor stops it; the report is not published.
	b := f.start(t, "op-b", "claude", "b")
	r := f.launch(t, b, f.sessionOf(t, b))
	f.dispose(t, r, DispositionReport, "finished anyway")
	if state, err := f.st.CancelTask(b); err != nil || state != "cancelling" {
		t.Fatalf("cancel running %s %v", state, err)
	}
	if runs, _ := f.st.CancelRequestedRuns(); len(runs) != 1 || runs[0].ID != r.ID {
		t.Fatalf("cancel requested %+v", runs)
	}
	ev := cleanExit
	ev.Cancelled = true
	if final, err := f.st.FinishTaskRun(r.ID, ev); err != nil || final != "cancelled" {
		t.Fatalf("finish %s %v", final, err)
	}
	for _, e := range f.events(t, AudienceCoordinator, 0) {
		if e.Kind == "report" {
			t.Fatal("report of a cancelled run published")
		}
	}
	if s, err := f.st.CancelTask(b); err != nil || s != "cancelled" {
		t.Fatalf("repeat cancel %s %v", s, err)
	}
}

func TestRestartInterruptsRunsButKeepsIntents(t *testing.T) {
	f := newTaskFixture(t)
	a := f.start(t, "op-a", "claude", "a")
	f.launch(t, a, f.sessionOf(t, a))
	b := f.start(t, "op-b", "claude", "b")
	f.st.Close()
	st, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RevokeInherited(); err != nil {
		t.Fatal(err)
	}
	ta, _ := st.TaskByID(a)
	tb, _ := st.TaskByID(b)
	if ta.State != "interrupted" || tb.State != "running" {
		t.Fatalf("states %s %s", ta.State, tb.State)
	}
	runs, _ := st.NextTaskRunIntents(10)
	if len(runs) != 1 {
		t.Fatalf("intents %v", runs)
	}
	if r, _ := st.TaskRun(runs[0]); r.TaskID != b {
		t.Fatal("interrupted task relaunched")
	}
}

func TestPausedQueueLaunchesNoTaskRun(t *testing.T) {
	f := newTaskFixture(t)
	f.start(t, "op", "claude", "x")
	if err := f.st.Pause(); err != nil {
		t.Fatal(err)
	}
	runs, _ := f.st.NextTaskRunIntents(10)
	if _, _, err := f.st.BeginTaskRun(runs[0], "res"); !errors.Is(err, ErrPaused) {
		t.Fatalf("launch while paused: %v", err)
	}
}

func TestTaskEventsReplayByCursorAndProject(t *testing.T) {
	f := newTaskFixture(t)
	other := mkdir(t, filepath.Join(filepath.Dir(f.folder), "other"))
	if err := f.st.CreateProject("Other", other); err != nil {
		t.Fatal(err)
	}
	f.start(t, "op-1", "claude", "one")
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Other", OpID: "op-1", Provider: "claude", Text: "elsewhere"}); err != nil {
		t.Fatal(err)
	}
	f.start(t, "op-2", "claude", "two")
	first, cursor, err := f.st.TaskEvents("Game", AudienceLog, 0, 1)
	if err != nil || len(first) != 1 || first[0].Text != "one" {
		t.Fatalf("first %+v %v", first, err)
	}
	// The same cursor returns the same events: nothing is lost on a retry.
	again, _, _ := f.st.TaskEvents("Game", AudienceLog, 0, 1)
	if again[0].Seq != first[0].Seq {
		t.Fatal("replay differs")
	}
	rest, _, _ := f.st.TaskEvents("Game", AudienceLog, cursor, 10)
	if len(rest) != 1 || rest[0].Text != "two" {
		t.Fatalf("rest %+v", rest)
	}
	latest, err := f.st.LatestTaskEvent("Game")
	if err != nil || latest != rest[0].Seq {
		t.Fatalf("latest %d %v", latest, err)
	}
}

func TestTaskFailureReason(t *testing.T) {
	failedWith := func(t *testing.T, ev RunEvidence) (taskFixture, Task, []TaskEvent) {
		f := newTaskFixture(t)
		id := f.start(t, "op", "claude", "x")
		r := f.launch(t, id, f.sessionOf(t, id))
		if final, err := f.st.FinishTaskRun(r.ID, ev); err != nil || final != "failed" {
			t.Fatalf("final %s %v", final, err)
		}
		task, _ := f.st.TaskByID(id)
		return f, task, f.events(t, AudienceCoordinator, 0)
	}
	failed := RunEvidence{ExitObserved: true, ExitCode: 1, GroupEmpty: true, Detail: "Claude result success"}

	ev := failed
	ev.Reason, ev.ResetAt = "usage_limit", "2026-11-07T18:00:00Z"
	f, task, events := failedWith(t, ev)
	if task.FailureReason != "usage_limit" || task.FailureReset != "2026-11-07T18:00:00Z" {
		t.Fatalf("task %+v", task)
	}
	if !strings.Contains(events[0].Text, "Reason: usage limit; it resets at 2026-11-07T18:00:00Z. Tell the owner the reset time; do not start this task again before it.") {
		t.Fatalf("event %q", events[0].Text)
	}
	view, err := f.st.WindowView("Game", "2026-11-07")
	if err != nil || len(view.Tasks) != 1 || view.Tasks[0].FailureReason != "usage_limit" || view.Tasks[0].FailureReset != "2026-11-07T18:00:00Z" {
		t.Fatalf("window %+v %v", view.Tasks, err)
	}
	// Any later state clears the explanation.
	if err := f.st.tx(func(tx *sql.Tx) error { return setTaskState(tx, task.ID, "blocked", "x") }); err != nil {
		t.Fatal(err)
	}
	if task, _ = f.st.TaskByID(task.ID); task.FailureReason != "" || task.FailureReset != "" {
		t.Fatalf("not cleared %+v", task)
	}
	// Nothing is relaunched because of a limit.
	if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("relaunch intent %v", runs)
	}

	// A reset time is kept only for a usage limit, and only in RFC 3339.
	ev = failed
	ev.Reason, ev.ResetAt = "rate_limit", "2026-11-07T18:00:00Z"
	if _, task, events = failedWith(t, ev); task.FailureReason != "rate_limit" || task.FailureReset != "" || !strings.Contains(events[0].Text, "Reason: rate limit.") {
		t.Fatalf("rate limit %+v %q", task, events[0].Text)
	}
	ev = failed
	ev.Reason, ev.ResetAt = "usage_limit", "tomorrow"
	if _, task, events = failedWith(t, ev); task.FailureReset != "" || !strings.Contains(events[0].Text, "no reset time was reported") {
		t.Fatalf("bad reset %+v %q", task, events[0].Text)
	}
	// Unknown or unclassified reasons add nothing.
	for _, reason := range []string{"", "other", "something"} {
		ev = failed
		ev.Reason = reason
		_, task, events = failedWith(t, ev)
		if (reason == "other") != (task.FailureReason == "other") || strings.Contains(events[0].Text, "Reason:") {
			t.Fatalf("%q: %+v %q", reason, task, events[0].Text)
		}
	}
}
