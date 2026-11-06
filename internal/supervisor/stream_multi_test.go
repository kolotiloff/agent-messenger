//go:build darwin || linux

package supervisor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// Two persistent Claude sessions are served at the same time: a live stream
// for one session does not starve the other.
func TestTwoClaudeStreamsRunSideBySide(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	env := mkdir(t, filepath.Join(root, "profile-2"))
	if _, err := st.Register("claude", env, "test-stream-2", "worker-2"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("worker-2", exe, root); err != nil {
		t.Fatal(err)
	}
	for alias, id := range map[string]string{"worker": "m1", "worker-2": "m2"} {
		if _, err := st.Enqueue(id, alias, "coordinator", id); err != nil {
			t.Fatal(err)
		}
	}
	_, done := startStreamClient(t, root, true)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool {
		if len(s.Replies) != 2 || len(s.StreamRuns) != 2 {
			return false
		}
		for _, run := range s.StreamRuns {
			if run["state"] != "idle" {
				return false
			}
		}
		return true
	})
	var pids []int
	for _, run := range snap.StreamRuns {
		obs, err := st.Observation(text(run["id"]))
		if err != nil {
			t.Fatal(err)
		}
		if owns, err := platform.OwnsObservation(obs); err != nil || !owns {
			t.Fatal("stream not alive between turns")
		}
		pids = append(pids, obs.PID)
	}
	if pids[0] == pids[1] {
		t.Fatal("both sessions share one process")
	}
	// A second message to each session goes to the same process.
	for alias, id := range map[string]string{"worker": "m3", "worker-2": "m4"} {
		if _, err := st.Enqueue(id, alias, "coordinator", id); err != nil {
			t.Fatal(err)
		}
	}
	snap = waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.Replies) == 4 })
	if len(snap.StreamRuns) != 2 {
		t.Fatalf("stream runs %d", len(snap.StreamRuns))
	}
	// stop-job stops every live stream, not only one.
	streamControl(t, root, "stop-job")
	waitStreamState(t, st, func(s core.Snapshot) bool {
		return s.StreamRuns[0]["state"] == "uncertain" && s.StreamRuns[1]["state"] == "uncertain"
	})
	streamControl(t, root, "shutdown")
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked")
	}
}
