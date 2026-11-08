//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
}
