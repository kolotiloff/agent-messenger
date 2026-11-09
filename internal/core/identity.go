package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// canonicalCLI resolves an absolute path to a regular executable file.
func canonicalCLI(cli string) (string, error) {
	if !filepath.IsAbs(cli) {
		return "", ErrRejected
	}
	resolved, err := filepath.EvalSymlinks(cli)
	if err != nil {
		return "", ErrRejected
	}
	info, err := os.Stat(resolved)
	if err != nil || !platform.IsExecutable(resolved, info) {
		return "", ErrRejected
	}
	return resolved, nil
}

// CanonicalEnvironment is the real CLI storage directory, not a display name.
// Symlinks collapse. On a case-insensitive volume, Profile and profile share one key.
func CanonicalEnvironment(path string) (string, error) {
	return resolveEnvironment(path, sameFileFold)
}

func resolveEnvironment(path string, fold func(dir, diskName, requested string) bool) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: environment must be an absolute directory path", ErrRejected)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
	}
	folded, err := foldWalk(resolved, fold)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(folded)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
	}
	return folded, nil
}

func sameFileFold(dir, diskName, requested string) bool {
	user, errUser := os.Stat(filepath.Join(dir, requested))
	disk, errDisk := os.Stat(filepath.Join(dir, diskName))
	return errUser == nil && errDisk == nil && os.SameFile(user, disk)
}

func foldWalk(path string, fold func(dir, diskName, requested string) bool) (string, error) {
	vol := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path, vol)
	parts := strings.Split(rest, string(os.PathSeparator))
	cur := vol
	if filepath.IsAbs(path) {
		if vol == "" {
			cur = string(os.PathSeparator)
		} else {
			cur = vol + string(os.PathSeparator)
		}
	}
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
		}
		name, err := chooseComponent(cur, part, entries, fold)
		if err != nil {
			return "", err
		}
		next := filepath.Join(cur, name)
		info, err := os.Stat(next)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
		}
		cur = next
	}
	return cur, nil
}

func chooseComponent(dir, part string, entries []os.DirEntry, fold func(dir, diskName, requested string) bool) (string, error) {
	var folded string
	for _, entry := range entries {
		if entry.Name() == part {
			return part, nil
		}
		if fold != nil && strings.EqualFold(entry.Name(), part) && fold(dir, entry.Name(), part) {
			if folded != "" && folded != entry.Name() {
				return "", fmt.Errorf("%w: environment case is ambiguous", ErrRejected)
			}
			folded = entry.Name()
		}
	}
	if folded == "" {
		return "", fmt.Errorf("%w: environment must be an existing directory", ErrRejected)
	}
	return folded, nil
}
