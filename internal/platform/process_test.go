package platform

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestParsePSFailsClosed(t *testing.T) {
	live, err := ParsePS("  10  10 Wed Nov  5 12:00:00 2026\n")
	if err != nil || len(live) != 1 || live[0].PGID != 10 {
		t.Fatalf("parse = %+v %v", live, err)
	}
	if _, err := ParsePS("not a process\n"); err == nil {
		t.Fatal("malformed ps was accepted")
	}
}

func TestEvaluateStop(t *testing.T) {
	stored := Observation{PID: 10, PGID: 10, StartToken: "100", BootID: "boot-a"}
	if proof, err := EvaluateStop(stored, nil, "boot-b", nil); err != nil || !proof.Proven {
		t.Fatalf("boot change = %+v %v", proof, err)
	}
	if _, err := EvaluateStop(stored, []Proc{{PID: 10, PGID: 10, StartToken: "100"}}, "boot-a", nil); !errors.Is(err, ErrStillRunning) {
		t.Fatalf("live group = %v", err)
	}
	if _, err := EvaluateStop(stored, nil, "boot-a", nil); !errors.Is(err, ErrNotProven) {
		t.Fatalf("empty group = %v", err)
	}
	if _, err := EvaluateStop(Observation{}, nil, "boot-a", nil); !errors.Is(err, ErrNotProven) {
		t.Fatalf("incomplete = %v", err)
	}
	if _, err := EvaluateStop(stored, nil, "", errors.New("snapshot failed")); !errors.Is(err, ErrNotProven) {
		t.Fatalf("snapshot error = %v", err)
	}
}

func TestOwnsObservationRejectsMismatchedStartToken(t *testing.T) {
	if _, err := ReadObservation(os.Getpid()); errors.Is(err, ErrUnsupported) {
		t.Skip(err.Error())
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := SetNewGroup(cmd); err != nil {
		if errors.Is(err, ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	obs, err := ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := OwnsObservation(obs)
	if err != nil || !owned {
		t.Fatalf("owner = %t %v", owned, err)
	}
	obs.StartToken = "not-the-recorded-start"
	owned, err = OwnsObservation(obs)
	if err != nil || owned {
		t.Fatalf("mismatched start token was accepted: %t %v", owned, err)
	}
}

func TestLinuxProcessGroupIsObservedAndNotTreatedAsDescendantProof(t *testing.T) {
	if _, err := ReadObservation(1); errors.Is(err, ErrUnsupported) {
		t.Skip("process observation is not implemented on this OS")
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := SetNewGroup(cmd); err != nil {
		if errors.Is(err, ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	obs, err := ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := GroupEmpty(obs.PGID)
	if err != nil || empty {
		t.Fatalf("live group empty=%t %v", empty, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for {
		empty, err = GroupEmpty(obs.PGID)
		if err == nil && empty {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("group remained: empty=%t %v", empty, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	proof, err := CurrentProof(obs)
	if proof.Proven || !errors.Is(err, ErrNotProven) {
		t.Fatalf("absence was treated as proof: %+v %v", proof, err)
	}
}

func TestParsePSTreeAndDescendants(t *testing.T) {
	live, err := ParsePSTree("  1 0 1 Wed Nov  5 12:00:00 2026\n 10 1 10 Wed Nov  5 12:00:01 2026\n 11 10 10 Wed Nov  5 12:00:02 2026\n 12 11 12 Wed Nov  5 12:00:03 2026\n 13 1 10 Wed Nov  5 12:00:04 2026\n 14 1 14 Wed Nov  5 12:00:05 2026\n")
	if err != nil || len(live) != 6 || live[2].PPID != 10 {
		t.Fatalf("%+v %v", live, err)
	}
	got := map[int]bool{}
	for _, p := range descendantsOf(Observation{PID: 10, PGID: 10}, live) {
		got[p.PID] = true
	}
	// 11 is a child, 12 a grandchild in its own group, 13 a reparented group member.
	if len(got) != 3 || !got[11] || !got[12] || !got[13] {
		t.Fatalf("descendants %v", got)
	}
	if _, err := ParsePSTree("10 x 10 Wed\n"); err == nil {
		t.Fatal("bad line accepted")
	}
}
