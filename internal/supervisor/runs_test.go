package supervisor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingRunFileAllowsFirstStartup(t *testing.T) {
	isolateUser(t)
	if err := reconcileRuns("/first-root"); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyOrCorruptRunFileBlocksStartup(t *testing.T) {
	isolateUser(t)
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileRuns("/other-root"); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("empty file = %v", err)
	}
	if _, err := loadRuns(); !errors.Is(err, ErrRunsCorrupt) {
		t.Fatalf("load empty = %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileRuns("/other-root"); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("corrupt file = %v", err)
	}
	if err := rememberRun(recordedRun{ID: "should-not-start", Root: "/old"}); err == nil {
		t.Fatal("rememberRun ignored a damaged registry")
	}
}

func TestInterruptedRunWriteKeepsPreviousReservation(t *testing.T) {
	isolateUser(t)
	if err := rememberRun(recordedRun{ID: "kept", Root: "/old-data", Provider: "codex", ConversationID: "conversation-1"}); err != nil {
		t.Fatal(err)
	}
	beforeRenameHook = func() error {
		return errors.New("interrupted before rename")
	}
	t.Cleanup(func() { beforeRenameHook = nil })
	if err := rememberRun(recordedRun{ID: "replacement", Root: "/other-data"}); err == nil {
		t.Fatal("interrupted write reported success")
	}
	runs, err := loadRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != "kept" || runs[0].Root != "/old-data" {
		t.Fatalf("previous reservation lost: %+v", runs)
	}
	if err := reconcileRuns("/new-data"); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("other root after interrupted write = %v", err)
	}
}
