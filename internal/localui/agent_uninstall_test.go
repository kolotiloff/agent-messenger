package localui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The removal script asks first, removes the native install's launcher and
// versions folder, and leaves the sign-in folder alone.
func TestUninstallScriptRemovesOnlyTheProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the PowerShell variant is not run here")
	}
	home := t.TempDir()
	versions := filepath.Join(home, ".local", "share", "claude", "versions", "1.0")
	bin := filepath.Join(home, ".local", "bin")
	profile := filepath.Join(home, ".claude")
	for _, d := range []string{versions, bin, profile} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(versions, "claude"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(versions, "claude"), filepath.Join(bin, "claude")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script, argv := uninstallScript(runtime.GOOS, "claude", dir)
	if err := os.WriteFile(filepath.Join(dir, "uninstall.sh"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(answer string) string {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		cmd.Stdin = strings.NewReader(answer + "\n")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return string(out)
	}
	if out := run("n"); !strings.Contains(out, "ничего не удалено") {
		t.Fatalf("refusal: %s", out)
	}
	if _, err := os.Lstat(filepath.Join(bin, "claude")); err != nil {
		t.Fatal("removed after a refusal")
	}
	if out := run("y"); !strings.Contains(out, "CLI удалён") {
		t.Fatalf("removal: %s", out)
	}
	if _, err := os.Lstat(filepath.Join(bin, "claude")); !os.IsNotExist(err) {
		t.Fatal("launcher kept")
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "claude")); !os.IsNotExist(err) {
		t.Fatal("versions folder kept")
	}
	if _, err := os.Stat(filepath.Join(profile, "settings.json")); err != nil {
		t.Fatal("sign-in folder touched")
	}
	if out := run("y"); !strings.Contains(out, "удалять нечего") {
		t.Fatalf("second run: %s", out)
	}
}

// The Windows script never opens a web page and asks before removing.
func TestUninstallScriptWindowsAsks(t *testing.T) {
	for _, p := range []string{"claude", "codex", "cursor"} {
		script, _ := uninstallScript("windows", p, `C:\x`)
		if !strings.Contains(script, "Read-Host") || strings.Contains(script, "Start-Process") || strings.Contains(script, "http") {
			t.Fatalf("%s: %s", p, script)
		}
	}
}
