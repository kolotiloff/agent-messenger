package core

import (
	"errors"
	"strings"
	"testing"
)

const testBroker = "brk-00000000000000aa"

// netPair is two computers of one network: a starts tasks, b runs them.
type netPair struct {
	a, b taskFixture
	seq  int64
}

func newNetPair(t *testing.T) *netPair {
	t.Helper()
	p := &netPair{a: newTaskFixture(t), b: newTaskFixture(t)}
	agents := []NetAgent{{Agent: "claude"}, {Agent: "codex"}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.SetNetwork(testBroker, "dev-b", []NetPeer{{ID: HostPeerID, Name: "Студия", Host: true, Age: 1, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	return p
}

// HostPeerID is the broker's name for the hosting computer.
const HostPeerID = "host"

// carry delivers every waiting message of one side to the other, as the
// supervisor does through the broker. It returns the delivered messages.
func (p *netPair) carry(t *testing.T, fromA bool) []NetIncoming {
	t.Helper()
	from, to, sender := p.a.st, p.b.st, HostPeerID
	if !fromA {
		from, to, sender = p.b.st, p.a.st, "dev-b"
	}
	pending, err := from.NetPending(testBroker, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []NetIncoming
	for _, m := range pending {
		p.seq++
		in := NetIncoming{Seq: p.seq, From: sender, OpID: m.OpID, Body: m.Body}
		if err := to.ApplyNet(testBroker, in); err != nil {
			t.Fatal(err)
		}
		if err := from.NetSent(m.Seq); err != nil {
			t.Fatal(err)
		}
		out = append(out, in)
	}
	return out
}

func (p *netPair) startRemote(t *testing.T, op, provider, text string) string {
	t.Helper()
	id, created, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: op, Computer: "dev-b", Provider: provider, Text: text})
	if err != nil || !created {
		t.Fatalf("start %v %v", created, err)
	}
	return id
}

func (p *netPair) mirror(t *testing.T, origin string) Task {
	t.Helper()
	tasks, err := p.b.st.Tasks("Game", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range tasks {
		var o string
		_ = p.b.st.db.QueryRow(`SELECT COALESCE(origin_task,'') FROM tasks WHERE id=?`, tk.ID).Scan(&o)
		if o == origin {
			return tk
		}
	}
	t.Fatalf("no mirror of %s", origin)
	return Task{}
}

// D-056: a computer accepts tasks of its network until its owner turns that
// off.
func TestRemoteTaskIsAcceptedByDefault(t *testing.T) {
	p := newNetPair(t)
	if on, err := p.b.st.NetAccept(); err != nil || !on {
		t.Fatalf("default %v %v", on, err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	p.mirror(t, id)
}

func TestRemoteTaskIsRefusedUnlessTheOwnerAllows(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(false); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	task, _ := p.a.st.TaskByID(id)
	if task.State != "running" || task.Computer != "dev-b" || task.SessionID != "" {
		t.Fatalf("remote task %+v", task)
	}
	// Nothing runs on the origin.
	if runs, _ := p.a.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("origin has runs %v", runs)
	}
	p.carry(t, true)
	if tasks, _ := p.b.st.Tasks("Game", 10); len(tasks) != 0 {
		t.Fatalf("task accepted while not allowed: %+v", tasks)
	}
	p.carry(t, false)
	task, _ = p.a.st.TaskByID(id)
	if task.State != "failed" || !strings.Contains(task.Detail, "refused") {
		t.Fatalf("refused task %+v", task)
	}
	if got := kinds(p.a.events(t, AudienceCoordinator, 0)); got != "failed" {
		t.Fatalf("events %s", got)
	}
}

func TestRemoteTaskStartRefusals(t *testing.T) {
	p := newNetPair(t)
	for _, tc := range []struct{ computer, agent string }{{"dev-unknown", "codex"}, {"dev-b", "claude"}} {
		if tc.agent == "claude" {
			agents := []NetAgent{{Agent: "codex"}}
			if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Agents: agents}}); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "x-" + tc.computer + tc.agent, Computer: tc.computer, Provider: tc.agent, Text: "t"}); !errors.Is(err, ErrRejected) {
			t.Fatalf("%+v accepted: %v", tc, err)
		}
	}
	if err := p.a.st.SetNetwork("", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "no-net", Computer: "dev-b", Provider: "codex", Text: "t"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("no network: %v", err)
	}
	if pending, _ := p.a.st.NetPending(testBroker, 10); len(pending) != 0 {
		t.Fatalf("refused starts queued messages %+v", pending)
	}
}

func TestRemoteTaskQuestionAnswerReport(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	// The same operation again returns the same task and sends nothing new.
	if again, created, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op1", Computer: "dev-b", Provider: "codex", Text: "Build level 1"}); err != nil || created || again != id {
		t.Fatalf("repeat %s %v %v", again, created, err)
	}
	start := p.carry(t, true)
	if len(start) != 1 {
		t.Fatalf("start messages %+v", start)
	}
	m := p.mirror(t, id)
	if m.State != "running" || m.Provider != "codex" || m.Folder != p.b.folder || m.Text != "Build level 1" {
		t.Fatalf("mirror %+v", m)
	}
	// A start read twice creates nothing new.
	if err := p.b.st.ApplyNet(testBroker, start[0]); err != nil {
		t.Fatal(err)
	}
	if tasks, _ := p.b.st.Tasks("Game", 10); len(tasks) != 1 {
		t.Fatalf("duplicate mirror %+v", tasks)
	}
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	if task.Folder != p.b.folder || task.State != "running" {
		t.Fatalf("accepted %+v", task)
	}

	// The worker on b asks the owner.
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	p.b.dispose(t, r, DispositionAskOwner, "Which level?")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	m, _ = p.b.st.TaskByID(m.ID)
	if m.OpenQuestion == nil || !m.FromNetwork {
		t.Fatalf("question on b %+v", m)
	}
	// b's own coordinator and owner are not asked: the task is a's.
	if got := kinds(p.b.events(t, AudienceCoordinator, 0)) + kinds(p.b.events(t, AudienceOwner, 0)); got != "" {
		t.Fatalf("b's audiences got %s", got)
	}
	// Nobody answers it on b: the computer that started it does.
	if err := p.b.st.AnswerTask(m.ID, m.OpenQuestion.ID, "b1", "here", AudienceOwner); !errors.Is(err, ErrRejected) {
		t.Fatalf("local answer to another computer's task: %v", err)
	}
	p.carry(t, false)
	task, _ = p.a.st.TaskByID(id)
	if task.State != "waiting_owner" || task.OpenQuestion == nil || task.OpenQuestion.ID != m.OpenQuestion.ID || task.OpenQuestion.Text != "Which level?" {
		t.Fatalf("question on a %+v %+v", task, task.OpenQuestion)
	}
	owner := p.a.events(t, AudienceOwner, 0)
	if kinds(owner) != "question" || owner[0].QuestionID != task.OpenQuestion.ID {
		t.Fatalf("owner events %+v", owner)
	}
	if runs, _ := p.a.st.NextTaskRunIntents(10); len(runs) != 0 {
		t.Fatalf("question opened a run on a %v", runs)
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil || feed.Task.OwnerQuestion == nil || feed.Task.Remote == nil || feed.Task.Remote.Name != "Ноутбук" {
		t.Fatalf("feed %+v %v", feed.Task, err)
	}

	if err := p.a.st.WindowAnswer(id, task.OpenQuestion.ID, "w1", "Level 2"); err != nil {
		t.Fatal(err)
	}
	if task, _ = p.a.st.TaskByID(id); task.State != "running" {
		t.Fatalf("answered %+v", task)
	}
	answer := p.carry(t, true)
	if len(answer) != 1 {
		t.Fatalf("answer messages %+v", answer)
	}
	// An answer read twice opens one run.
	if err := p.b.st.ApplyNet(testBroker, answer[0]); err != nil {
		t.Fatal(err)
	}
	if runs, _ := p.b.st.NextTaskRunIntents(10); len(runs) != 1 {
		t.Fatalf("runs on b %v", runs)
	}
	r2 := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	brief, err := p.b.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || brief.Answer != "Level 2" {
		t.Fatalf("brief %+v %v", brief, err)
	}
	if err := p.b.st.RecordDisposition(r2.ID, r2.Receipt, DispositionReport, "Level 2 is built"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.b.st.FinishTaskRun(r2.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	task, _ = p.a.st.TaskByID(id)
	if task.State != "done" || task.OpenQuestion != nil {
		t.Fatalf("done %+v", task)
	}
	events := p.a.events(t, AudienceCoordinator, 0)
	if kinds(events) != "report" || events[0].Text != "Level 2 is built" {
		t.Fatalf("report %+v", events)
	}
	feed, _ = p.a.st.TaskFeed(id)
	var got []string
	for _, it := range feed.Items {
		got = append(got, it.Kind+":"+it.Author)
	}
	if strings.Join(got, ",") != "task:coordinator,accepted:messenger,question:worker,answer:owner,report:worker" {
		t.Fatalf("feed items %v", got)
	}
}

func TestRemoteTaskCancel(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	// Cancelled before its start left: cancelled at once, nothing is sent.
	early := p.startRemote(t, "early", "codex", "Never sent")
	if state, err := p.a.st.CancelTask(early); err != nil || state != "cancelled" {
		t.Fatalf("early cancel %s %v", state, err)
	}
	if pending, _ := p.a.st.NetPending(testBroker, 10); len(pending) != 0 {
		t.Fatalf("withdrawn start still queued %+v", pending)
	}

	id := p.startRemote(t, "op", "codex", "Long job")
	p.carry(t, true)
	p.carry(t, false)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000c1")
	if state, err := p.a.st.CancelTask(id); err != nil || state != "cancelling" {
		t.Fatalf("cancel %s %v", state, err)
	}
	// A repeated cancel sends nothing new.
	if state, _ := p.a.st.CancelTask(id); state != "cancelling" {
		t.Fatalf("repeat cancel %s", state)
	}
	if task, _ := p.a.st.TaskByID(id); task.State != "running" {
		t.Fatalf("cancelled before the other computer confirmed: %+v", task)
	}
	if sent := p.carry(t, true); len(sent) != 1 {
		t.Fatalf("cancel messages %+v", sent)
	}
	if runs, _ := p.b.st.CancelRequestedRuns(); len(runs) != 1 || runs[0].ID != r.ID {
		t.Fatalf("cancel not requested on b %+v", runs)
	}
	ev := cleanExit
	ev.Cancelled = true
	if _, err := p.b.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if task, _ := p.a.st.TaskByID(id); task.State != "cancelled" {
		t.Fatalf("not cancelled %+v", task)
	}
}

func TestRemoteAnswerAfterTheOwnerThereCancelled(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Ask")
	p.carry(t, true)
	p.carry(t, false)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000d1")
	p.b.dispose(t, r, DispositionAsk, "Which?")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	if task.State != "waiting_coordinator" || task.OpenQuestion == nil {
		t.Fatalf("waiting %+v", task)
	}
	// The owner of b stops the work on b, while a answers.
	if state, err := p.b.st.CancelTask(m.ID); err != nil || state != "cancelled" {
		t.Fatalf("cancel on b %s %v", state, err)
	}
	if err := p.a.st.AnswerTask(id, task.OpenQuestion.ID, "a1", "This one", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	p.carry(t, false)
	task, _ = p.a.st.TaskByID(id)
	if task.State != "cancelled" {
		t.Fatalf("after cancel on b %+v", task)
	}
}

func TestUndeliverableMessages(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	// A start whose recipient left: it never ran there.
	id := p.startRemote(t, "gone", "codex", "x")
	pending, _ := p.a.st.NetPending(testBroker, 10)
	if err := p.a.st.NetUndeliverable(pending[0].Seq); err != nil {
		t.Fatal(err)
	}
	if task, _ := p.a.st.TaskByID(id); task.State != "failed" {
		t.Fatalf("undeliverable start %+v", task)
	}
	// A cancel that cannot arrive: the outcome there is unknown.
	id = p.startRemote(t, "op", "codex", "y")
	p.carry(t, true)
	p.carry(t, false)
	if _, err := p.a.st.CancelTask(id); err != nil {
		t.Fatal(err)
	}
	pending, _ = p.a.st.NetPending(testBroker, 10)
	if err := p.a.st.NetUndeliverable(pending[0].Seq); err != nil {
		t.Fatal(err)
	}
	if task, _ := p.a.st.TaskByID(id); task.State != "blocked" {
		t.Fatalf("undeliverable cancel %+v", task)
	}
}

func TestNetPeersAndAcceptSetting(t *testing.T) {
	p := newNetPair(t)
	peers, err := p.a.st.NetPeers()
	if err != nil || len(peers) != 1 || peers[0].Name != "Ноутбук" || len(peers[0].Agents) != 2 {
		t.Fatalf("peers %+v %v", peers, err)
	}
	if err := p.b.st.SetNetAccept(false); err != nil {
		t.Fatal(err)
	}
	if on, _ := p.b.st.NetAccept(); on {
		t.Fatal("still accepting after turning it off")
	}
	agents, err := p.a.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	list := agents["computers"].([]ComputerSummary)
	if len(list) != 2 || list[1].Computer != "dev-b" || list[1].This || !list[1].Online {
		t.Fatalf("list_agents %+v", list)
	}
}

// D-055: the computer that started a task sees that it was accepted and the
// worker's progress lines from the computer that runs it.
func TestRemoteTaskProgressReachesTheStarter(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	p.carry(t, false)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	if err := p.b.st.AppendTaskRunItem(r.ID, "text", "", "Reading the level files"); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.AppendTaskRunItem(r.ID, "tool", "Команда", "ls levels"); err != nil {
		t.Fatal(err)
	}
	sent := p.carry(t, false)
	if len(sent) != 2 {
		t.Fatalf("progress messages %d", len(sent))
	}
	// A message read twice is applied once.
	if err := p.a.st.ApplyNet(testBroker, sent[0]); err != nil {
		t.Fatal(err)
	}
	f, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, it := range f.Items {
		kinds = append(kinds, it.Kind+":"+it.Text)
	}
	want := "task:Build level 1|accepted:Ноутбук|text:Reading the level files|tool:ls levels"
	if got := strings.Join(kinds, "|"); got != want {
		t.Fatalf("feed\n got %s\nwant %s", got, want)
	}
	if f.Items[3].Title != "Команда" {
		t.Fatalf("tool title %+v", f.Items[3])
	}
	// Progress is for the window only: the coordinator gets no event for it.
	for _, e := range p.a.events(t, AudienceCoordinator, 0) {
		if e.Kind == "text" || e.Kind == "tool" || e.Kind == "progress" {
			t.Fatalf("coordinator event %+v", e)
		}
	}
	// A task of this computer sends nothing.
	before, _ := p.a.st.NetPending(testBroker, 100)
	local := p.a.start(t, "op-local", "codex", "Local work")
	lr := p.a.launch(t, local, "019a0000-0000-7000-8000-0000000000c1")
	if err := p.a.st.AppendTaskRunItem(lr.ID, "text", "", "local line"); err != nil {
		t.Fatal(err)
	}
	if after, _ := p.a.st.NetPending(testBroker, 100); len(after) != len(before) {
		t.Fatalf("a local item was sent: %d -> %d", len(before), len(after))
	}
}
