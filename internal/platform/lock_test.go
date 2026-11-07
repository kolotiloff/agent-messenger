package platform

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSecondLockIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supervisor.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second lock = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Release()
}
