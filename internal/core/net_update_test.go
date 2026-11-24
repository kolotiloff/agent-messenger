package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func netUpdate(t *testing.T, st *Store, peer string) NetUpdate {
	t.Helper()
	list, err := st.NetUpdates()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range list {
		if u.Computer == peer {
			return u
		}
	}
	return NetUpdate{}
}

// newUpdatePair is a network whose other computer reports its version.
func newUpdatePair(t *testing.T) *netPair {
	t.Helper()
	p := newNetPair(t)
	// An older computer reports no version and is not asked.
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "вручную") {
		t.Fatalf("computer without a version: %v", err)
	}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56"}}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNetUpdateRequestAndReplies(t *testing.T) {
	p := newUpdatePair(t)
	if _, err := p.a.st.RequestNetUpdate("dev-x", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown computer: %v", err)
	}
	id, err := p.a.st.RequestNetUpdate("dev-b", "")
	if err != nil || !netRef.MatchString(id) {
		t.Fatalf("request %q %v", id, err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateWaiting || u.ID != id || u.Stale {
		t.Fatalf("waiting %+v", u)
	}
	// A second request waits for the answer to the first.
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("duplicate request: %v", err)
	}
	p.carry(t, true)
	reqs, err := p.b.st.NetUpdateRequests(testBroker)
	if err != nil || len(reqs) != 1 || reqs[0].ID != id || reqs[0].Peer != HostPeerID {
		t.Fatalf("requests %+v %v", reqs, err)
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], "bogus", "", ""); err == nil {
		t.Fatal("unknown state accepted")
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateUpdating, "", ""); err != nil {
		t.Fatal(err)
	}
	if reqs, _ := p.b.st.NetUpdateRequests(testBroker); len(reqs) != 0 {
		t.Fatalf("answered request stays %+v", reqs)
	}
	// The final result goes after the first answer.
	if err := p.b.st.FinishNetUpdate(testBroker, HostPeerID, id, UpdateDone, "0.0.0-preview.57", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateDone || u.Version != "0.0.0-preview.57" {
		t.Fatalf("done %+v", u)
	}

	// A final answer is not replaced, and a late "updating" changes nothing.
	_ = p.a.st.ApplyNet(testBroker, NetIncoming{Seq: 900, From: "dev-b", OpID: "late." + id, Body: []byte(`{"kind":"update","task":"` + id + `","state":"updating"}`)})
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateDone {
		t.Fatalf("final state replaced %+v", u)
	}

	// A refusal comes back with its reason; a new request may follow.
	id2, err := p.a.st.RequestNetUpdate("dev-b", "")
	if err != nil || id2 == id {
		t.Fatalf("second request %q %v", id2, err)
	}
	p.carry(t, true)
	reqs, _ = p.b.st.NetUpdateRequests(testBroker)
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateRefused, "", "идут задачи"); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateRefused || u.Text != "идут задачи" || u.ID != id2 {
		t.Fatalf("refused %+v", u)
	}

	// Replies from another computer, for an old request or with an unknown
	// state change nothing.
	id3, _ := p.a.st.RequestNetUpdate("dev-b", "")
	for i, m := range []NetIncoming{
		{From: "dev-z", Body: []byte(`{"kind":"update","task":"` + id3 + `","state":"done"}`)},
		{From: "dev-b", Body: []byte(`{"kind":"update","task":"` + id + `","state":"failed"}`)},
		{From: "dev-b", Body: []byte(`{"kind":"update","task":"` + id3 + `","state":"installed"}`)},
	} {
		m.Seq, m.OpID = int64(1000+i), "x."+id3+"."+string(rune('a'+i))
		if err := p.a.st.ApplyNet(testBroker, m); err != nil {
			t.Fatal(err)
		}
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateWaiting || u.ID != id3 {
		t.Fatalf("foreign reply accepted %+v", u)
	}

	// An old unanswered request is shown as stale and may be asked again.
	if _, err := p.a.st.db.Exec(`UPDATE net_updates SET created_at=? WHERE id=?`, time.Now().UTC().Add(-UpdateWait-time.Minute).Format(time.RFC3339Nano), id3); err != nil {
		t.Fatal(err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); !u.Stale {
		t.Fatalf("not stale %+v", u)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); err != nil {
		t.Fatalf("request after a stale one: %v", err)
	}
}

func TestNetUpdateRefusals(t *testing.T) {
	p := newUpdatePair(t)
	// A computer that is not on line is not asked.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: PeerFresh + 10, Version: "0.0.0-preview.56"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "не на связи") {
		t.Fatalf("offline computer: %v", err)
	}
	if err := p.a.st.SetNetwork("", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("no network: %v", err)
	}
	// A final result with a bad id or state is refused; one for another
	// network is dropped.
	for _, c := range [][3]string{{"bad id!", UpdateDone, ""}, {"upd-1", UpdateUpdating, ""}, {"upd-1", "installed", ""}} {
		if err := p.b.st.FinishNetUpdate(testBroker, HostPeerID, c[0], c[1], "", ""); !errors.Is(err, ErrRejected) {
			t.Errorf("%v accepted: %v", c, err)
		}
	}
	if err := p.b.st.FinishNetUpdate("other-broker", HostPeerID, "upd-1", UpdateDone, "", ""); err != nil {
		t.Fatal(err)
	}
	if pending, _ := p.b.st.NetPending("other-broker", 10); len(pending) != 0 {
		t.Fatalf("result for another network queued %+v", pending)
	}
}

