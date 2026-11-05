package core

import (
	"errors"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"testing"
)

func streamTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	s, env := openTest(t)
	if _, err := s.Register("claude", env, "test-id", "worker"); err != nil {
		t.Fatal(err)
	}
	enqueue(t, s, "worker", "m1")
	run, err := s.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BeginLaunch(run); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginStream(run); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordProcess(run, platform.Observation{PID: 1234, PGID: 1234, StartToken: "start", BootID: "boot"}); err != nil {
		t.Fatal(err)
	}
	return s, run
}

func TestFailStreamStartRejectsObservedProcess(t *testing.T) {
	s, run := streamTestStore(t)
	if err := s.FailStreamStart(run, "not a definite refusal"); !errors.Is(err, ErrRejected) {
		t.Fatal("released an observed process", err)
	}
	if _, err := s.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatal("session owner lost", err)
	}
}

func finishTestTurn(t *testing.T, s *Store, run string) StreamMessage {
	t.Helper()
	if _, err := s.PrepareStreamSend(run); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StreamRead(run); err == nil {
		t.Fatal("MCP opened before identity")
	}
	if err := s.ConfirmStreamIdentity(run, "test-id"); err != nil {
		t.Fatal(err)
	}
	m, err := s.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StreamReply(run, m.Receipt, "answer"); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteStreamTurn(run, true); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStreamTwoTurnsRetainOwnerAndRevokeOldReceipt(t *testing.T) {
	s, run := streamTestStore(t)
	old := finishTestTurn(t, s, run)
	if _, err := s.StreamRead(run); err == nil {
		t.Fatal("MCP open while idle")
	}
	if _, err := s.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatal("owner unlocked", err)
	}
	enqueue(t, s, "worker", "m2")
	if err := s.Pause(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.NextStreamTurn(run); !errors.Is(err, ErrPaused) {
		t.Fatal("paused stream advanced", err)
	}
	if err := s.ResumeWork(); err != nil {
		t.Fatal(err)
	}
	next, err := s.NextStreamTurn(run)
	if err != nil {
		t.Fatal(err)
	}
	if next == run {
		t.Fatal("attempt reused")
	}
	if _, err := s.PrepareStreamSend(run); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareStreamSend(run); err == nil {
		t.Fatal("duplicate send intent accepted")
	}
	if err := s.ConfirmStreamIdentity(run, "test-id"); err != nil {
		t.Fatal(err)
	}
	m, err := s.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StreamReply(run, old.Receipt, "late"); err == nil {
		t.Fatal("old receipt reached new turn")
	}
	reply, err := s.StreamReply(run, m.Receipt, "answer two")
	if err != nil {
		t.Fatal(err)
	}
	if reply.ReplyTo != "m2" {
		t.Fatal("wrong route", reply)
	}
	if err := s.CompleteStreamTurn(run, true); err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Replies) != 2 || len(snap.Attempts) != 2 || snap.StreamRuns[0]["state"] != "idle" {
		t.Fatal("incorrect state", snap)
	}
	for _, v := range snap.Verifications {
		if v["provider_ok"] != int64(1) || v["exit_observed"] != int64(0) || v["exit_code"] != nil {
			t.Fatal("turn invented process exit", v)
		}
	}
}

func TestStreamRestartDoesNotReplayAndKeepsCompletedHistory(t *testing.T) {
	for _, unfinished := range []bool{false, true} {
		s, run := streamTestStore(t)
		finishTestTurn(t, s, run)
		enqueue(t, s, "worker", "m2")
		if unfinished {
			if _, err := s.NextStreamTurn(run); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PrepareStreamSend(run); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.RevokeInherited(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.StreamRead(run); err == nil {
			t.Fatal("old capability works after restart")
		}
		if _, err := s.NextStreamTurn(run); err == nil {
			t.Fatal("stream restarted")
		}
		if _, err := s.Claim("worker"); !errors.Is(err, ErrBusy) {
			t.Fatal("second CLI became possible", err)
		}
		snap, err := s.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if snap.Attempts[0]["state"] != "turn_completed" || snap.Requests[0]["state"] != "completed" {
			t.Fatal("completed history lost")
		}
		expected := "queued"
		if unfinished {
			expected = "uncertain"
		}
		if snap.Requests[1]["state"] != expected {
			t.Fatal("restart silently retried message")
		}
		if _, err := s.RecoverFailedAttempt("claude", snap.Sessions[0]["environment"].(string), "test-id"); !errors.Is(err, ErrRecoveryBlocked) {
			t.Fatal("one-shot recovery bypassed stream owner", err)
		}
	}
}

func TestStreamStopRevokesReplyBeforeAnyExit(t *testing.T) {
	s, run := streamTestStore(t)
	if _, err := s.PrepareStreamSend(run); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStreamIdentity(run, "wrong"); err == nil {
		t.Fatal("wrong init accepted")
	}
	if err := s.ConfirmStreamIdentity(run, "test-id"); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStreamIdentity(run, "test-id"); err == nil {
		t.Fatal("duplicate init within one turn accepted")
	}
	m, err := s.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StopStream(run, "stop test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StreamReply(run, m.Receipt, "late"); err == nil {
		t.Fatal("reply accepted after stop")
	}
	if err := s.CompleteStreamTurn(run, true); err == nil {
		t.Fatal("stopped turn completed")
	}
	if err := s.RecordStreamExit(run, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatal("exit 0 unlocked owner")
	}
}

func TestStreamCannotCompleteWithoutReplyOrProviderSuccess(t *testing.T) {
	s, run := streamTestStore(t)
	if _, err := s.PrepareStreamSend(run); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStreamIdentity(run, "test-id"); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteStreamTurn(run, true); err == nil {
		t.Fatal("missing reply accepted")
	}
	m, err := s.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StreamReply(run, m.Receipt, "answer"); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteStreamTurn(run, false); !errors.Is(err, ErrNotVerified) {
		t.Fatal("bad result accepted")
	}
	if _, err := s.NextStreamTurn(run); err == nil {
		t.Fatal("next turn despite invalid result")
	}
}
