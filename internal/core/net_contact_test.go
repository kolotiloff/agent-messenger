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

func TestNetworkContactDuringMacUpdate(t *testing.T) {
	for _, scenario := range []string{"recovers", "overdue", "different network"} {
		t.Run(scenario, func(t *testing.T) {
			f := newTaskFixture(t)
			at := time.Now()
			setNetwork := func(broker string, age int) {
				t.Helper()
				if err := f.st.SetNetwork(broker, "mac", []NetPeer{{ID: "host", Name: "Server", Host: true, Age: age}}); err != nil {
					t.Fatal(err)
				}
			}
			noteContact := func(at time.Time, want string) {
				t.Helper()
				if kind, err := f.st.NoteNetworkContact(at); err != nil || kind != want {
					t.Fatalf("contact event = %q, want %q (%v)", kind, want, err)
				}
			}
			setNetwork("b1", 3)
			noteContact(at, "")
			for _, when := range []time.Time{at, at.Add(time.Second)} {
				if err := f.st.NoteMacUpdateRestart(when); err != nil {
					t.Fatal(err)
				}
			}
			events, _, err := f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
			if err != nil || len(events) != 1 || events[0].Kind != MacUpdateRestartEvent {
				t.Fatalf("expected one update notice: %+v %v", events, err)
			}
			if scenario == "different network" {
				setNetwork("b2", 3)
				noteContact(at.Add(time.Second), "")
				setNetwork("b2", 90)
				noteContact(at.Add(10*time.Second), NetworkLostEvent)
				return
			}
			setNetwork("b1", 90)
			noteContact(at.Add(time.Minute), "")
			if ev := contactEvents(t, f.st, "Game"); len(ev) != 0 {
				t.Fatalf("planned outage raised a network alert: %+v", ev)
			}
			if scenario == "recovers" {
				setNetwork("b1", 3)
				noteContact(at.Add(70*time.Second), "")
				setNetwork("b1", 90)
				noteContact(at.Add(90*time.Second), NetworkLostEvent)
			} else {
				// Only the coordinator is polling now, as when the client
				// stopped without its replacement starting.
				for _, when := range []time.Time{at.Add(110 * time.Second), at.Add(macUpdateRestartGrace), at.Add(3 * time.Minute)} {
					if err := f.st.CheckExpiredMacUpdateRestart(when); err != nil {
						t.Fatal(err)
					}
				}
			}
			ev := contactEvents(t, f.st, "Game")
			if len(ev) != 1 || ev[0].Kind != NetworkLostEvent {
				t.Fatalf("expected exactly one real loss event: %+v", ev)
			}
			setNetwork("b1", 3)
			noteContact(at.Add(4*time.Minute), NetworkBackEvent)
		})
	}
}
