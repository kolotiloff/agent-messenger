package supervisor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func seedRecoverableStream(t *testing.T, complete bool) (string, *core.Store, recordedRun) {
	t.Helper()
	root, st := seedStreamClient(t, "resume-only")
	if err := st.SetNewClaudeSession("worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	run, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(run); err != nil {
		t.Fatal(err)
	}
	if err := st.BeginStream(run); err != nil {
		t.Fatal(err)
	}
	obs := platform.Observation{PID: 123456789, PGID: 123456789, StartToken: "synthetic-start", BootID: "legacy-observation-boot"}
	if err := st.RecordProcess(run, obs); err != nil {
		t.Fatal(err)
	}
	if complete {
		if _, err := st.PrepareStreamSend(run); err != nil {
			t.Fatal(err)
		}
		if err := st.ConfirmStreamIdentity(run, "test-stream"); err != nil {
			t.Fatal(err)
		}
		msg, err := st.StreamRead(run)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.StreamReply(run, msg.Receipt, "ACK:one"); err != nil {
			t.Fatal(err)
		}
		if err := st.CompleteStreamTurn(run, true); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Enqueue("m2", "worker", "coordinator", "uncertain-two"); err != nil {
			t.Fatal(err)
		}
		if _, err := st.NextStreamTurn(run); err != nil {
			t.Fatal(err)
		}
		if _, err := st.PrepareStreamSend(run); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.StopStream(run, "synthetic stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m3", "worker", "coordinator", "three"); err != nil {
		t.Fatal(err)
	}
	target, err := st.StreamRecoveryTarget("worker", run)
	if err != nil {
		t.Fatal(err)
	}
	r := recordedRun{ID: "test-reservation", Root: root, StreamRunID: run, Provider: target.Provider, Environment: target.Environment, ConversationID: target.ConversationID, PID: obs.PID, PGID: obs.PGID, StartToken: obs.StartToken, BootID: obs.BootID, Epoch: platform.RecoveryEpoch{Version: 1, Host: strings.Repeat("a", 64), Boot: "11111111-1111-1111-1111-111111111111"}}
	if err := saveRuns([]recordedRun{r}); err != nil {
		t.Fatal(err)
	}
	previous := recoveryEpoch
	recoveryEpoch = func() (platform.RecoveryEpoch, error) {
		e := r.Epoch
		e.Boot = "22222222-2222-2222-2222-222222222222"
		return e, nil
	}
	t.Cleanup(func() { recoveryEpoch = previous })
	return root, st, r
}

func TestStreamRecoveryResumeNoReplay(t *testing.T) {
	root, st, r := seedRecoverableStream(t, true)
	if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Paused || len(snap.StreamRuns) != 0 || len(snap.ArchivedStreamRuns) != 1 || len(snap.Attempts) != 2 || len(snap.Replies) != 1 || snap.Sessions[0]["active_token"] != nil || snap.Sessions[0]["start_mode"] != "resume" {
		t.Fatal("bad recovered state", snap)
	}
	if snap.Requests[0]["state"] != "completed" || snap.Requests[1]["state"] != "uncertain" || snap.Requests[2]["state"] != "queued" {
		t.Fatal("history rewritten", snap.Requests)
	}
	if _, err := st.StreamRead(r.StreamRunID); err == nil {
		t.Fatal("old MCP reopened")
	}
	if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	after, _ := st.Snapshot()
	if !reflect.DeepEqual(snap, after) {
		t.Fatal("repeated recovery changed history")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{AllowModels: true, ClaudeStream: true}) }()
	defer func() { cancel(); <-done }()
	waitSocket(t, root)
	response, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !response.OK || !response.Status.Paused {
		t.Fatal("recovery did not retain pause", response, err)
	}
	response, err = control.Call(root, control.Request{Cmd: "resume"})
	if err != nil || !response.OK {
		t.Fatal(response, err)
	}
	snap = waitStreamState(t, st, func(s core.Snapshot) bool {
		return len(s.Replies) == 2 && len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "idle"
	})
	if len(snap.Attempts) != 3 || snap.Replies[1]["reply_to"] != "m3" || snap.Requests[1]["state"] != "uncertain" {
		t.Fatal("unexpected replay", snap)
	}
	runs, err := loadRuns()
	if err != nil || len(runs) != 1 || runs[0].StreamRunID != snap.StreamRuns[0]["id"] {
		t.Fatal("new execution has no exact registry binding", err)
	}
}

