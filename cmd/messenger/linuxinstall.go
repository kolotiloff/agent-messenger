package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
)

// Owner, 2026-11-12: on Linux a new version could only be put in place by
// asking an agent. A downloaded program file was a second copy: the menu
// went on starting the installed old version, and `install` refused while
// the background client ran. Now, as on Windows, a downloaded file opened
// on a computer with an installation updates that installation (the same
// path an update from the window takes, D-106) and opens from it.

// installedLink is the bin/messenger link of the installation in prefix and
// the version it points at, or "" when prefix holds no installation.
func installedLink(prefix string) (string, string) {
	link := filepath.Join(prefix, "bin", "messenger")
	st, err := os.Lstat(link)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		return "", ""
	}
	target, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", ""
	}
	if want, err := filepath.EvalSymlinks(prefix); err != nil || distribution.InstallPrefix(target) != want {
		return "", ""
	}
	dir := filepath.Base(filepath.Dir(target))
	i := strings.LastIndex(dir, "-")
	if i <= 0 {
		return "", ""
	}
	return link, dir[:i]
}

// openFromInstallation runs before the window of a program file that is not
// itself an installation: it updates an older installation with this
// version and returns the link to open from. "" keeps this file running.
func openFromInstallation(exe, prefix, version string, install func(source, prefix string) error, out io.Writer) string {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if distribution.InstallPrefix(exe) != "" {
		return ""
	}
	link, installed := installedLink(prefix)
	if link == "" {
		return ""
	}
	if selfupdate.Build(version) > selfupdate.Build(installed) {
		if err := install(exe, prefix); err != nil {
			fmt.Fprintln(out, "Установленную программу не удалось обновить, открываю этот файл:", err)
			return ""
		}
		fmt.Fprintln(out, "Установленная программа обновлена до", version)
	}
	return link
}

// linuxSelfInstall is openFromInstallation for the default installation in
// ~/.local, the one `messenger install` makes.
func linuxSelfInstall(out io.Writer) string {
	if runtime.GOOS != "linux" {
		return ""
	}
	exe, err := os.Executable()
	home, herr := os.UserHomeDir()
	if err != nil || herr != nil {
		return ""
	}
	return openFromInstallation(exe, filepath.Join(home, ".local"), buildinfo.Version, func(source, prefix string) error {
		info := buildinfo.Current()
		if !distribution.ValidBuild(info.Version, info.Revision) {
			return errors.New("эта сборка не версия для установки")
		}
		_, err := distribution.InstallUpdate(source, prefix, info)
		return err
	}, out)
}
