package main

import (
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
)

// Windows installation (T-053, D-082). A downloaded .exe started with a
// double click copies itself to the user's programs folder, adds a Start
// menu shortcut and an entry in «Приложения» with «Удалить», and opens the
// installed copy. No administrator rights are needed. The data folder
// (%LOCALAPPDATA%\AgentMessenger) is never touched, not even by «Удалить».
// The pure parts live here so they are tested on every system.

const (
	winAppName     = "Agent Messenger"
	winProgramFile = "AgentMessenger.exe"
	winUninstallID = "AgentMessenger"
)

type winPaths struct {
	Dir, Exe, Shortcut string
}

// winInstallPaths returns where the program and its shortcut go, from
// %LOCALAPPDATA% and %APPDATA%.
func winInstallPaths(localAppData, appData string) (winPaths, bool) {
	if !filepath.IsAbs(localAppData) || !filepath.IsAbs(appData) {
		return winPaths{}, false
	}
	dir := filepath.Join(localAppData, "Programs", winAppName)
	return winPaths{
		Dir:      dir,
		Exe:      filepath.Join(dir, winProgramFile),
		Shortcut: filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", winAppName+".lnk"),
	}, true
}

// winInstallAction says what a double click on the program at running
// does: "install" (copy over, or first install), "open-installed" (an
// equal or newer version is installed: open that one) or "" (run here:
// this is the installed copy, or a development build).
func winInstallAction(running, installed, version, installedVersion string, installedExists bool) string {
	if !distribution.ValidBuild(version, strings.Repeat("0", 40)) {
		return ""
	}
	if strings.EqualFold(filepath.Clean(running), filepath.Clean(installed)) {
		return ""
	}
	if installedExists && installedVersion != "" && selfupdate.Build(installedVersion) >= selfupdate.Build(version) {
		return "open-installed"
	}
	return "install"
}

// psQuote quotes s for a single-quoted PowerShell string.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// winShortcutScript is the PowerShell that writes the Start menu shortcut.
func winShortcutScript(p winPaths) string {
	return "$s=(New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(p.Shortcut) + ");" +
		"$s.TargetPath=" + psQuote(p.Exe) + ";$s.WorkingDirectory=" + psQuote(p.Dir) + ";" +
		"$s.IconLocation=" + psQuote(p.Exe+",0") + ";$s.Description=" + psQuote(winAppName) + ";$s.Save()"
}

// winRemoveCommand is the whole cmd.exe command line that removes the
// program folder a few seconds after the uninstaller exits (a running
// program cannot delete its own file). It is passed as is: Go's usual
// argument quoting writes \" for inner quotes, which cmd.exe does not
// understand. With /s, cmd.exe drops only the outer pair of quotes. Paths
// cmd.exe could misread are refused; the folder then stays and the owner is
// told.
func winRemoveCommand(dir string) (string, bool) {
	if !filepath.IsAbs(dir) || strings.ContainsAny(dir, "\"%^&|<>!\r\n") || filepath.Base(dir) != winAppName {
		return "", false
	}
	return `cmd.exe /d /s /c "ping -n 4 127.0.0.1 >nul & rmdir /s /q "` + dir + `""`, true
}

// winUninstallValues are the string values of the «Приложения» entry.
func winUninstallValues(p winPaths, version string) map[string]string {
	return map[string]string{
		"DisplayName":     winAppName,
		"DisplayVersion":  version,
		"Publisher":       winAppName,
		"DisplayIcon":     p.Exe + ",0",
		"InstallLocation": p.Dir,
		"UninstallString": `"` + p.Exe + `" uninstall`,
	}
}
