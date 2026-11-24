package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestRemoteUpdateCancellationLeavesWorkRunningAcrossProjects(t *testing.T) {
	ctx := context.Background()
	oldOS, oldVersion := updateOS, buildinfo.Version
	updateOS, buildinfo.Version = "linux", "0.0.0-preview.100"
	t.Cleanup(func() { updateOS, buildinfo.Version = oldOS, oldVersion })
	a, folderA := netComputer(t, ctx, "a")
	b, folderB := netComputer(t, ctx, "b")
	for _, machine := range []struct {
		s      *Supervisor
		folder string
	}{{a, folderA}, {b, folderB}} {
		if err := machine.s.store.CreateProject("Other", machine.folder); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.server.Enable(broker.ScopeLocal, freeLocalPort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := a.server.Invite(a.server.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := b.server.Connect(ctx, code.Code, "Notebook"); err != nil {
		t.Fatal(err)
	}
	network, ok := b.server.Network()
	if !ok {
		t.Fatal("worker did not join")
	}
	running, _, err := b.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "job-game", Provider: "codex", Text: "existing work"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.store.SetAutoUpdate(true); err != nil {
		t.Fatal(err)
	}
	if queued, err := b.store.QueueAutoUpdate("0.0.0-preview.101"); err != nil || !queued {
		t.Fatalf("automatic request: %v %v", queued, err)
	}
	report := func() {
		t.Helper()
		b.server.ReportPresence(ctx)
		a.server.ReportPresence(ctx)
		a.net.round = 0
		a.networkRound(ctx)
	}
	report()
	if _, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Other", Computer: network.Self, OpID: "before", Provider: "codex", Text: "blocked by the update"}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("expected update refusal: %v", err)
	}
	b.startUpdate = func(string, RemoteUpdate) error { t.Fatal("helper started during cancellation"); return nil }

	if _, err := a.store.RequestUpdateCancellation(network.Self, "Other"); err != nil {
		t.Fatal(err)
	}
	a.networkRound(ctx)
	b.networkRound(ctx)
	a.networkRound(ctx)
	if p := a.networkProblem() + b.networkProblem(); p != "" {
		t.Fatalf("mailbox problem: %s", p)
	}
	if pending, err := b.store.HasPendingUpdate(); err != nil || pending {
		t.Fatalf("receiver still blocked: %v %v", pending, err)
	}
	if u, ok, err := b.store.LocalUpdate(); err != nil || !ok || u.State != core.UpdateCancelled {
		t.Fatalf("local update result: %+v %v %v", u, ok, err)
	}
	if queued, err := b.store.QueueAutoUpdate("0.0.0-preview.101"); err != nil || queued {
		t.Fatalf("cancelled automatic release was queued again: %v %v", queued, err)
	}
	if task, err := b.store.TaskByID(running); err != nil || task.State != "running" {
		t.Fatalf("existing work changed: %+v %v", task, err)
	}
	// The broker can still hold the earlier pending presence report.
	a.server.ReportPresence(ctx)
	a.net.round = 0
	a.networkRound(ctx)
	if _, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Other", Computer: network.Self, OpID: "after", Provider: "codex", Text: "new work"}); err != nil {
		t.Fatalf("computer still refuses new tasks: %v", err)
	}
}

func TestQueuedUpdateCompletesWhileTasksRunWhenVersionAlreadyInstalled(t *testing.T) {
	ctx := context.Background()
	oldOS, oldVersion := updateOS, buildinfo.Version
	updateOS, buildinfo.Version = "linux", "0.0.0-preview.100"
	t.Cleanup(func() { updateOS, buildinfo.Version = oldOS, oldVersion })
	for _, legacy := range []bool{false, true} {
		s, _ := netComputer(t, ctx, "local")
		program := filepath.Join(s.root, "messenger")
		if err := os.WriteFile(program, []byte("program"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := WriteProgramPath(s.root, program); err != nil {
			t.Fatal(err)
		}
		s.startUpdate = func(string, RemoteUpdate) error { t.Fatal("already installed release launched a helper"); return nil }
		task, _, err := s.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "work", Provider: "codex", Text: "work continues"})
		if err != nil {
			t.Fatal(err)
		}
		if legacy {
			if _, err := s.store.RequestNetUpdate("", "Game"); err != nil {
				t.Fatal(err)
			}
			s.findUpdate = func(context.Context) (string, error) { return buildinfo.Version, nil }
		} else {
			if err := s.store.SetAutoUpdate(true); err != nil {
				t.Fatal(err)
			}
			if queued, err := s.store.QueueAutoUpdate(buildinfo.Version); err != nil || !queued {
				t.Fatalf("queue: %v %v", queued, err)
			}
		}
		s.networkRound(ctx)
		waitAutoLook(t, s)
		s.networkRound(ctx)
		if u, ok, err := s.store.LocalUpdate(); err != nil || !ok || u.State != core.UpdateDone || u.Version != buildinfo.Version {
			t.Fatalf("existing version not reconciled: %+v %v %v", u, ok, err)
		}
		if got, err := s.store.TaskByID(task); err != nil || got.State != "running" {
			t.Fatalf("task interrupted: %+v %v", got, err)
		}
	}
}
