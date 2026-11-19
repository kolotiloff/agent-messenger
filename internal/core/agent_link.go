package core

import (
	"os"
	"path/filepath"
	"runtime"
)

// agentLinkNames are the names an agent's installer gives its command.
var agentLinkNames = map[string][]string{
	"claude": {"claude"},
	"codex":  {"codex"},
	"cursor": {"cursor-agent", "agent"},
}

// agentLinkDirs are the folders where installers put those commands.
var agentLinkDirs = func() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home != "" {
		dirs = append([]string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin")}, dirs...)
	}
	return dirs
}

// agentLinkFor returns the agent's command link that points at cli right
// now, or "" when cli is itself a link, or no known link points there
// (T-207). The link then follows the agent's own updates.
func agentLinkFor(provider, cli string) string {
	if runtime.GOOS == "windows" || cli == "" {
		return ""
	}
	if info, err := os.Lstat(cli); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	for _, dir := range agentLinkDirs() {
		for _, name := range agentLinkNames[provider] {
			link := filepath.Join(dir, name)
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if resolved, err := filepath.EvalSymlinks(link); err == nil && resolved == cli {
				return link
			}
		}
	}
	return ""
}
