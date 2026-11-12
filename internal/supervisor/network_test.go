package supervisor

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/journal"
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
	// T-142: both computers of this test write one journal.
	journalDir := t.TempDir()
	journal.Open(journalDir)
	t.Cleanup(journal.Close)
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
	raw, _ := os.ReadFile(filepath.Join(journalDir, journal.File))
	for _, want := range []string{"sent to «Ноутбук»: start " + id, "received from", ": start " + id, ": accepted " + id} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("journal has no %q:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "Build level 1") {
		t.Fatalf("task text in the journal:\n%s", raw)
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

// A computer's token usage reaches the others and their statistics can be
// filtered by it (T-076).
func TestUsageReachesTheOtherComputers(t *testing.T) {
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
	p, err := b.presence()
	if err != nil || p.Usage == nil || p.Usage.Days == nil {
		t.Fatalf("b reports no usage: %+v %v", p.Usage, err)
	}
	b.server.ReportPresence(ctx)
	a.networkRound(ctx)
	peers, err := a.store.NetPeers()
	if err != nil || len(peers) != 1 || peers[0].Usage == nil {
		t.Fatalf("a's peers %+v %v", peers, err)
	}
	value, err := a.UsageStats("week", peers[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if s := value.(core.UsageStats); s.Computer != "Ноутбук" || s.Total.Tasks != 0 {
		t.Fatalf("stats of b %+v", s)
	}
}

func TestUpdateOfAnotherComputer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	oldOS := updateOS
	updateOS = "linux"
	t.Cleanup(func() { updateOS = oldOS })
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
	if len(peers) != 1 || peers[0].Version != buildinfo.Version {
		t.Fatalf("a's peers %+v", peers)
	}
	peer := peers[0].ID
	round := func() {
		t.Helper()
		a.networkRound(ctx) // sends what a has
		b.networkRound(ctx) // reads it and answers
		a.networkRound(ctx) // reads the answer
		if p := a.networkProblem() + b.networkProblem(); p != "" {
			t.Fatalf("network problem %q", p)
		}
	}
	status := func() core.NetUpdate {
		t.Helper()
		list, err := a.store.NetUpdates()
		if err != nil || len(list) != 1 {
			t.Fatalf("updates %+v %v", list, err)
		}
		return list[0]
	}
	ask := func() core.NetUpdate {
		t.Helper()
		if _, err := a.store.RequestNetUpdate(peer); err != nil {
			t.Fatal(err)
		}
		round()
		return status()
	}

	// A client started without the hook (from a terminal) refuses.
	if u := ask(); u.State != core.UpdateRefused || u.Text != updateRefusedUnsupported {
		t.Fatalf("no hook %+v", u)
	}
	var started []RemoteUpdate
	var programs []string
	var startErr error
	b.startUpdate = func(program string, job RemoteUpdate) error {
		programs, started = append(programs, program), append(started, job)
		return startErr
	}
	// The window never recorded the program file.
	if u := ask(); u.State != core.UpdateRefused || u.Text != updateRefusedProgram {
		t.Fatalf("no program %+v", u)
	}
	program := filepath.Join(b.root, "messenger")
	if err := os.WriteFile(program, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteProgramPath(b.root, program); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(b.root, ProgramFile)); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("program file %v %v", fi, err)
	}
	// A task that is not finished holds the update back.
	task, _, err := b.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op-1", Provider: "codex", Text: "работа"})
	if err != nil {
		t.Fatal(err)
	}
	if u := ask(); u.State != core.UpdateRefused || !strings.Contains(u.Text, "идут задачи (1)") || !strings.Contains(u.Text, "«работа» (") {
		t.Fatalf("running task %+v", u)
	}
	if _, err := b.store.CancelTask(task); err != nil {
		t.Fatal(err)
	}
	// A Mac updates in its own app.
	updateOS = "darwin"
	if u := ask(); u.State != core.UpdateRefused || u.Text != updateRefusedMac {
		t.Fatalf("mac %+v", u)
	}
	updateOS = "linux"
	if len(started) != 0 {
		t.Fatalf("helper started on a refusal: %+v", started)
	}

	// Nothing holds it back: the helper starts and the answer is "updating".
	if u := ask(); u.State != core.UpdateUpdating {
		t.Fatalf("updating %+v", u)
	}
	if len(started) != 1 || programs[0] != program || started[0].Peer != "host" || started[0].ID != status().ID {
		t.Fatalf("helper %+v %v", started, programs)
	}
	if _, err := os.Stat(filepath.Join(b.root, remoteUpdateMarker)); err != nil {
		t.Fatalf("no marker: %v", err)
	}
	// The helper's result is sent by the client and its files removed.
	job := started[0]
	job.State, job.Version = core.UpdateDone, "0.0.0-preview.57"
	if err := FinishRemoteUpdate(b.root, job); err != nil {
		t.Fatal(err)
	}
	round()
	if u := status(); u.State != core.UpdateDone || u.Version != "0.0.0-preview.57" {
		t.Fatalf("done %+v", u)
	}
	for _, name := range []string{RemoteUpdateResult, remoteUpdateMarker} {
		if _, err := os.Stat(filepath.Join(b.root, name)); !os.IsNotExist(err) {
			t.Fatalf("%s left: %v", name, err)
		}
	}

	// A helper that does not start is reported as failed, never retried.
	startErr = errors.New("no such file")
	if u := ask(); u.State != core.UpdateFailed || !strings.Contains(u.Text, "no such file") {
		t.Fatalf("start failed %+v", u)
	}
	if len(started) != 2 {
		t.Fatalf("helper retried: %d", len(started))
	}
	if _, err := os.Stat(filepath.Join(b.root, remoteUpdateMarker)); !os.IsNotExist(err) {
		t.Fatalf("marker left: %v", err)
	}

	// While a helper runs, another request is refused; a helper that never
	// reports is answered as failed after a while.
	startErr = nil
	if u := ask(); u.State != core.UpdateUpdating {
		t.Fatalf("updating again %+v", u)
	}
	if refusal := b.updateRefusal(); refusal != updateRefusedBusy {
		t.Fatalf("busy refusal %q", refusal)
	}
	past := time.Now().Add(-updateMarkerAge - time.Minute)
	if err := os.Chtimes(filepath.Join(b.root, remoteUpdateMarker), past, past); err != nil {
		t.Fatal(err)
	}
	round()
	if u := status(); u.State != core.UpdateFailed || !strings.Contains(u.Text, "не сообщил результат") {
		t.Fatalf("silent helper %+v", u)
	}
	if len(started) != 3 {
		t.Fatalf("helper retried: %d", len(started))
	}
}

// T-137: messages waiting for a computer whose mailbox is full (it is still
// listed but nobody reads it) must not hold back the messages to the
// others, however many there are.
func TestFullMailboxDoesNotHoldBackOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := netComputer(t, ctx, "a")
	b, _ := netComputer(t, ctx, "b")
	c, _ := netComputer(t, ctx, "c")
	if err := a.server.Enable(broker.ScopeLocal, freeLocalPort(t)); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct {
		sup  *Supervisor
		name string
	}{{b, "Ноутбук"}, {c, "Старый"}} {
		code, err := a.server.Invite(a.server.Status().Host.Addresses[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := m.sup.server.Connect(ctx, code.Code, m.name); err != nil {
			t.Fatal(err)
		}
		m.sup.server.ReportPresence(ctx)
	}
	a.server.ReportPresence(ctx)
	b.server.ReportPresence(ctx)
	a.networkRound(ctx)
	b.networkRound(ctx)
	peers, err := b.store.NetPeers()
	if err != nil {
		t.Fatal(err)
	}
	var dead string
	for _, p := range peers {
		if p.Name == "Старый" {
			dead = p.ID
		}
	}
	if dead == "" {
		t.Fatalf("b's peers %+v", peers)
	}
	if err := c.store.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	if err := a.store.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	// c never reads its mail: fill b's share of its mailbox.
	n, _ := b.server.Network()
	for i := 0; i < 100; i++ {
		if err := b.server.Send(ctx, n, dead, "fill-"+strconv.Itoa(i), []byte(`{}`)); err != nil {
			t.Fatal(i, err)
		}
	}
	for i := 0; i < 60; i++ {
		if _, _, err := b.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "dead-" + strconv.Itoa(i), Computer: dead, Provider: "codex", Text: "never read"}); err != nil {
			t.Fatal(err)
		}
	}
	id, _, err := b.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "live", Computer: broker.HostPeer, Provider: "codex", Text: "Build level 1"})
	if err != nil {
		t.Fatal(err)
	}
	b.networkRound(ctx) // the start to a goes although c's mailbox is full
	a.networkRound(ctx) // a accepts it
	b.networkRound(ctx) // b applies the acceptance
	task, _ := b.store.TaskByID(id)
	if task.Folder == "" {
		t.Fatalf("the start to a was held back: %+v, problem %q", task, b.networkProblem())
	}
	if !strings.Contains(b.networkProblem(), "не отправлены") {
		t.Fatalf("the full mailbox is not reported: %q", b.networkProblem())
	}
}
