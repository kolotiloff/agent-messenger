package supervisor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func streamFixture(mode string) error {
	if mode == "resume-only" {
		resume, fresh := false, false
		for _, a := range os.Args {
			resume = resume || a == "--resume"
			fresh = fresh || a == "--session-id"
		}
		if !resume || fresh {
			return errors.New("recovered session must use exact resume")
		}
	}
	var sid, config string
	for i, a := range os.Args {
		if i+1 < len(os.Args) {
			switch a {
			case "--resume", "--session-id":
				sid = os.Args[i+1]
			case "--mcp-config":
				config = os.Args[i+1]
			}
		}
	}
	var c struct {
		Servers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(config), &c); err != nil {
		return err
	}
	server := c.Servers["messenger_bound"]
	call := func(name string, args map[string]any) (map[string]any, error) {
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		req, err := http.NewRequest("POST", server.URL, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", server.Headers["Authorization"])
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		var rpc struct {
			Result struct {
				Error   bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.NewDecoder(response.Body).Decode(&rpc); err != nil {
			return nil, err
		}
		if rpc.Result.Error || len(rpc.Result.Content) != 1 {
			return nil, errors.New("MCP call rejected")
		}
		var value map[string]any
		err = json.Unmarshal([]byte(rpc.Result.Content[0].Text), &value)
		return value, err
	}
	encoder := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			return err
		}
		if frame["session_id"] != sid {
			return errors.New("wrong input session")
		}
		id := sid
		if mode == "wrong-id" {
			id = "different"
		}
		if err := encoder.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": id}); err != nil {
			return err
		}
		// Simulates provider latency before tools; no language model is involved.
		time.Sleep(60 * time.Millisecond)
		if mode == "hold" {
			for {
				time.Sleep(time.Hour)
			}
		}
		// stdout identity and HTTP tools travel independently. A fixed provider
		// delay cannot prove the gate is open on a loaded race-test runner.
		// Only the idempotent read may wait/retry; reply is sent once.
		current, err := call("read_current", map[string]any{})
		deadline := time.Now().Add(2 * time.Second)
		for err != nil && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
			current, err = call("read_current", map[string]any{})
		}
		if err != nil {
			return err
		}
		if _, err := call("reply", map[string]any{"receipt": current["receipt"], "text": "ACK:" + current["text"].(string)}); err != nil {
			return err
		}
		if err := encoder.Encode(map[string]any{"type": "assistant", "session_id": sid, "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "name": "mcp__messenger_bound__read_current"}, map[string]any{"type": "tool_use", "name": "mcp__messenger_bound__reply"}}}}); err != nil {
			return err
		}
		success := "success"
		if mode == "bad-result" {
			success = "error_during_execution"
		}
		if err := encoder.Encode(map[string]any{"type": "result", "session_id": sid, "subtype": success, "is_error": mode == "bad-result"}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func seedStreamClient(t *testing.T, mode string) (string, *core.Store) {
	t.Helper()
	isolateUser(t)
	t.Setenv("MESSENGER_STREAM_FIXTURE", mode)
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "profile"))
	st := mustStore(t, root)
	if _, err := st.Register("claude", env, "test-stream", "worker"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("worker", exe, root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		snap, err := st.Snapshot()
		if err == nil {
			for _, run := range snap.StreamRuns {
				obs, err := st.Observation(text(run["id"]))
				if err == nil {
					if owns, _ := platform.OwnsObservation(obs); owns {
						p, _ := os.FindProcess(obs.PID)
						_ = p.Kill()
					}
				}
			}
		}
		st.Close()
	})
	return root, st
}

func waitStreamState(t *testing.T, st *core.Store, condition func(core.Snapshot) bool) core.Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := st.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		if condition(snap) {
			return snap
		}
		time.Sleep(10 * time.Millisecond)
	}
	snap, _ := st.Snapshot()
	t.Fatalf("stream state timeout: %+v", snap)
	return snap
}

func startStreamClient(t *testing.T, root string, allow bool) (context.CancelFunc, chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{AllowModels: allow, ClaudeStream: true}) }()
	waitSocket(t, root)
	t.Cleanup(cancel)
	return cancel, done
}

func streamControl(t *testing.T, root, command string) {
	t.Helper()
	response, err := control.Call(root, control.Request{Cmd: command})
	if err != nil || !response.OK {
		t.Fatalf("%s: %+v %v", command, response, err)
	}
}

