//go:build windows

package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The test binary doubles as the processes of a run: "parent" starts a
// "sleeper" child, writes its PID and sleeps; "sleeper" only sleeps.
func TestMain(m *testing.M) {
	switch os.Getenv("PLATFORM_TEST_ROLE") {
	case "parent":
		self, _ := os.Executable()
		child := exec.Command(self)
		child.Env = append(os.Environ(), "PLATFORM_TEST_ROLE=sleeper")
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		_ = os.WriteFile(os.Getenv("PLATFORM_TEST_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "sleeper":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	r, _ := windows.WaitForSingleObject(h, 0)
	return r == uint32(windows.WAIT_TIMEOUT)
}

// A run's job keeps its descendants after the first process exits, and the
// job's stop ends them; only then is the group empty.
func TestWindowsJobHoldsDescendantsUntilStopped(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(), "PLATFORM_TEST_ROLE=parent", "PLATFORM_TEST_PIDFILE="+pidFile)
	if err := SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	if err := StartGroup(cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SignalGroup(cmd.Process.Pid) })
	obs, err := ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if obs.PGID != cmd.Process.Pid || obs.StartToken == "" || obs.BootID == "" {
		t.Fatalf("observation %+v", obs)
	}
	if owned, err := OwnsObservation(obs); err != nil || !owned {
		t.Fatalf("owned=%t %v", owned, err)
	}
	stale := obs
	stale.StartToken = "1"
	if owned, err := OwnsObservation(stale); err != nil || owned {
		t.Fatalf("a different start token was accepted: %t %v", owned, err)
	}
	var childPID int
	waitFor(t, "the child's pid", func() bool {
		raw, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		childPID, err = strconv.Atoi(string(raw))
		return err == nil
	})
	if empty, err := GroupEmpty(obs.PGID); err != nil || empty {
		t.Fatalf("live job empty=%t %v", empty, err)
	}
	// The first process exits; its child stays in the job.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if !alive(childPID) {
		t.Fatal("the child ended with its parent")
	}
	if empty, err := GroupEmpty(obs.PGID); err != nil || empty {
		t.Fatalf("job with a live child empty=%t %v", empty, err)
	}
	if err := SignalGroup(obs.PGID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "an empty job", func() bool {
		empty, err := GroupEmpty(obs.PGID)
		return err == nil && empty
	})
	if alive(childPID) {
		t.Fatal("the child survived the job's stop")
	}
	// An empty job is not a proof by itself.
	if proof, err := CurrentProof(obs); proof.Proven || !errors.Is(err, ErrNotProven) {
		t.Fatalf("absence was treated as proof: %+v %v", proof, err)
	}
	// A job this client saw empty has nothing to stop.
	if err := SignalGroup(obs.PGID); err != nil {
		t.Fatal(err)
	}
}

// A job name still held by an earlier run is never shared.
func TestWindowsJobNameOfAnEarlierRunIsRefused(t *testing.T) {
	pid := 4_000_000 + os.Getpid()
	name, _ := windows.UTF16PtrFromString(jobName(pid))
	held, err := windows.CreateJobObject(nil, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := joinJob(pid); err == nil {
		windows.CloseHandle(held)
		t.Fatal("an existing job was reused")
	}
	if empty, err := GroupEmpty(pid); err != nil || !empty {
		windows.CloseHandle(held)
		t.Fatalf("a job without processes empty=%t %v", empty, err)
	}
	// Once no handle is open the name is gone. A job this client does not
	// hold and cannot open is never taken as empty.
	windows.CloseHandle(held)
	if empty, err := GroupEmpty(pid); empty || !errors.Is(err, ErrNotProven) {
		t.Fatalf("an unreachable job empty=%t %v", empty, err)
	}
	if err := SignalGroup(pid); !errors.Is(err, ErrNotProven) {
		t.Fatalf("an unreachable job was taken as stopped: %v", err)
	}
}

// A command that was never prepared cannot be started in a group, and a
// missing program is a definite "never ran".
func TestWindowsStartGroupNeedsPreparation(t *testing.T) {
	if err := StartGroup(exec.Command("cmd.exe")); err == nil {
		t.Fatal("unprepared command started")
	}
	cmd := exec.Command(filepath.Join(t.TempDir(), "missing.exe"))
	if err := SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	err := StartGroup(cmd)
	if err == nil || !NeverRan(cmd, err) {
		t.Fatalf("missing program: %v never ran=%t", err, NeverRan(cmd, err))
	}
}

func TestWindowsBootIdentityIsStable(t *testing.T) {
	first, err := bootID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := bootID()
	if err != nil || second != first {
		t.Fatalf("boot identity %q then %q %v", first, second, err)
	}
	t.Log("boot identity:", first)
}

func TestWindowsExecutableNeedsExe(t *testing.T) {
	dir := t.TempDir()
	for name, want := range map[string]bool{"tool.exe": true, "TOOL.EXE": true, "tool.cmd": false, "tool.bat": false, "tool": false} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := IsExecutable(path, info); got != want {
			t.Fatalf("%s: %t", name, got)
		}
		os.Remove(path)
	}
}
