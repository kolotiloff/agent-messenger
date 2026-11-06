package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
)

// CollectDesktop also checks explicit installation directories because apps
// opened through LaunchServices do not inherit the user's interactive PATH.
// Candidates are never executed by discovery.
func CollectDesktop(ctx context.Context, root string) Report {
	r := Collect(ctx, root)
	if runtime.GOOS != "darwin" {
		return r
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return r
	}
	return desktopCandidates(r, []string{filepath.Join(home, ".local", "bin"), "/opt/homebrew/bin", "/usr/local/bin"})
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
					path := filepath.Join(dir, command)
					info, err := os.Stat(path)
					if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
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
