package core

import (
	"errors"
	"strings"
	"testing"
)

// T-087: a long final message is kept whole; the report tells the
// coordinator to read it, and task_output returns it in parts.
func TestFinalMessageIsKeptWhole(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Audit the scenes")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Audit done; details in the final message")
	ev := cleanExit
	ev.FinalText = "Full audit:\n" + strings.Repeat("Scene 12 has no description. ", 1000)
	if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	events, _, _ := f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	last := events[len(events)-1]
	if last.Kind != "report" || !strings.Contains(last.Text, outputMarker) || !strings.Contains(last.Text, "task_output") {
		t.Fatalf("report %+v", last)
	}
	total := []rune(strings.TrimSpace(ev.FinalText))
	var got strings.Builder
	offset := 0
	for {
		part, err := f.st.TaskOutputOf(id, 0, offset)
		if err != nil {
			t.Fatal(err)
		}
		if part.Run != 1 || part.Total != len(total) {
			t.Fatalf("part %+v", part)
		}
		got.WriteString(part.Text)
		if part.Done {
			break
		}
		offset = part.Next
	}
	if got.String() != string(total) {
		t.Fatal("output differs")
	}
	if _, err := f.st.TaskOutputOf(id, 7, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("missing run: %v", err)
	}
	// A short final message that says what the report says adds no note.
	f2 := newTaskFixture(t)
	id2 := f2.start(t, "op", "codex", "Small")
	r2 := f2.launch(t, id2, "019a0000-0000-7000-8000-000000000002")
	f2.dispose(t, r2, DispositionReport, "Done")
	ev2 := cleanExit
	ev2.FinalText = "Done."
	if _, err := f2.st.FinishTaskRun(r2.ID, ev2); err != nil {
		t.Fatal(err)
	}
	events, _, _ = f2.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	if last := events[len(events)-1]; strings.Contains(last.Text, outputMarker) {
		t.Fatalf("note for a short message %+v", last)
	}
}

// T-087: the computer that ran a task sends the output back before the
// report, so the computer that started it can read it.
func TestFinalMessageComesBackFromAnotherComputer(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op1", "codex", "Audit")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	p.b.dispose(t, r, DispositionReport, "Audit done")
	ev := cleanExit
	ev.FinalText = strings.Repeat("Ячейка без описания. ", 800)
	if _, err := p.b.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	part, err := p.a.st.TaskOutputOf(id, 0, 0)
	if err != nil || part.Total != len([]rune(strings.TrimSpace(ev.FinalText))) {
		t.Fatalf("output on a %+v %v", part, err)
	}
	events, _, _ := p.a.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	if last := events[len(events)-1]; last.Kind != "report" || strings.Count(last.Text, outputMarker) != 1 {
		t.Fatalf("report on a %+v", last)
	}
}
