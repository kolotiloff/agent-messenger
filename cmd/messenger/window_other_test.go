//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

func TestAppWindowCommandPassesOnlyTheFile(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "chromium"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cmd := appWindowCommand("/data/window-profile-1", "/data/window-1.html")
	if cmd == nil {
		t.Fatal("no app window")
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--app=file:///data/window-1.html") || !strings.Contains(args, "--user-data-dir=/data/window-profile-1") || strings.Contains(args, "#") {
		t.Fatal(args)
	}
	t.Setenv("PATH", t.TempDir())
	if appWindowCommand("/data", "/data/window-1.html") != nil {
		t.Fatal("app window without a browser")
	}
}

func TestMenuEntry(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	installMenuEntry()
	raw, err := os.ReadFile(filepath.Join(data, "applications", "agent-messenger.desktop"))
	exe, _ := os.Executable()
	if err != nil || !strings.Contains(string(raw), "Exec=\""+exe+"\" open") {
		t.Fatal(string(raw), err)
	}
	// The entry has the program's icon (T-045).
	icon := filepath.Join(data, "icons", "hicolor", "scalable", "apps", "agent-messenger.svg")
	if svg, err := os.ReadFile(icon); err != nil || !strings.HasPrefix(string(svg), "<svg") || !strings.Contains(string(raw), "Icon="+icon+"\n") {
		t.Fatal(string(raw), err)
	}
}

// Owner, 2026-11-12: after an update made outside the window the menu
// started the old version. An installation's entry starts its bin link.
func TestMenuEntryOfInstallationStartsTheLink(t *testing.T) {
	prefix := t.TempDir()
	version := filepath.Join(prefix, "lib", "agent-messenger", "0.62-abc")
	if err := os.MkdirAll(version, 0o700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(version, "messenger")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := menuProgram(program); got != program {
		t.Fatalf("without a link: %s", got)
	}
	if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(prefix, "bin", "messenger")
	if err := os.Symlink(program, link); err != nil {
		t.Fatal(err)
	}
	if got := menuProgram(program); got != link {
		t.Fatalf("installation: %s", got)
	}
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	menuEntryAfterInstall(link)
	raw, err := os.ReadFile(filepath.Join(data, "applications", "agent-messenger.desktop"))
	if err != nil || !strings.Contains(string(raw), "Exec=\""+link+"\" open") {
		t.Fatal(string(raw), err)
	}
	plain := filepath.Join(t.TempDir(), "AgentMessenger-Linux-x64")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := menuProgram(plain); got != plain {
		t.Fatalf("plain file: %s", got)
	}
}

// After an update from another computer the new client fixes an existing
// entry; it never creates one, and only for the desktop's data folder.
func TestClientRefreshesExistingMenuEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	root, err := platform.DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(home, ".local")
	version := filepath.Join(prefix, "lib", "agent-messenger", "0.62-abc")
	program := filepath.Join(version, "messenger")
	link := filepath.Join(prefix, "bin", "messenger")
	for _, d := range []string{version, filepath.Dir(link), root} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(program, link); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.WriteProgramPath(root, program); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(home, "data", "applications", "agent-messenger.desktop")
	refreshMenuEntry(root)
	if _, err := os.Stat(entry); err == nil {
		t.Fatal("the client created a menu entry")
	}
	writeMenuEntry(program)
	refreshMenuEntry(filepath.Join(home, "other"))
	if raw, _ := os.ReadFile(entry); !strings.Contains(string(raw), "Exec=\""+program+"\" open") {
		t.Fatal("another data folder changed the entry: " + string(raw))
	}
	refreshMenuEntry(root)
	if raw, _ := os.ReadFile(entry); !strings.Contains(string(raw), "Exec=\""+link+"\" open") {
		t.Fatal("entry not refreshed: " + string(raw))
	}
}