func TestNetUpdateUndeliverable(t *testing.T) {
	p := newUpdatePair(t)
	id, _ := p.a.st.RequestNetUpdate("dev-b", "")
	pending, _ := p.a.st.NetPending(testBroker, 10)
	if len(pending) != 1 {
		t.Fatalf("pending %+v", pending)
	}
	if err := p.a.st.NetUndeliverable(pending[0].Seq); err != nil {
		t.Fatal(err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.ID != id || u.State != UpdateFailed {
		t.Fatalf("undeliverable %+v", u)
	}
}

func TestActiveWork(t *testing.T) {
	f := newTaskFixture(t)
	if n, err := f.st.ActiveWork(); err != nil || n != 0 {
		t.Fatalf("empty %d %v", n, err)
	}
	id := f.start(t, "op-1", "claude", "работа")
	if n, _ := f.st.ActiveWork(); n != 1 {
		t.Fatalf("running task not counted: %d", n)
	}
	if n, _ := f.st.LiveWork(); n != 1 {
		t.Fatalf("running task not live: %d", n)
	}
	if _, err := f.st.CancelTask(id); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.st.ActiveWork(); n != 0 {
		t.Fatalf("cancelled task counted: %d", n)
	}
}

// T-155: an interrupted or blocked task stays in ActiveWork but does not
// hold an update. A queued update of this computer refuses a new start.
func TestLiveWorkAndLocalUpdate(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "claude", "работа")
	f.launch(t, id, f.sessionOf(t, id))
	interrupt(t, f.st, id)
	if n, _ := f.st.ActiveWork(); n != 1 {
		t.Fatalf("interrupted not active: %d", n)
	}
	if n, _ := f.st.LiveWork(); n != 0 {
		t.Fatalf("interrupted held the update: %d", n)
	}

	upd, err := f.st.RequestNetUpdate("", "Game")
	if err != nil || upd == "" {
		t.Fatalf("local update %q %v", upd, err)
	}
	if pending, err := f.st.HasPendingUpdate(); err != nil || !pending {
		t.Fatalf("pending %v %v", pending, err)
	}
	if u, ok, err := f.st.LocalUpdate(); err != nil || !ok || u.State != UpdateQueued || u.ID != upd {
		t.Fatalf("local row %+v %v %v", u, ok, err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-new", Provider: "codex", Text: "ещё"}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("new task accepted: %v", err)
	}
	if _, err := f.st.RequestNetUpdate("", "Game"); !errors.Is(err, ErrRejected) {
		t.Fatalf("duplicate local update: %v", err)
	}
	events, _, err := f.st.TaskEvents("Game", AudienceCoordinator, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Kind == MessengerUpdateEvent && e.TaskID == "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no messenger_update %+v", events)
	}
	out, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	computers := out["computers"].([]ComputerSummary)
	if len(computers) == 0 || !computers[0].This || !computers[0].UpdatePending {
		t.Fatalf("this computer not waiting %+v", computers)
	}
	if _, ok := out["idle_computers"]; ok {
		t.Fatalf("waiting computer is idle: %v", out["idle_computers"])
	}
	if err := f.st.FinishPendingUpdate(upd, UpdateDone, "0.0.0-preview.57", ""); err != nil {
		t.Fatal(err)
	}
	if pending, _ := f.st.HasPendingUpdate(); pending {
		t.Fatal("pending after done")
	}
	if u, ok, _ := f.st.LocalUpdate(); !ok || u.State != UpdateDone || u.Version != "0.0.0-preview.57" {
		t.Fatalf("done row %+v", u)
	}
}

func TestNetUpdateRefusesAWaitingPeer(t *testing.T) {
	p := newUpdatePair(t)
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56", UpdatePending: true, Agents: []NetAgent{{Agent: "codex"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b", ""); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "ждёт") {
		t.Fatalf("waiting peer: %v", err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-b", Provider: "codex", Text: "x", Computer: "dev-b"}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("remote start to waiting peer: %v", err)
	}
}