func TestRecoverStreamHonorsSupervisorAndStartupLocks(t *testing.T) {
	root, st, r := seedRecoverableStream(t, true)
	before := readRunsFile(t)
	snap, _ := st.Snapshot()
	path, err := platform.SupervisorLockFile()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, path + ".startup"} {
		lock, err := platform.Acquire(name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation)
		lock.Release()
		if !errors.Is(err, platform.ErrAlreadyRunning) {
			t.Fatal("recovery bypassed owner", err)
		}
	}
	after, _ := st.Snapshot()
	if before != readRunsFile(t) || !reflect.DeepEqual(snap, after) {
		t.Fatal("locked recovery changed state")
	}
}

func TestRecoverStreamRefusesUnprovenOrMismatchedState(t *testing.T) {
	for _, name := range []string{"same-boot", "other-host", "legacy", "incomplete", "mismatched", "duplicate", "epoch-error", "bad-confirmation", "wrong-run", "corrupt", "no-completed-turn"} {
		t.Run(name, func(t *testing.T) {
			root, st, r := seedRecoverableStream(t, name != "no-completed-turn")
			confirmation := core.StreamRecoveryConfirmation
			id := r.StreamRunID
			switch name {
			case "same-boot":
				recoveryEpoch = func() (platform.RecoveryEpoch, error) { return r.Epoch, nil }
			case "other-host":
				recoveryEpoch = func() (platform.RecoveryEpoch, error) {
					e := r.Epoch
					e.Host = strings.Repeat("b", 64)
					e.Boot = "22222222-2222-2222-2222-222222222222"
					return e, nil
				}
			case "legacy":
				r.Epoch = platform.RecoveryEpoch{}
			case "incomplete":
				r.PID = 0
			case "mismatched":
				r.StartToken = "wrong"
			case "epoch-error":
				recoveryEpoch = func() (platform.RecoveryEpoch, error) {
					return platform.RecoveryEpoch{}, errors.New("observation unavailable")
				}
			case "bad-confirmation":
				confirmation = "yes"
			case "wrong-run":
				id = "wrong-run"
			}
			runs := []recordedRun{r}
			if name == "duplicate" {
				runs = append(runs, r)
			}
			if err := saveRuns(runs); err != nil {
				t.Fatal(err)
			}
			if name == "corrupt" {
				p, _ := runsPath()
				if err := os.WriteFile(p, []byte{}, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := readRunsFile(t)
			snap, _ := st.Snapshot()
			if _, err := RecoverStream(root, "worker", id, confirmation); err == nil {
				t.Fatal("unsafe recovery accepted")
			}
			after, _ := st.Snapshot()
			if readRunsFile(t) != before || !reflect.DeepEqual(snap, after) {
				t.Fatal("refusal mutated state")
			}
		})
	}
}

func TestRecoverStreamRetriesInterruptedRegistryCommit(t *testing.T) {
	for _, removed := range []bool{false, true} {
		t.Run(fmt.Sprint(removed), func(t *testing.T) {
			root, st, r := seedRecoverableStream(t, true)
			unrelated := recordedRun{ID: "other", Root: t.TempDir(), Provider: "codex", Environment: "other", ConversationID: "other"}
			if err := saveRuns([]recordedRun{r, unrelated}); err != nil {
				t.Fatal(err)
			}
			if removed {
				raw, _ := json.Marshal(r)
				fingerprint := fmt.Sprintf("%x", sha256.Sum256(raw))
				current, _ := recoveryEpoch()
				proof, _ := platform.ProvePreviousBoot(r.Epoch, current)
				if err := st.PrepareStreamRecovery("worker", r.StreamRunID, fingerprint, string(raw), proof); err != nil {
					t.Fatal(err)
				}
				if err := saveRuns([]recordedRun{unrelated}); err != nil {
					t.Fatal(err)
				}
			} else {
				beforeRenameHook = func() error { return errors.New("injected disk failure") }
				_, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation)
				beforeRenameHook = nil
				if err == nil {
					t.Fatal("registry error ignored")
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := Run(ctx, root, DisabledLauncher{}); !errors.Is(err, core.ErrRecoveryBlocked) {
				t.Fatal("pending recovery did not block serve", err)
			}
			if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
				t.Fatal(err)
			}
			runs, err := loadRuns()
			if err != nil || len(runs) != 1 || runs[0].ID != "other" {
				t.Fatal("unrelated reservation changed", runs, err)
			}
			snap, _ := st.Snapshot()
			if len(snap.ArchivedStreamRuns) != 1 || len(snap.StreamRecoveries) != 1 || snap.StreamRecoveries[0]["phase"] != "done" || len(snap.Attempts) != 2 {
				t.Fatal("recovery duplicated history", snap)
			}
		})
	}
}
