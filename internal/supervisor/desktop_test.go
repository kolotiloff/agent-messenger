package supervisor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestDesktopRootPreservesReservations(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, dbName), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skip(err)
	}
	if err := saveRuns([]recordedRun{{Root: root}, {Root: alias}}); err != nil {
		t.Fatal(err)
	}
	path, _ := runsPath()
	before, _ := os.ReadFile(path)
	got, err := DesktopRoot()
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(root)
	if got != canonical {
		t.Fatalf("root %q, expected %q", got, canonical)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("desktop altered reservation")
	}
}
func TestDesktopRootRejectsMissingOrAmbiguousData(t *testing.T) {
	for _, mode := range []string{"missing", "multiple", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			isolateUser(t)
			root := t.TempDir()
			second := t.TempDir()
			for _, p := range []string{root, second} {
				if err := os.WriteFile(filepath.Join(p, dbName), []byte("test"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			records := []recordedRun{{Root: root}}
			if mode == "multiple" {
				records = append(records, recordedRun{Root: second})
			}
			if mode == "missing" {
				if err := os.Remove(filepath.Join(root, dbName)); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveRuns(records); err != nil {
				t.Fatal(err)
			}
			path, _ := runsPath()
			if mode == "corrupt" {
				if err := os.WriteFile(path, []byte("null"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(path)
			if _, err := DesktopRoot(); err == nil {
				t.Fatal("unsafe root was selected")
			} else if mode == "corrupt" && !errors.Is(err, ErrRunsCorrupt) {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("registry mutated")
			}
		})
	}
}
func TestDesktopRootWithoutReservationsUsesDefault(t *testing.T) {
	isolateUser(t)
	want, err := platform.DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DesktopRoot()
	if err != nil || got != want {
		t.Fatalf("%q %v", got, err)
	}
}