func TestNetPeerVersion(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56"}, {ID: "dev-c", Name: "Старый", Age: 1}}); err != nil {
		t.Fatal(err)
	}
	peers, err := f.st.NetPeers()
	if err != nil || len(peers) != 2 || peers[0].Version != "0.0.0-preview.56" || peers[1].Version != "" {
		t.Fatalf("peers %+v %v", peers, err)
	}
}

// T-204: the computer drops its flag when it sends the result, but its next
// presence comes a round later. The final result clears the flag at once.
func TestNetUpdateResultClearsStalePendingFlag(t *testing.T) {
	p := newUpdatePair(t)
	id, err := p.a.st.RequestNetUpdate("dev-b", "")
	if err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	reqs, err := p.b.st.NetUpdateRequests(testBroker)
	if err != nil || len(reqs) != 1 {
		t.Fatalf("requests %+v %v", reqs, err)
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateQueued, "", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	stale := []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56", UpdatePending: true, Agents: []NetAgent{{Agent: "codex"}}}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, stale); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-wait", Provider: "codex", Text: "x", Computer: "dev-b"}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("queued update must refuse tasks: %v", err)
	}
	if err := p.b.st.FinishNetUpdate(testBroker, HostPeerID, id, UpdateDone, "0.0.0-preview.57", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	// The server still hands out the old presence.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, stale); err != nil {
		t.Fatal(err)
	}
	peers, err := p.a.st.NetPeers()
	if err != nil || len(peers) != 1 || peers[0].UpdatePending || peers[0].Version != "0.0.0-preview.57" {
		t.Fatalf("after done %+v %v", peers, err)
	}
	// A result also advances a version that never reported a pending flag.
	stale[0].UpdatePending = false
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, stale); err != nil {
		t.Fatal(err)
	}
	if peers, err := p.a.st.NetPeers(); err != nil || len(peers) != 1 || peers[0].Version != "0.0.0-preview.57" {
		t.Fatalf("version without pending flag: %+v %v", peers, err)
	}
	// Another computer can already have queued a different request while
	// this computer receives the result of the previous one.
	other := append([]NetPeer(nil), stale...)
	other[0].UpdatePending, other[0].UpdateRequestID = true, "upd-next"
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, other); err != nil {
		t.Fatal(err)
	}
	if peers, err := p.a.st.NetPeers(); err != nil || len(peers) != 1 || !peers[0].UpdatePending || peers[0].UpdateRequestID != "upd-next" {
		t.Fatalf("a previous result hid a different pending request: %+v %v", peers, err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-new-update", Provider: "codex", Text: "x", Computer: "dev-b"}); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("a different queued update must still refuse tasks: %v", err)
	}
	newer := append([]NetPeer(nil), stale...)
	newer[0].Version, newer[0].UpdatePending = "0.0.0-preview.58", true
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, newer); err != nil {
		t.Fatal(err)
	}
	if peers, err := p.a.st.NetPeers(); err != nil || len(peers) != 1 || peers[0].Version != "0.0.0-preview.58" || !peers[0].UpdatePending {
		t.Fatalf("newer presence regressed: %+v %v", peers, err)
	}
	stale[0].UpdatePending = true
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, stale); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-done", Provider: "codex", Text: "x", Computer: "dev-b"}); err != nil && strings.Contains(err.Error(), "update") {
		t.Fatalf("updated computer still refused: %v", err)
	}
	// A result older than the window no longer outweighs the presence.
	if _, err := p.a.st.db.Exec(`UPDATE net_updates SET updated_at=? WHERE peer='dev-b'`, time.Now().Add(-updateResultFresh-time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if peers, _ := p.a.st.NetPeers(); len(peers) != 1 || !peers[0].UpdatePending {
		t.Fatalf("old result %+v", peers)
	}
}

// T-211: an older computer would refuse a model with settings as invalid;
// the coordinator says why before sending.
func TestRemoteStartRefusesModelSettingsForAnOldPeer(t *testing.T) {
	p := newUpdatePair(t)
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.62", Agents: []NetAgent{{Agent: "claude"}}}}); err != nil {
		t.Fatal(err)
	}
	_, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-b", Provider: "claude", Model: "claude-opus-5-5[effort=medium]", Text: "x", Computer: "dev-b"})
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "0.0.0-preview.62") || !strings.Contains(err.Error(), "update_messenger") {
		t.Fatalf("old peer: %v", err)
	}
	if _, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-c", Provider: "claude", Model: "claude-opus-5-5", Text: "x", Computer: "dev-b"}); err != nil {
		t.Fatalf("plain model to old peer: %v", err)
	}
}
