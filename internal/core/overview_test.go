package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestOverviewMergesAliasesAndDoesNotDisclosePrivateState(t *testing.T) {
	s, env := openTest(t)
	key, err := s.Register("claude", env, "private-conversation", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register("claude", env, "private-conversation", "beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("private-message-id", "beta", "private-sender", "private-payload"); err != nil {
		t.Fatal(err)
	}
	before, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Sessions) != 1 || view.Truncated {
		t.Fatalf("unexpected view: %+v", view)
	}
	c := view.Sessions[0]
	if !reflect.DeepEqual(c.Aliases, []string{"alpha", "beta"}) || c.Queued != 1 || c.OwnerRetained || c.LaunchConfigured || c.Authentication != "not_checked" || c.Readiness != "not_checked" {
		t.Fatalf("incorrect card: %+v", c)
	}
	raw, _ := json.Marshal(view)
	for _, secret := range []string{key, env, "private-conversation", "private-message-id", "private-sender", "private-payload"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("private field escaped projection")
		}
	}
	after, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("overview changed persistent state")
	}
}

func TestOverviewIdleOwnerAndUncertaintyRemainDistinct(t *testing.T) {
	s, env := openTest(t)
	key, err := s.Register("claude", env, "private-conversation", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("m1", "alpha", "sender", "text"); err != nil {
		t.Fatal(err)
	}
	token, err := s.Claim("alpha")
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic recorded states: overview must never mistake a retained owner
	// between stream turns for current model activity or proven process liveness.
	if _, err := s.db.Exec(`INSERT INTO stream_runs(id,session_key,state) VALUES (?,?,'idle')`, token, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE attempts SET state='turn_completed' WHERE token=?`, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE requests SET state='completed' WHERE id='m1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("m2", "alpha", "sender", "text"); err != nil {
		t.Fatal(err)
	}
	if err := s.Pause(); err != nil {
		t.Fatal(err)
	}
	view, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	c := view.Sessions[0]
	if !view.Paused || !c.OwnerRetained || c.RecordedStreamState != "idle" || c.RecordedAttemptState != "turn_completed" || c.Queued != 1 || c.Uncertain != 0 || c.Readiness != "not_checked" {
		t.Fatalf("idle card: %+v", view)
	}
	if _, err := s.db.Exec(`UPDATE stream_runs SET state='uncertain'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE attempts SET state='uncertain'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE requests SET state='uncertain' WHERE id='m1'`); err != nil {
		t.Fatal(err)
	}
	view, err = s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	c = view.Sessions[0]
	if !c.OwnerRetained || c.RecordedStreamState != "uncertain" || c.Uncertain != 1 || c.Queued != 1 {
		t.Fatalf("uncertain card: %+v", c)
	}
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), token) {
		t.Fatal("attempt token leaked")
	}
}

func TestOverviewBoundsSessionsAndAliases(t *testing.T) {
	s, env := openTest(t)
	for i := 0; i <= overviewLimit; i++ {
		if _, err := s.Register("claude", env, fmt.Sprint(i), fmt.Sprintf("session-%03d", i)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Register("claude", env, "0", fmt.Sprintf("alias-%03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if !view.Truncated || len(view.Sessions) != overviewLimit || !view.Sessions[0].AliasesTruncated || len(view.Sessions[0].Aliases) != overviewLimit {
		t.Fatal("unbounded or silently truncated overview")
	}
}

func TestOverviewEmptySessionsAreArray(t *testing.T) {
	s, _ := openTest(t)
	view, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if view.Sessions == nil || len(view.Sessions) != 0 || view.Truncated {
		t.Fatalf("empty view: %+v", view)
	}
}
