package core

import (
	"strings"
	"testing"
	"time"
)

func contactEvents(t *testing.T, st *Store, project string) []TaskEvent {
	t.Helper()
	events, _, err := st.TaskEvents(project, AudienceCoordinator, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	var out []TaskEvent
	for _, e := range events {
		if e.Kind == NetworkLostEvent || e.Kind == NetworkBackEvent {
			out = append(out, e)
		}
	}
	return out
}

func TestNetworkContactLostAndBackOnce(t *testing.T) {
	allowTempFolders(t)
	st, _ := openTest(t)
	for _, p := range []string{"Game", "Site"} {
		if err := st.CreateProject(p, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	peers := func(age int) []NetPeer {
		return []NetPeer{{ID: "srv", Name: "ПК-2", Host: true, Age: age}, {ID: "nb", Name: "Ноут", Age: age}}
	}
	note := func() string {
		t.Helper()
		kind, err := st.NoteNetworkContact(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return kind
	}
	// Lost before any contact since joining: nothing to report.
	if err := st.SetNetwork("b1", "me", peers(300)); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != "" {
		t.Fatalf("event without earlier contact: %q", k)
	}
	if err := st.SetNetwork("b1", "me", peers(5)); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != "" {
		t.Fatalf("first contact is not news: %q", k)
	}
	// One computer still reports: not every contact is lost.
	if err := st.SetNetwork("b1", "me", []NetPeer{{ID: "srv", Name: "ПК-2", Host: true, Age: 5}, {ID: "nb", Name: "Ноут", Age: 300}}); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != "" {
		t.Fatalf("one computer lost is not all: %q", k)
	}
	if err := st.SetNetwork("b1", "me", peers(PeerFresh+30)); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != NetworkLostEvent {
		t.Fatalf("lost not noted: %q", k)
	}
	if k := note(); k != "" {
		t.Fatalf("lost noted twice: %q", k)
	}
	for _, p := range []string{"Game", "Site"} {
		ev := contactEvents(t, st, p)
		if len(ev) != 1 || ev[0].Kind != NetworkLostEvent || ev[0].TaskID != "" || !strings.Contains(ev[0].Text, "«ПК-2»") || !strings.Contains(ev[0].Text, "server runs on") {
			t.Fatalf("%s: %+v", p, ev)
		}
	}
	if err := st.SetNetwork("b1", "me", peers(3)); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != NetworkBackEvent {
		t.Fatalf("back not noted: %q", k)
	}
	if ev := contactEvents(t, st, "Game"); len(ev) != 2 || ev[1].Kind != NetworkBackEvent {
		t.Fatalf("events %+v", ev)
	}
	// Another network starts afresh: its first stale list is not news.
	if err := st.SetNetwork("b2", "me", peers(300)); err != nil {
		t.Fatal(err)
	}
	if k := note(); k != "" {
		t.Fatalf("new network inherited the state: %q", k)
	}
}
