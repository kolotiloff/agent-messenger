package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsInstallPlan(t *testing.T) {
	local, roaming := filepath.Join(string(filepath.Separator)+"u", "Local"), filepath.Join(string(filepath.Separator)+"u", "Roaming")
	if runtime.GOOS == "windows" {
		local, roaming = `C:\Users\Анна\AppData\Local`, `C:\Users\Анна\AppData\Roaming`
	}
	p, ok := winInstallPaths(local, roaming)
	if !ok || filepath.Base(p.Exe) != "AgentMessenger.exe" || filepath.Base(p.Dir) != "Agent Messenger" || !strings.HasSuffix(p.Shortcut, "Agent Messenger.lnk") {
		t.Fatalf("paths %+v", p)
	}
	if _, ok := winInstallPaths("relative", roaming); ok {
		t.Fatal("relative LOCALAPPDATA accepted")
	}
	download := filepath.Join(filepath.Dir(local), "Downloads", "AgentMessenger-Windows-x64.exe")
	for _, c := range []struct {
		running, version, installed string
		exists                      bool
		want                        string
	}{
		{download, "0.0.0-preview.48", "", false, "install"},
		{download, "0.0.0-preview.48", "0.0.0-preview.47", true, "install"},
		{download, "0.0.0-preview.48", "", true, "install"},
		{download, "0.0.0-preview.47", "0.0.0-preview.48", true, "open-installed"},
		{download, "0.0.0-preview.48", "0.0.0-preview.48", true, "open-installed"},
		{p.Exe, "0.0.0-preview.48", "0.0.0-preview.47", true, ""},
		{download, "dev", "", false, ""},
	} {
		if got := winInstallAction(c.running, p.Exe, c.version, c.installed, c.exists); got != c.want {
			t.Fatalf("%+v: %q", c, got)
		}
	}
	script := winShortcutScript(winPaths{Dir: `C:\x\Agent Messenger`, Exe: `C:\O'Brien\AgentMessenger.exe`, Shortcut: `C:\s\Agent Messenger.lnk`})
	if !strings.Contains(script, `'C:\O''Brien\AgentMessenger.exe'`) || strings.Count(script, "'")%2 != 0 {
		t.Fatalf("script %s", script)
	}
	if line, ok := winRemoveCommand(p.Dir); !ok || line != `cmd.exe /d /s /c "ping -n 4 127.0.0.1 >nul & rmdir /s /q "`+p.Dir+`""` {
		t.Fatalf("remove %q %v", line, ok)
	}
	for _, bad := range []string{`C:\a&b\Agent Messenger`, `C:\%USERNAME%\Agent Messenger`, `C:\Users\x`, `relative\Agent Messenger`} {
		if _, ok := winRemoveCommand(bad); ok && runtime.GOOS == "windows" {
			t.Fatalf("remove accepted %q", bad)
		}
	}
	if _, ok := winRemoveCommand(filepath.Join(local, "a&b", "Agent Messenger")); ok {
		t.Fatal("remove accepted a path with &")
	}
	v := winUninstallValues(p, "0.0.0-preview.48")
	if v["UninstallString"] != `"`+p.Exe+`" uninstall` || v["DisplayVersion"] != "0.0.0-preview.48" {
		t.Fatalf("values %+v", v)
	}
}
