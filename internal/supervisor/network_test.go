package supervisor

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// Shared task lists (T-047): after one round each computer shows the other's
// tasks of the project; a task that already runs here is not shown twice.
func TestSharedTaskListsBetweenComputers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := netComputer(t, ctx, "a")
	b, _ := netComputer(t, ctx, "b")
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
	peers := func() []core.NetPeer {
		a.networkRound(ctx)
		p, _ := a.store.NetPeers()
		return p
	}()
	if len(peers) != 1 {
		t.Fatalf("a's peers %+v", peers)
	}
	if err := b.store.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op1", Provider: "codex", Text: "Menu"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op2", Computer: peers[0].ID, Provider: "codex", Text: "Build on the laptop"}); err != nil {
		t.Fatal(err)
	}
	a.round(t, 0)
	a.networkRound(ctx) // sends the start, shares its list
	b.networkRound(ctx) // accepts the task, shares its (empty) list, gets a's
	if p := a.networkProblem() + b.networkProblem(); p != "" {
		t.Fatalf("network problem %q", p)
	}
	v, err := b.store.WindowView("Game", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Shared) != 1 || v.Shared[0].Peer != broker.HostPeer || len(v.Shared[0].Tasks) != 1 || v.Shared[0].Tasks[0].Title != "Menu" {
		t.Fatalf("b shows %+v", v.Shared)
	}
	if len(v.Tasks) != 1 || v.Tasks[0].Title != "Build on the laptop" {
		t.Fatalf("b's own tasks %+v", v.Tasks)
	}
}

// round sets the supervisor's round counter, so the next round shares lists.
func (s *Supervisor) round(t *testing.T, n int) {
	t.Helper()
	s.net.mu.Lock()
	s.net.round = n
	s.net.mu.Unlock()
}

func TestLogOfAnotherComputer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := netComputer(t, ctx, "a")
	b, _ := netComputer(t, ctx, "b")
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
	a.networkRound(ctx)
	peers, _ := a.store.NetPeers()
	if len(peers) != 1 {
		t.Fatalf("a's peers %+v", peers)
	}
	if err := os.WriteFile(filepath.Join(b.root, "background-1.log"), []byte("start\nsession 0f8fad5b-d9cb-469f-a165-70867728950e\nlast\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ask := func() core.NetLog {
		id, err := a.store.RequestNetLog(peers[0].ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		a.networkRound(ctx) // sends the request
		b.networkRound(ctx) // reads it and answers
		a.networkRound(ctx) // reads the answer
		if p := a.networkProblem() + b.networkProblem(); p != "" {
			t.Fatalf("network problem %q", p)
		}
		l, err := a.store.NetLog(id)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	// Off by default: the other computer refuses.
	if l := ask(); l.State != "refused" || !strings.Contains(l.Text, "Отдавать журнал по сети") {
		t.Fatalf("default %+v", l)
	}
	if err := b.store.SetNetLogShare(true); err != nil {
		t.Fatal(err)
	}
	l := ask()
	if l.State != "done" || !strings.Contains(l.Text, "start\n") || !strings.HasSuffix(l.Text, "last\n") || strings.Contains(l.Text, "0f8fad5b") {
		t.Fatalf("shared %+v", l)
	}
}

func TestReadinessReachesTheCoordinator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := netComputer(t, ctx, "a")
	b, _ := netComputer(t, ctx, "b")
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
	if err := b.store.SetNetAccept(false); err != nil {
		t.Fatal(err)
	}
	b.server.ReportPresence(ctx)
	a.round(t, 0)
	a.networkRound(ctx)
	list, err := a.store.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	computers := list["computers"].([]core.ComputerSummary)
	if len(computers) != 2 || computers[1].AcceptsTasks == nil || *computers[1].AcceptsTasks || computers[1].HasProject == nil || !*computers[1].HasProject {
		t.Fatalf("readiness %+v", computers)
	}
}
