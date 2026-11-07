//go:build windows

package selfupdate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// The program being replaced is running (the window, or a background client
// started from it). Windows keeps it as a leftover until it exits; a second
// update while it still runs uses another name.
func TestWindowsReplacesARunningProgram(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.exe, f.program, 0o755); err != nil {
		t.Fatal(err)
	}
	running := exec.Command(f.exe)
	running.Env = append(os.Environ(), "SELFUPDATE_SLEEP=1")
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = running.Process.Kill(); _ = running.Wait() })

	m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
	f.reports(m.Version, revision)
	if s, err := f.updater(32).Apply(context.Background()); err != nil || s.State != "installed" {
		t.Fatalf("first update %+v %v", s, err)
	}
	RemoveLeftover(f.exe)
	if _, err := os.Stat(f.exe + ".old"); err != nil {
		t.Fatalf("the running program's file is gone: %v", err)
	}
	m = f.publish(34, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
	f.reports(m.Version, revision)
	if s, err := f.updater(33).Apply(context.Background()); err != nil || s.State != "installed" {
		t.Fatalf("second update while the first program runs %+v %v", s, err)
	}
	if _, err := os.Stat(f.exe + ".old-1"); err != nil {
		t.Fatalf("no second leftover: %v", err)
	}
	_ = running.Process.Kill()
	_ = running.Wait()
	RemoveLeftover(f.exe)
	for _, name := range []string{f.exe + ".old", f.exe + ".old-1"} {
		if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s was not removed: %v", name, err)
		}
	}
}
