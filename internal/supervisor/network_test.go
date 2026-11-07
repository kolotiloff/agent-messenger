package supervisor

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// netComputer is one computer of a test network: its store and its
// connection to the broker, without the rest of the client.
func netComputer(t *testing.T, ctx context.Context, name string) (*Supervisor, string) {
	t.Helper()
	old := core.TemporaryFolder
	core.TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { core.TemporaryFolder = old })
	root := t.TempDir()
	st := mustStore(t, root)
	t.Cleanup(func() { st.Close() })
	folder, err := filepath.EvalSymlinks(mkdir(t, filepath.Join(root, "game")))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject("Game", folder); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(root, "codex")
	if runtime.GOOS == "windows" {
		cli += ".exe"
	}
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := st.SetComputerAgent("codex", cli, "", "messenger-only"); err != nil {
		t.Fatal(err)
	}
	svc := broker.NewService(ctx, mkdir(t, filepath.Join(root, "server-"+name)))
	t.Cleanup(svc.Close)
	sup := &Supervisor{store: st, server: svc, root: root, ctx: ctx}
	svc.SetPresence(sup.presence)
	return sup, folder
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestTaskTravelsBetweenComputers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := netComputer(t, ctx, "a")
	b, folderB := netComputer(t, ctx, "b")
	if err := a.server.Enable(broker.ScopeLocal, freeLocalPort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := a.server.Invite(a.server.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := b.server.Connect(ctx, code.Code, "Ноутбук"); err != nil {
		t.Fatal(err)
	}
	b.server.ReportPresence(ctx)
	a.server.ReportPresence(ctx)
	b.server.ReportPresence(ctx)
	a.networkRound(ctx)
	b.networkRound(ctx)
	peers, err := a.store.NetPeers()
	if err != nil || len(peers) != 1 || peers[0].Name != "Ноутбук" {
		t.Fatalf("a's peers %+v %v", peers, err)
	}
	bID := peers[0].ID
	if err := b.store.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}

	id, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op1", Computer: bID, Provider: "codex", Text: "Build level 1"})
	if err != nil {
		t.Fatal(err)
	}
	a.networkRound(ctx) // sends the start
	b.networkRound(ctx) // accepts it, sends accepted
	a.networkRound(ctx) // applies accepted
	if p := a.networkProblem() + b.networkProblem(); p != "" {
		t.Fatalf("network problem %q", p)
	}
	task, _ := a.store.TaskByID(id)
	if task.Folder != folderB || task.State != "running" {
		t.Fatalf("a's task %+v", task)
	}
	runs, _ := b.store.NextTaskRunIntents(10)
	if len(runs) != 1 {
		t.Fatalf("b's runs %v", runs)
	}
	// The run on b, as the task runner would take it.
	if _, _, err := b.store.BeginTaskRun(runs[0], "res"); err != nil {
		t.Fatal(err)
	}
	if err := b.store.RecordTaskRunProcess(runs[0], platform.Observation{PID: 7, PGID: 7, StartToken: "s", BootID: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := b.store.BindTaskSession(runs[0], "019a0000-0000-7000-8000-0000000000e1"); err != nil {
		t.Fatal(err)
	}
	r, _ := b.store.TaskRun(runs[0])
	if _, err := b.store.ReadTask(r.ID, r.Receipt, false); err != nil {
		t.Fatal(err)
	}
	if err := b.store.RecordDisposition(r.ID, r.Receipt, core.DispositionReport, "Level 1 is built"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.store.FinishTaskRun(r.ID, core.RunEvidence{ExitObserved: true, GroupEmpty: true, ProviderOK: true}); err != nil {
		t.Fatal(err)
	}
	b.networkRound(ctx)
	a.networkRound(ctx)
	task, _ = a.store.TaskByID(id)
	if task.State != "done" {
		t.Fatalf("report did not arrive: %+v", task)
	}
	events, _, _ := a.store.TaskEvents("Game", core.AudienceCoordinator, 0, 10)
	if len(events) != 1 || events[0].Kind != "report" || events[0].Text != "Level 1 is built" {
		t.Fatalf("a's events %+v", events)
	}
	// Further rounds change nothing: every message was applied once.
	a.networkRound(ctx)
	b.networkRound(ctx)
	if events, _, _ = a.store.TaskEvents("Game", core.AudienceCoordinator, 0, 10); len(events) != 1 {
		t.Fatalf("events repeated %+v", events)
	}

	// Revoking b: a's next start cannot be delivered, so it fails here.
	if err := a.server.Revoke(bID); err != nil {
		t.Fatal(err)
	}
	id2, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op2", Computer: bID, Provider: "codex", Text: "After revoke"})
	if err != nil {
		t.Fatal(err)
	}
	a.networkRound(ctx)
	if task, _ := a.store.TaskByID(id2); task.State != "failed" {
		t.Fatalf("start to a revoked computer %+v", task)
	}
	b.networkRound(ctx)
	if b.networkProblem() == "" {
		t.Fatal("revoked computer shows no problem")
	}
}