func TestStreamServeTwoQueuedMessagesPauseStopAndRestart(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	if err := st.Pause(); err != nil {
		t.Fatal(err)
	}
	_, done := startStreamClient(t, root, true)
	time.Sleep(80 * time.Millisecond)
	snap, _ := st.Snapshot()
	if len(snap.Attempts) != 0 {
		t.Fatal("paused start launched CLI")
	}
	streamControl(t, root, "resume")
	first := waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "idle" })
	run := text(first.StreamRuns[0]["id"])
	obs, err := st.Observation(run)
	if err != nil {
		t.Fatal(err)
	}
	streamControl(t, root, "pause")
	if _, err := st.Enqueue("m2", "worker", "coordinator", "two"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	snap, _ = st.Snapshot()
	if len(snap.Attempts) != 1 {
		t.Fatal("paused stream sent next turn")
	}
	streamControl(t, root, "resume")
	second := waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.Replies) == 2 && s.StreamRuns[0]["state"] == "idle" })
	if second.Attempts[0]["pid"] != second.Attempts[1]["pid"] {
		t.Fatal("second CLI launched")
	}
	if owns, err := platform.OwnsObservation(obs); err != nil || !owns {
		t.Fatal("stream not alive between turns", err)
	}
	streamControl(t, root, "pause")
	if _, err := st.Enqueue("m3", "worker", "coordinator", "three"); err != nil {
		t.Fatal(err)
	}
	streamControl(t, root, "stop-job")
	waitStreamState(t, st, func(s core.Snapshot) bool { return s.StreamRuns[0]["state"] == "uncertain" })
	streamControl(t, root, "shutdown")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
	_, again := startStreamClient(t, root, true)
	streamControl(t, root, "resume")
	time.Sleep(150 * time.Millisecond)
	snap, _ = st.Snapshot()
	if len(snap.Attempts) != 2 || snap.Requests[2]["state"] != "queued" {
		t.Fatal("restart replayed or relaunched held session")
	}
	for _, v := range snap.Verifications {
		if v["provider_ok"] != int64(1) || v["exit_observed"] != int64(0) {
			t.Fatal("turn verification lost", v)
		}
	}
	streamControl(t, root, "shutdown")
	select {
	case <-again:
	case <-time.After(2 * time.Second):
		t.Fatal("second shutdown blocked")
	}
}

func TestStreamShutdownRevokesActiveTurnWithoutWaitingForHungCLI(t *testing.T) {
	root, st := seedStreamClient(t, "hold")
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	_, done := startStreamClient(t, root, true)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "active" })
	run := text(snap.StreamRuns[0]["id"])
	started := time.Now()
	streamControl(t, root, "shutdown")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for CLI")
	}
	if time.Since(started) > time.Second {
		t.Fatal("shutdown too slow")
	}
	if _, err := st.StreamRead(run); err == nil {
		t.Fatal("MCP remained open")
	}
	obs, err := st.Observation(run)
	if err != nil {
		t.Fatal(err)
	}
	// Fixture intentionally remains alive. The reserve must still prevent a new root.
	if owns, err := platform.OwnsObservation(obs); err != nil || !owns {
		t.Fatal("fixture exited unexpectedly", err)
	}
	if err := reconcileRuns(t.TempDir()); !errors.Is(err, ErrLiveCLI) {
		t.Fatal("live stream bypassed root guard", err)
	}
	_, again := startStreamClient(t, root, true)
	streamControl(t, root, "stop-job") // finds the original live stream after restart
	streamControl(t, root, "shutdown")
	select {
	case <-again:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}

func TestStreamModelOptInAndProviderErrorKeepQueue(t *testing.T) {
	for _, mode := range []string{"normal", "bad-result", "wrong-id"} {
		t.Run(mode, func(t *testing.T) {
			root, st := seedStreamClient(t, mode)
			for i := 1; i <= 2; i++ {
				if _, err := st.Enqueue(fmt.Sprintf("m%d", i), "worker", "coordinator", "payload"); err != nil {
					t.Fatal(err)
				}
			}
			allow := mode != "normal"
			_, done := startStreamClient(t, root, allow)
			if !allow {
				time.Sleep(100 * time.Millisecond)
			} else {
				waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "uncertain" })
			}
			snap, _ := st.Snapshot()
			expected := 1
			if !allow {
				expected = 0
			}
			if len(snap.Attempts) != expected {
				t.Fatal("unexpected retries or launch", snap)
			}
			if snap.Requests[1]["state"] != "queued" {
				t.Fatal("second message consumed after failure")
			}
			streamControl(t, root, "shutdown")
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("shutdown blocked")
			}
		})
	}
}

func TestStreamAbruptSupervisorCrashRetainsOwnerAndDoesNotReplay(t *testing.T) {
	root, st := seedStreamClient(t, "hold")
	for _, id := range []string{"m1", "m2"} {
		if _, err := st.Enqueue(id, "worker", "coordinator", "payload"); err != nil {
			t.Fatal(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe)
	child.Env = append(os.Environ(), "MESSENGER_STREAM_SERVE_ROOT="+root)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill() })
	waitSocket(t, root)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "active" })
	run := text(snap.StreamRuns[0]["id"])
	obs, err := st.Observation(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	_, done := startStreamClient(t, root, true)
	waitStreamState(t, st, func(s core.Snapshot) bool { return s.StreamRuns[0]["state"] == "uncertain" })
	time.Sleep(100 * time.Millisecond)
	snap, _ = st.Snapshot()
	if len(snap.Attempts) != 1 || snap.Requests[0]["state"] != "uncertain" || snap.Requests[1]["state"] != "queued" {
		t.Fatal("crash caused replay", snap)
	}
	if _, err := st.StreamRead(run); err == nil {
		t.Fatal("old MCP capability usable after crash")
	}
	if owns, err := platform.OwnsObservation(obs); err != nil || !owns {
		t.Fatal("original CLI should still be alive", err)
	}
	streamControl(t, root, "stop-job")
	streamControl(t, root, "shutdown")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
}
