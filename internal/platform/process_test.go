package platform

import (
	"errors"
	"testing"
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
