package core

import "testing"

func TestTaskActivity(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Fix the menu")
	if a, err := f.st.TaskActivity(id); err != nil || a.At != "" || a.Last != "" || a.Tokens != 0 {
		t.Fatalf("fresh %+v %v", a, err)
	}
	r := f.launch(t, id, f.sessionOf(t, id))
	_ = f.st.AppendTaskRunItem(r.ID, "text", "", "Looking at the menu")
	_ = f.st.AppendTaskRunItem(r.ID, "tool", "Bash", "go test ./...")
	a, err := f.st.TaskActivity(id)
	if err != nil || a.At == "" || a.Last != "Bash: go test ./..." {
		t.Fatalf("tool %+v %v", a, err)
	}
	long := ""
	for len(long) < 1000 {
		long += "слово "
	}
	_ = f.st.AppendTaskRunItem(r.ID, "text", "", long)
	if a, _ = f.st.TaskActivity(id); len([]rune(a.Last)) > maxActivityText {
		t.Fatalf("not clipped %d", len([]rune(a.Last)))
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET usage_input=1200,usage_output=300 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if a, _ = f.st.TaskActivity(id); a.Tokens != 1500 {
		t.Fatalf("tokens %+v", a)
	}
}

func TestRemoteTaskActivity(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op", "codex", "Build")
	p.carry(t, true)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, p.b.sessionOf(t, m.ID))
	_ = p.b.st.AppendTaskRunItem(r.ID, "text", "", "Building the player")
	p.carry(t, false)
	if a, err := p.a.st.TaskActivity(id); err != nil || a.At == "" || a.Last != "Building the player" {
		t.Fatalf("remote %+v %v", a, err)
	}
}
