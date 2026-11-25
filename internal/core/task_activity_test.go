package core

import (
	"testing"
	"time"
)

func TestTaskActivity(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Fix the menu")
	if a, err := f.st.TaskActivity(id); err != nil || a.At != "" || a.Last != "" || a.Tokens != 0 {
		t.Fatalf("fresh %+v %v", a, err)
	}
	r := f.launch(t, id, f.sessionOf(t, id))
	_ = f.st.AppendTaskRunItem(r.ID, "text", "", "Looking at the menu", "")
	_ = f.st.AppendTaskRunItem(r.ID, "tool", "Bash", "go test ./...", "")
	a, err := f.st.TaskActivity(id)
	if err != nil || a.At == "" || a.Last != "Bash: go test ./..." {
		t.Fatalf("tool %+v %v", a, err)
	}
	long := ""
	for len(long) < 1000 {
		long += "слово "
	}
	_ = f.st.AppendTaskRunItem(r.ID, "text", "", long, "")
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
	_ = p.b.st.AppendTaskRunItem(r.ID, "text", "", "Building the player", "")
	p.carry(t, false)
	if a, err := p.a.st.TaskActivity(id); err != nil || a.At == "" || a.Last != "Building the player" {
		t.Fatalf("remote %+v %v", a, err)
	}
}

func TestCodexRunningCommandAndQuietNoticeStayOnTheTaskRoute(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op", "codex", "Run the checks")
	p.carry(t, true)
	mirror := p.mirror(t, id)
	run := p.b.launch(t, mirror.ID, p.b.sessionOf(t, mirror.ID))

	appendItem := func(kind, key, command string) {
		t.Helper()
		if err := p.b.st.AppendTaskRunItem(run.ID, kind, run.ID+"/"+key, command, ""); err != nil {
			t.Fatal(err)
		}
	}
	assertCommand := func(want string) {
		t.Helper()
		p.carry(t, false)
		local, err := p.b.st.TaskRunningCommand(mirror.ID, false)
		if err != nil || local.Command != want || (local.At != "") != (want != "") {
			t.Fatalf("local command: %+v %v", local, err)
		}
		remote, err := p.a.st.TaskRunningCommand(id, true)
		if err != nil || remote.Command != want || (remote.At != "") != (want != "") {
			t.Fatalf("remote command: %+v %v", remote, err)
		}
	}
	appendItem("command_start", "first", "go test ./...")
	appendItem("command_start", "second", "go vet ./...")
	assertCommand("go vet ./...")
	appendItem("command_end", "second", "")
	assertCommand("go test ./...")
	appendItem("command_end", "first", "")
	assertCommand("")
	if err := p.b.st.NotifyCodexQuiet(run.ID, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	events, _, err := p.a.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	if err != nil || len(events) != 1 || events[0].Kind != TaskEventWorkerQuiet || events[0].TaskID != id {
		t.Fatalf("coordinator events: %+v %v", events, err)
	}
	if events, _, err := p.b.st.TaskEvents("Game", AudienceCoordinator, 0, 50); err != nil || len(events) != 0 {
		t.Fatalf("the executor's coordinator saw the notice: %+v %v", events, err)
	}
	if task, err := p.a.st.TaskByID(id); err != nil || task.State != "running" {
		t.Fatalf("remote task: %+v %v", task, err)
	}
}
