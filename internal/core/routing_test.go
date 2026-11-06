package core

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func routingStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "env"))
	st, err := Open(filepath.Join(root, "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for alias, conversation := range map[string]string{"a": "conv-a", "b": "conv-b"} {
		if _, err := st.Register("synthetic", env, conversation, alias); err != nil {
			t.Fatal(err)
		}
	}
	return st, env
}

func replyFromA(t *testing.T, st *Store, env, sender string, hops int) {
	t.Helper()
	if _, err := st.Enqueue("m1", "a", sender, "hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE requests SET hops=? WHERE id='m1'`, hops); err != nil {
		t.Fatal(err)
	}
	token := claimBeginConfirm(t, st, env, "a", "conv-a")
	if _, err := st.Reply(token, "ACK"); err != nil {
		t.Fatal(err)
	}
}

func findRequest(snap Snapshot, sessionOf string) []Row {
	var out []Row
	for _, r := range snap.Requests {
		if r["in_reply_to"] == sessionOf {
			out = append(out, r)
		}
	}
	return out
}

func TestReplyToAnotherAgentIsQueuedForIt(t *testing.T) {
	st, env := routingStore(t)
	replyFromA(t, st, env, "b", 0)
	snap := mustSnap(t, st)
	routed := findRequest(snap, "m1")
	if len(routed) != 1 {
		t.Fatalf("routed %+v", snap.Requests)
	}
	r := routed[0]
	if r["sender"] != "a" || r["text"] != "ACK" || r["state"] != "queued" || r["hops"] != int64(1) || r["id"] != "reply-"+snap.Replies[0]["id"].(string) {
		t.Fatalf("routed message %+v", r)
	}
	var key string
	if err := st.db.QueryRow(`SELECT session_key FROM aliases WHERE alias='b'`).Scan(&key); err != nil || r["session_key"] != key {
		t.Fatalf("routed to the wrong session: %+v", r)
	}
}

func TestAutoReplyLimitStopsTheChain(t *testing.T) {
	st, env := routingStore(t)
	replyFromA(t, st, env, "b", MaxAutoReplyHops-1)
	if r := findRequest(mustSnap(t, st), "m1"); len(r) != 1 || r[0]["hops"] != int64(MaxAutoReplyHops) {
		t.Fatalf("last allowed hop: %+v", r)
	}

	st2, env2 := routingStore(t)
	replyFromA(t, st2, env2, "b", MaxAutoReplyHops)
	snap := mustSnap(t, st2)
	if len(findRequest(snap, "m1")) != 0 || len(snap.Replies) != 1 {
		t.Fatalf("reply past the limit was routed: %+v", snap.Requests)
	}
	limited := false
	for _, e := range snap.Events {
		limited = limited || e["kind"] == "auto_reply_limit"
	}
	if !limited {
		t.Fatal("limit not recorded")
	}
}

func TestRepliesToUsersClosedOrOwnSessionsAreNotRouted(t *testing.T) {
	for name, sender := range map[string]string{"user": "user", "own alias": "a"} {
		t.Run(name, func(t *testing.T) {
			st, env := routingStore(t)
			replyFromA(t, st, env, sender, 0)
			if snap := mustSnap(t, st); len(snap.Requests) != 1 {
				t.Fatalf("routed: %+v", snap.Requests)
			}
		})
	}
	st, env := routingStore(t)
	if err := st.FailClosed("b"); err != nil {
		t.Fatal(err)
	}
	replyFromA(t, st, env, "b", 0)
	if snap := mustSnap(t, st); len(snap.Requests) != 1 {
		t.Fatalf("routed to a closed session: %+v", snap.Requests)
	}
}

func sendFromA(t *testing.T, st *Store, token, to, text string) (string, error) {
	t.Helper()
	var id string
	err := st.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "running")
		if err != nil {
			return err
		}
		id, err = sendTx(tx, row, to, text)
		return err
	})
	return id, err
}

func TestAgentSendIsBoundedAndRouted(t *testing.T) {
	st, env := routingStore(t)
	if _, err := st.Enqueue("m1", "a", "user", "hello"); err != nil {
		t.Fatal(err)
	}
	token := claimBeginConfirm(t, st, env, "a", "conv-a")
	for name, to := range map[string]string{"unknown": "nobody", "self": "a", "empty": ""} {
		if _, err := sendFromA(t, st, token, to, "x"); !errors.Is(err, ErrRejected) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
	for i := 0; i < MaxSendsPerTurn; i++ {
		if _, err := sendFromA(t, st, token, "b", "task"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sendFromA(t, st, token, "b", "task"); !errors.Is(err, ErrRejected) {
		t.Fatalf("per-turn limit not enforced: %v", err)
	}
	snap := mustSnap(t, st)
	sent := 0
	for _, r := range snap.Requests {
		if r["sender"] == "a" && r["hops"] == int64(1) && r["in_reply_to"] == "m1" {
			sent++
		}
	}
	if sent != MaxSendsPerTurn {
		t.Fatalf("sent %d: %+v", sent, snap.Requests)
	}
}

func TestAgentSendRespectsChainLimit(t *testing.T) {
	st, env := routingStore(t)
	if _, err := st.Enqueue("m1", "a", "b", "hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE requests SET hops=? WHERE id='m1'`, MaxAutoReplyHops); err != nil {
		t.Fatal(err)
	}
	token := claimBeginConfirm(t, st, env, "a", "conv-a")
	if _, err := sendFromA(t, st, token, "b", "more"); !errors.Is(err, ErrRejected) {
		t.Fatalf("send past the chain limit: %v", err)
	}
}
