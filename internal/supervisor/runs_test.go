package supervisor

import (
	"context"
	"errors"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestNullRunRegistryMustNotAllowAnotherRoot(t *testing.T) {
	isolateUser(t)
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"null", " \n null\t"} {
		if err = os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err = loadRuns(); !errors.Is(err, ErrRunsCorrupt) {
			t.Fatalf("null registry accepted as empty: %v", err)
		}
		if err = reconcileRuns("/other-root"); !errors.Is(err, ErrLiveCLI) {
			t.Fatalf("another root allowed: %v", err)
		}
		if err = rememberRun(recordedRun{ID: "new", Root: "/new"}); err == nil {
			t.Fatal("damaged registry replaced")
		}
		if _, err = RecoverMissingRoots(core.RecoveryConfirmation); !errors.Is(err, ErrRunsCorrupt) {
			t.Fatalf("recovery accepted null: %v", err)
		}
		if got := ReadReservations(); got.Error == "" {
			t.Fatalf("damaged registry read as %+v", got)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = Run(ctx, t.TempDir(), AdapterLauncher{AllowTasks: true})
		cancel()
		if !errors.Is(err, ErrLiveCLI) {
			t.Fatalf("supervisor started: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != raw {
			t.Fatal("damaged registry was changed")
		}
	}
}

func TestEmptyArrayRunRegistryAllowsFirstStartup(t *testing.T) {
	isolateUser(t)
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(" [] "), 0o600); err != nil {
		t.Fatal(err)
	}
	runs, err := loadRuns()
	if err != nil || runs == nil || len(runs) != 0 {
		t.Fatalf("valid empty array rejected: %v", err)
	}
	if err = reconcileRuns("/first-root"); err != nil {
		t.Fatal(err)
	}
}

func TestRelativeOrMissingRegistryRootCannotMatchCurrentDirectory(t *testing.T) {
	isolateUser(t)
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`[{}]`, `[null]`, `[{"id":"old","root":"."}]`, `[{"id":"old","root":"data"}]`} {
		if err = os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if err = reconcileRuns("."); !errors.Is(err, ErrLiveCLI) {
			t.Fatalf("unidentified root matched current directory: %s: %v", raw, err)
		}
		if err = reconcileRuns("data"); !errors.Is(err, ErrLiveCLI) {
			t.Fatalf("relative root accepted in a different working directory: %s: %v", raw, err)
		}
	}
}
