package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// exeSuffix is the file extension of a program started directly. On Windows
// PATH lookup would otherwise also find .cmd scripts, which Messenger does
// not start (D-044).
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// CollectDesktop also checks explicit installation directories because a
// window opened from the desktop does not inherit the user's interactive
// PATH. Candidates are never executed by discovery.
func CollectDesktop(ctx context.Context, root string) Report {
	r := Collect(ctx, root)
	home, err := os.UserHomeDir()
	if err != nil {
		return r
	}
	switch runtime.GOOS {
	case "darwin":
		return desktopCandidates(r, []string{filepath.Join(home, ".local", "bin"), "/opt/homebrew/bin", "/usr/local/bin"})
	case "linux":
		return desktopCandidates(r, []string{filepath.Join(home, ".local", "bin"), "/usr/local/bin", "/usr/bin"})
	case "windows":
		r = desktopCandidates(r, []string{filepath.Join(home, ".local", "bin")})
		if appData := os.Getenv("AppData"); filepath.IsAbs(appData) {
			r = npmCodexCandidate(r, filepath.Join(appData, "npm"))
		}
		if local := os.Getenv("LocalAppData"); filepath.IsAbs(local) {
			r = cursorNodeCandidate(r, filepath.Join(local, "cursor-agent", "versions"))
		}
	}
	return r
}

func desktopCandidates(r Report, directories []string) Report {
	for i := range r.Agents {
		a := &r.Agents[i]
		if a.CLI.State == "candidate_found" {
			continue
		}
		for _, dir := range directories {
			if !filepath.IsAbs(dir) {
				continue
			}
			for _, p := range providers {
				if p.id != a.Provider {
					continue
				}
				for _, command := range p.commands {
					path := filepath.Join(dir, command+exeSuffix())
					info, err := os.Stat(path)
					if err == nil && platform.IsExecutable(path, info) {
						a.CLI.State, a.CLI.Path, a.CLI.Command, a.CLI.Source = "candidate_found", path, command, "installation_directory"
						break
					}
				}
			}
			if a.CLI.State == "candidate_found" {
				break
			}
		}
	}
	return r
}

// npmCodexCandidate finds the Codex program an npm installation keeps inside
// its package on Windows. The npm command itself is a .cmd script, which
// Messenger does not start; the program it runs is a plain .exe.
func npmCodexCandidate(r Report, npm string) Report {
	for i := range r.Agents {
		a := &r.Agents[i]
		if a.Provider != "codex" || a.CLI.State == "candidate_found" {
			continue
		}
		modules := filepath.Join(npm, "node_modules", "@openai")
		var found []string
		// Older packages keep the program in vendor/<target>/codex, newer
		// ones in vendor/<target>/bin.
		var patterns []string
		for _, folder := range []string{"codex", "bin"} {
			patterns = append(patterns,
				filepath.Join(modules, "codex*", "vendor", "*", folder, "codex.exe"),
				filepath.Join(modules, "codex", "node_modules", "@openai", "codex*", "vendor", "*", folder, "codex.exe"))
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			found = append(found, matches...)
		}
		sort.Strings(found)
		for _, path := range found {
			if info, err := os.Stat(path); err == nil && platform.IsExecutable(path, info) {
				a.CLI.State, a.CLI.Path, a.CLI.Command, a.CLI.Source = "candidate_found", path, "codex", "installation_directory"
				break
			}
		}
	}
	return r
}

// cursorNodeCandidate finds Cursor on Windows: the node.exe of the newest
// version folder, which Messenger starts with its index.js (D-052). The
// cursor-agent command itself is a .cmd script.
func cursorNodeCandidate(r Report, versions string) Report {
	for i := range r.Agents {
		a := &r.Agents[i]
		if a.Provider != "cursor" || a.CLI.State == "candidate_found" {
			continue
		}
		if dir := adapters.LatestCursorVersion(versions); dir != "" {
			path := filepath.Join(dir, "node.exe")
			if info, err := os.Stat(path); err == nil && platform.IsExecutable(path, info) {
				a.CLI.State, a.CLI.Path, a.CLI.Command, a.CLI.Source = "candidate_found", path, "cursor-agent", "installation_directory"
			}
		}
	}
	return r
}
