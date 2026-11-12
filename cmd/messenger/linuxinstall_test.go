//go:build !windows

package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func fakeInstallation(t *testing.T, version string) (prefix, link string) {
	t.Helper()
	prefix = t.TempDir()
	dir := filepath.Join(prefix, "lib", "agent-messenger", version+"-abc123")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "messenger"), []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(prefix, "bin", "messenger")
	if err := os.Symlink(filepath.Join(dir, "messenger"), link); err != nil {
		t.Fatal(err)
	}
	return prefix, link
}

// Owner, 2026-11-12: a downloaded newer file updates the installation and
// opens from it; an older or equal one opens the installation as it is.
func TestDownloadedFileOpensFromInstallation(t *testing.T) {
	prefix, link := fakeInstallation(t, "v0.0.0-preview.62")
	download := filepath.Join(t.TempDir(), "AgentMessenger-Linux-x64")
	if err := os.WriteFile(download, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if l, v := installedLink(prefix); l != link || v != "v0.0.0-preview.62" {
		t.Fatalf("installed %q %q", l, v)
	}
	installs := 0
	install := func(source, p string) error {
		if source != download || p != prefix {
			t.Fatalf("install %s into %s", source, p)
		}
		installs++
		return nil
	}
	if got := openFromInstallation(download, prefix, "v0.0.0-preview.63", install, io.Discard); got != link || installs != 1 {
		t.Fatalf("newer: %q, %d installs", got, installs)
	}
	if got := openFromInstallation(download, prefix, "v0.0.0-preview.62", install, io.Discard); got != link || installs != 1 {
		t.Fatalf("same: %q, %d installs", got, installs)
	}
	// The installed copy itself, or a computer without an installation,
	// keeps running as it is.
	target, _ := filepath.EvalSymlinks(link)
	if got := openFromInstallation(target, prefix, "v0.0.0-preview.63", install, io.Discard); got != "" {
		t.Fatalf("installed copy: %q", got)
	}
	if got := openFromInstallation(download, t.TempDir(), "v0.0.0-preview.63", install, io.Discard); got != "" {
		t.Fatalf("no installation: %q", got)
	}
	failing := func(string, string) error { return os.ErrPermission }
	if got := openFromInstallation(download, prefix, "v0.0.0-preview.64", failing, io.Discard); got != "" {
		t.Fatalf("failed install opens the installation: %q", got)
	}
}
