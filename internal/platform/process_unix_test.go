//go:build unix

package platform

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

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
