package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTimelinePaginationAliasesAndConcurrentAppend(t *testing.T) {
	s, env := openTest(t)
	for _, alias := range []string{"alpha", "beta"} {
		if _, err := s.Register("synthetic", env, "private-id", alias); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Register("synthetic", env, "other-private-id", "other"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		enqueue(t, s, "alpha", id)
	}
	enqueue(t, s, "other", "other-message")
	// Pagination follows insertion order even if timestamps tie or regress.
	if _, err := s.db.Exec(`UPDATE requests SET created_at='same-time'`); err != nil {
		t.Fatal(err)
	}
	before := mustSnap(t, s)
	page, err := s.Timeline("beta", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "m4" || page.Items[1].ID != "m5" || !page.HasMore || page.NextBefore != "m4" {
		t.Fatalf("page %+v", page)
	}
	if !reflect.DeepEqual(before, mustSnap(t, s)) {
		t.Fatal("read mutated database")
	}
	enqueue(t, s, "alpha", "m6")
	page, err = s.Timeline("alpha", 2, page.NextBefore)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "m2" || page.Items[1].ID != "m3" || page.NextBefore != "m2" {
		t.Fatalf("older page %+v", page)
	}
	page, err = s.Timeline("beta", 2, page.NextBefore)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "m1" || page.HasMore || page.NextBefore != "" {
		t.Fatalf("oldest page %+v", page)
	}
	for _, cursor := range []string{"other-message", "missing"} {
		if _, err := s.Timeline("alpha", 2, cursor); err == nil {
			t.Fatal("foreign/missing cursor accepted")
		}
	}
	for _, limit := range []int{-1, 0, 101} {
		if _, err := s.Timeline("alpha", limit, ""); err == nil {
			t.Fatal("invalid limit accepted")
		}
	}
	if _, err := s.Timeline("missing", 2, ""); err == nil {
		t.Fatal("missing alias accepted")
	}
	page, err = s.Timeline("alpha", 2, "m1")
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.HasMore {
		t.Fatalf("empty page %+v %v", page, err)
	}
}

func TestTimelineReplyIsNotSuccessfulUntilVerified(t *testing.T) {
	for _, exit := range []int{0, 1} {
		t.Run(string(rune('0'+exit)), func(t *testing.T) {
			s, env := openTest(t)
			if _, err := s.Register("synthetic", env, "private-id", "worker"); err != nil {
				t.Fatal(err)
			}
			token := running(t, s, env, "worker", "m1", "private-id")
			reply, err := s.Reply(token, "saved answer")
			if err != nil {
				t.Fatal(err)
			}
			page, err := s.Timeline("worker", 10, "")
			if err != nil {
				t.Fatal(err)
			}
			if page.Items[0].Reply == nil || page.Items[0].Result != "not_verified" {
				t.Fatal("stored answer marked successful")
			}
			if _, err := s.FinishObserved(token, exit, true, true, "private process detail"); err != nil {
				t.Fatal(err)
			}
			page, err = s.Timeline("worker", 10, "")
			if err != nil {
				t.Fatal(err)
			}
			want := "verified_exit"
			if exit != 0 {
				want = "process_failed"
			}
			i := page.Items[0]
			if i.Result != want || i.State != "uncertain" || i.Reply.ID != reply.ID || i.Reply.ReplyTo != "m1" || i.ExitCode == nil || *i.ExitCode != exit {
				t.Fatalf("outcome %+v", i)
			}
			raw, _ := json.Marshal(page)
			for _, secret := range []string{env, "private-id", token, "private process detail"} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("private execution metadata leaked")
				}
			}
		})
	}
}

func TestTimelineStreamResultAndBrokenEvidence(t *testing.T) {
	for _, mutation := range []string{"", `UPDATE verifications SET provider_ok=0`, `UPDATE verifications SET identity_confirmed=0`, `UPDATE verifications SET reply_id='wrong'`, `UPDATE replies SET recipient='wrong'`, `UPDATE replies SET attempt_token='wrong'`, `UPDATE verifications SET exit_code=1,exit_observed=1`} {
		t.Run(mutation, func(t *testing.T) {
			s, run := streamTestStore(t)
			finishTestTurn(t, s, run)
			if mutation != "" {
				if _, err := s.db.Exec(mutation); err != nil {
					t.Fatal(err)
				}
			}
			page, err := s.Timeline("worker", 10, "")
			if err != nil {
				t.Fatal(err)
			}
			if mutation == "" {
				if page.Items[0].Result != "verified_turn" || page.Items[0].ExitCode != nil {
					t.Fatalf("stream result %+v", page.Items[0])
				}
				if err := s.StopStream(run, "test shutdown"); err != nil {
					t.Fatal(err)
				}
				page, err = s.Timeline("worker", 10, "")
				if err != nil || page.Items[0].Result != "verified_turn" {
					t.Fatal("shutdown erased verified turn", err)
				}
			} else if strings.HasPrefix(page.Items[0].Result, "verified_") {
				t.Fatal("broken evidence marked successful")
			}
		})
	}
}

func TestTimelineTextLimitAndRetryLink(t *testing.T) {
	s, env := openTest(t)
	if _, err := s.Register("synthetic", env, "private-id", "worker"); err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("界", timelineTextLimit+1)
	if _, err := s.Enqueue("m1", "worker", "sender", text); err != nil {
		t.Fatal(err)
	}
	enqueue(t, s, "worker", "m2")
	if _, err := s.db.Exec(`UPDATE requests SET retry_of='m1' WHERE id='m2'`); err != nil {
		t.Fatal(err)
	}
	page, err := s.Timeline("worker", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if !page.Items[0].TextTruncated || len([]rune(page.Items[0].Text)) != timelineTextLimit || page.Items[1].RetryOf != "m1" || page.Items[1].TextTruncated {
		t.Fatal("text limit or retry link incorrect")
	}
}
