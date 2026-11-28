package core

import (
	"errors"
	"testing"
)

func TestContinueFromWindowUsesNewSettingsInSameSession(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetAgentFullAccess("codex", true); err != nil {
		t.Fatal(err)
	}
	const initial = "gpt-6.2-sol[effort=medium]"
	const changed = "gpt-6.2-astra[effort=high]"
	if err := f.st.SetAgentModels("codex", []string{initial, changed}); err != nil {
		t.Fatal(err)
	}
	id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "chat", Text: "Initial", Model: initial, Provider: "codex", ByOwner: true})
	if err != nil {
		t.Fatal(err)
	}
	const session = "019a0000-0000-7000-8000-0000000000b1"
	first := f.launch(t, id, session)
	next := TaskContinuation{Model: changed, Access: "standard"}
	files := []TaskAttachment{{Path: "photo.png", Data: []byte{0, 1, 255}}}
	if err := f.st.ContinueFromWindow(id, "next", "Continue", next, files); !errors.Is(err, ErrRejected) {
		t.Fatalf("working task accepted changed settings: %v", err)
	}
	before, _ := f.st.TaskByID(id)
	if before.Model != initial || !before.FullAccess {
		t.Fatalf("working task settings changed: %+v", before)
	}
	stopRun := func(run TaskRun) {
		t.Helper()
		if state, err := f.st.CancelTask(id); err != nil || state != "cancelling" {
			t.Fatalf("stop: %s %v", state, err)
		}
		exit := cleanExit
		exit.Cancelled = true
		if state, err := f.st.FinishTaskRun(run.ID, exit); err != nil || state != "cancelled" {
			t.Fatalf("finish: %s %v", state, err)
		}
	}
	stopRun(first)
	for i := 0; i < 2; i++ {
		if err := f.st.ContinueFromWindow(id, "next", "Continue", next, files); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.ContinueFromWindow(id, "next", "Continue", TaskContinuation{Access: "full"}, files); !errors.Is(err, ErrRejected) {
		t.Fatalf("operation replay accepted different settings: %v", err)
	}
	second := f.launch(t, id, session)
	after, _ := f.st.TaskByID(id)
	if second.Kind != "continue" || after.SessionID != session || after.Model != changed || after.FullAccess {
		t.Fatalf("unexpected resumed task: %+v %+v", second, after)
	}
	stopRun(second)
	if err := f.st.ContinueFromWindow(id, "restore", "Continue again", TaskContinuation{Access: "default"}, nil); err != nil {
		t.Fatal(err)
	}
	f.launch(t, id, session)
	after, _ = f.st.TaskByID(id)
	if !after.FullAccess || after.Model != changed || after.RequestedAccess != "" {
		t.Fatalf("executor defaults were not restored: %+v", after)
	}
}

func TestRemoteContinuationSettingsRespectExecutorPolicy(t *testing.T) {
	p := newNetPair(t)
	const model = "gpt-6.2-astra[effort=high]"
	if err := p.b.st.SetAgentModels("codex", []string{model}); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.SetAgentFullAccess("codex", true); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.SetNetFullAccess(false); err != nil {
		t.Fatal(err)
	}
	setPeer := func(version string) {
		t.Helper()
		if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Worker", Age: 1, Version: version, Agents: []NetAgent{{Agent: "codex"}}}}); err != nil {
			t.Fatal(err)
		}
	}
	setPeer("1.0.17-preview.149")
	id := p.startRemote(t, "remote", "codex", "Initial")
	p.carry(t, true)
	mirror := p.mirror(t, id)
	const session = "019a0000-0000-7000-8000-0000000000b1"
	first := p.b.launch(t, mirror.ID, session)
	p.b.dispose(t, first, DispositionReport, "Done")
	if _, err := p.b.st.FinishTaskRun(first.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	options := TaskContinuation{Model: model, Access: "full"}
	if err := p.a.st.ContinueFromWindow(id, "next", "Continue", options, nil); !errors.Is(err, ErrRejected) {
		t.Fatalf("old executor accepted changed settings: %v", err)
	}
	setPeer("1.0.17-preview.150")
	if err := p.a.st.ContinueFromWindow(id, "next", "Continue", options, nil); err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.ContinueFromWindow(id, "next", "Continue", options, nil); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	p.b.launch(t, mirror.ID, session)
	task, _ := p.b.st.TaskByID(mirror.ID)
	if task.Model != model || task.SessionID != session || task.FullAccess {
		t.Fatalf("remote settings or executor policy not applied: %+v", task)
	}
}
