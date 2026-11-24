package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestUpdateQueueDeadline(t *testing.T) {
	s, _ := netComputer(t, context.Background(), "local")
	task, _, err := s.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "work", Provider: "codex", Text: "ongoing work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.RequestNetUpdate("", "Game"); err != nil {
		t.Fatal(err)
	}
	p, ok, err := s.store.PendingUpdate()
	if err != nil || !ok {
		t.Fatalf("pending update: %+v, %v", p, err)
	}
	start, err := time.Parse(time.RFC3339Nano, p.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	check := func(elapsed time.Duration, pending bool) {
		t.Helper()
		if msg := s.startQueuedUpdate(start.Add(elapsed)); msg != "" {
			t.Fatal(msg)
		}
		got, err := s.store.HasPendingUpdate()
		if err != nil || got != pending {
			t.Fatalf("pending after %s: %v, %v", elapsed, got, err)
		}
	}
	check(5*time.Minute-time.Nanosecond, true)
	marker := filepath.Join(s.root, remoteUpdateMarker)
	if err := os.WriteFile(marker, []byte("installer owns request"), 0600); err != nil {
		t.Fatal(err)
	}
	check(5*time.Minute, true)
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	check(5*time.Minute, false)
	check(10*time.Minute, false)
	if u, ok, err := s.store.LocalUpdate(); err != nil || !ok || u.State != core.UpdateRefused || !strings.Contains(u.Text, "5 минут") {
		t.Fatalf("timeout result: %+v %v %v", u, ok, err)
	}
	if got, err := s.store.TaskByID(task); err != nil || got.State != "running" {
		t.Fatalf("existing task: %+v, %v", got, err)
	}
}

func TestAllWindowsWatchAndCancelTheSameRemoteUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	oldVersion := buildinfo.Version
	buildinfo.Version = "1.0.0-preview.103"
	defer func() { buildinfo.Version = oldVersion }()
	host, _ := netComputer(t, ctx, "host")
	target, _ := netComputer(t, ctx, "target")
	observer, _ := netComputer(t, ctx, "observer")
	if err := host.server.Enable(broker.ScopeLocal, freeLocalPort(t)); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*Supervisor{target, observer} {
		code, err := host.server.Invite(host.server.Status().Host.Addresses[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := m.server.Connect(ctx, code.Code, filepath.Base(m.root)); err != nil {
			t.Fatal(err)
		}
		m.server.ReportPresence(ctx)
	}
	host.server.ReportPresence(ctx)
	observer.server.ReportPresence(ctx)
	host.networkRound(ctx)
	observer.networkRound(ctx)
	network, _ := target.server.Network()
	type observed struct {
		s   broker.UpdateSnapshot
		err error
	}
	results := make(chan observed, 3)
	for _, m := range []*Supervisor{host, target, observer} {
		snap, err := m.server.WatchUpdates(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		go func(m *Supervisor) { s, err := m.server.WatchUpdates(ctx, snap.Cursor); results <- observed{s, err} }(m)
	}
	start := time.Now()
	id, err := host.store.RequestNetUpdate(network.Self, "Game")
	if err != nil {
		t.Fatal(err)
	}
	host.networkRound(ctx)
	for i := 0; i < 3; i++ {
		change := <-results
		m := change.s.Marks[network.Self]
		if change.err != nil || m.ID != id || m.State != "waiting" {
			t.Fatalf("update signal %+v: %v", m, change.err)
		}
	}
	t.Logf("three windows received the same request after %s", time.Since(start))
	// The observer cancels while the receiver has not yet read its inbox.
	if _, err := observer.Server(ctx, "cancel-update", []byte(fmt.Sprintf(`{"device":%q,"broker":%q,"request_id":%q}`, network.Self, network.BrokerID, id))); err != nil {
		t.Fatal(err)
	}
	observer.networkRound(ctx)
	target.networkRound(ctx)
	host.networkRound(ctx)
	for _, m := range []*Supervisor{host, target, observer} {
		snap, err := m.server.WatchUpdates(ctx, "")
		if err != nil || snap.Marks[network.Self].State != "cancelled" {
			t.Fatalf("final snapshot %+v: %v", snap, err)
		}
	}
	if u, err := host.store.NetUpdates(); err != nil || len(u) != 1 || u[0].State != core.UpdateCancelled {
		t.Fatalf("requester result: %+v %v", u, err)
	}
}
