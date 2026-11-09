package core

import (
	"errors"
	"strings"
	"testing"
)

// T-085: the owner writes to a working worker; the message waits and the
// same conversation continues with it as soon as the current run ends.
func TestOwnerNoteWaitsForTheRun(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Build the grid")
	for _, bad := range []struct{ op, text string }{{"", "x"}, {"n0", " "}} {
		if err := f.st.AddOwnerNote(id, bad.op, bad.text); !errors.Is(err, ErrRejected) {
			t.Fatalf("accepted %+v: %v", bad, err)
		}
	}
	r := f.launch(t, id, f.sessionOf(t, id))
	if err := f.st.AddOwnerNote(id, "n1", "Use blue"); err != nil {
		t.Fatal(err)
	}
	// Repeating after a lost response adds nothing; a second note is kept.
	if err := f.st.AddOwnerNote(id, "n1", "Use blue"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AddOwnerNote(id, "n1", "Other text"); !errors.Is(err, ErrRejected) {
		t.Fatalf("op reused: %v", err)
	}
	if err := f.st.AddOwnerNote(id, "n2", "And bigger cells"); err != nil {
		t.Fatal(err)
	}
	// The coordinator hears that the owner wrote.
	events, _, _ := f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	noted := 0
	for _, e := range events {
		if e.Kind == "owner_note" {
			noted++
		}
	}
	if noted != 2 {
		t.Fatalf("coordinator events %+v", events)
	}
	f.dispose(t, r, DispositionReport, "Grid built")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	// The report is held: the task runs on with the notes.
	task, _ := f.st.TaskByID(id)
	if task.State != "running" {
		t.Fatalf("task %+v", task)
	}
	events, _, _ = f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	for _, e := range events {
		if e.Kind == "report" {
			t.Fatalf("held report published: %+v", e)
		}
	}
	r2 := f.launch(t, id, f.sessionOf(t, id))
	if r2.Kind != "note" || r2.Ordinal != 2 {
		t.Fatalf("run %+v", r2)
	}
	brief, err := f.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || brief.Task != "" || brief.Note == "" || !strings.Contains(brief.OwnerNotes, "Use blue") || !strings.Contains(brief.OwnerNotes, "And bigger cells") || !strings.Contains(brief.HeldReport, "Grid built") {
		t.Fatalf("brief %+v %v", brief, err)
	}
	// The feed shows both notes as delivered and the first report as held.
	feed, err := f.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	notes, held := 0, false
	for _, it := range feed.Items {
		if it.Kind == "note" && it.Author == AudienceOwner && it.State == "delivered" {
			notes++
		}
		if it.Kind == "report" && it.Audience == "held" {
			held = true
		}
		if it.Kind == "answer" {
			t.Fatalf("note shown as an answer: %+v", it)
		}
	}
	if notes != 2 || !held {
		t.Fatalf("feed %+v", feed.Items)
	}
	f.dispose(t, r2, DispositionReport, "Grid built, blue, bigger")
	if _, err := f.st.FinishTaskRun(r2.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "done" {
		t.Fatalf("task %+v", task)
	}
	if err := f.st.AddOwnerNote(id, "n3", "Late"); !errors.Is(err, ErrRejected) {
		t.Fatalf("note to a finished task: %v", err)
	}
}

// A note that waits while the worker asks a question goes with the answer;
// a note to a task that then fails is marked undelivered, never resent.
func TestOwnerNoteWithQuestionsAndFailures(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Build the grid")
	r := f.launch(t, id, f.sessionOf(t, id))
	if err := f.st.AddOwnerNote(id, "n1", "Use blue"); err != nil {
		t.Fatal(err)
	}
	f.dispose(t, r, DispositionAsk, "Which scene?")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "waiting_coordinator" || task.OpenQuestion == nil {
		t.Fatalf("task %+v", task)
	}
	// T-125: the owner comes first. While the worker waits for the
	// coordinator, the owner's message answers its question at once.
	if err := f.st.AddOwnerNote(id, "n2", "Main scene"); err != nil {
		t.Fatal(err)
	}
	// A repeat after a lost response changes nothing.
	if err := f.st.AddOwnerNote(id, "n2", "Main scene"); err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "running" {
		t.Fatalf("task after the owner's answer %+v", task)
	}
	// The coordinator learns of it, and its own late answer is refused.
	events, _, _ := f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	told := false
	for _, e := range events {
		told = told || (e.Kind == "owner_answered" && e.Text == "Main scene")
	}
	if !told {
		t.Fatalf("coordinator events %+v", events)
	}
	if err := f.st.AnswerTask(id, task.OpenQuestion.ID, "a1", "Other scene", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("late coordinator answer: %v", err)
	}
	r2 := f.launch(t, id, f.sessionOf(t, id))
	brief, err := f.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || brief.Answer != "Main scene" || !strings.Contains(brief.OwnerNotes, "Use blue") {
		t.Fatalf("brief %+v %v", brief, err)
	}
	if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("a second run was recorded: %v", runs)
	}
	if err := f.st.AddOwnerNote(id, "n3", "Never mind"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordDisposition(r2.ID, r2.Receipt, DispositionReport, "Done"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.FinishTaskRun(r2.ID, RunEvidence{ExitObserved: true, ExitCode: 1, GroupEmpty: true}); err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "failed" {
		t.Fatalf("task %+v", task)
	}
	if runs, _ := f.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("a failed task was started again: %v", runs)
	}
	feed, _ := f.st.TaskFeed(id)
	undelivered := 0
	for _, it := range feed.Items {
		if it.Kind == "note" && it.State == "undelivered" {
			undelivered++
		}
	}
	if undelivered != 1 {
		t.Fatalf("feed %+v", feed.Items)
	}
}

// The owner answers a question to the owner instead of writing a note, and
// a task that is not run here takes no notes.
func TestOwnerNoteRefusals(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Build the grid")
	r := f.launch(t, id, f.sessionOf(t, id))
	f.dispose(t, r, DispositionAskOwner, "Which color?")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AddOwnerNote(id, "n1", "Blue"); !errors.Is(err, ErrRejected) {
		t.Fatalf("note instead of an answer: %v", err)
	}
	if err := f.st.AddOwnerNote("task-unknown", "n2", "x"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown task: %v", err)
	}
	if err := f.st.AddOwnerNote(id, "n3", strings.Repeat("x", MaxTaskText+1)); !errors.Is(err, ErrRejected) {
		t.Fatalf("long note: %v", err)
	}
}
