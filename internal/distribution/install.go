// Package distribution installs an immutable, versioned local preview binary.
package distribution

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-preview[0-9A-Za-z.-]*$`)
var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func ValidBuild(version, revision string) bool {
	return len(version) <= 80 && versionPattern.MatchString(version) && revisionPattern.MatchString(revision)
}

// Install does not start a process, modify shell files, migrate SQLite or remove
// previous binaries. The shared supervisor/startup leases protect activation.
func Install(source, prefix string, info buildinfo.Info) (string, error) {
	return install(source, prefix, info, supervisor.WithInstallationLease)
}

// InstallUpdate installs a verified update next to the running version
// while the background client keeps running (T-107). The client runs from
// its own version folder, which is never removed or changed, so it needs no
// supervisor lease; it goes on with its version until it is restarted, as on
// Windows (D-048). The installation's own lock keeps two installs apart.
func InstallUpdate(source, prefix string, info buildinfo.Info) (string, error) {
	return install(source, prefix, info, func(install func() error) error { return install() })
}

func install(source, prefix string, info buildinfo.Info, lease func(func() error) error) (string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return "", errors.New("installer is validated only on macOS and Linux; on Windows start the downloaded .exe with a double click: it installs itself")
	}
	if !ValidBuild(info.Version, info.Revision) {
		return "", errors.New("install requires a versioned preview build with a full source revision")
	}
	if !filepath.IsAbs(prefix) || filepath.Clean(prefix) == string(filepath.Separator) {
		return "", errors.New("prefix must be an absolute user directory")
	}
	var installed string
	err := lease(func() error {
		canonicalSource, err := filepath.EvalSymlinks(source)
		if err != nil {
			return err
		}
		input, err := os.Open(canonicalSource)
		if err != nil {
			return err
		}
		defer input.Close()
		stat, err := input.Stat()
		if err != nil {
			return err
		}
		if !stat.Mode().IsRegular() {
			return errors.New("source is not a regular file")
		}
		h := sha256.New()
		if _, err = io.Copy(h, input); err != nil {
			return err
		}
		digest := hex.EncodeToString(h.Sum(nil))
		if _, err = input.Seek(0, io.SeekStart); err != nil {
			return err
		}
		prefix = filepath.Clean(prefix)
		binDir := filepath.Join(prefix, "bin")
		library := filepath.Join(prefix, "lib", "agent-messenger")
		for _, dir := range []string{binDir, library} {
			if err = os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
		}
		library, err = filepath.EvalSymlinks(library)
		if err != nil {
			return err
		}
		lock, err := platform.Acquire(filepath.Join(library, "install.lock"))
		if err != nil {
			if errors.Is(err, platform.ErrAlreadyRunning) {
				return errors.New("another installation of this program is in progress")
			}
			return err
		}
		defer lock.Release()
		link := filepath.Join(binDir, "messenger")
		if old, err := os.Lstat(link); err == nil {
			if old.Mode()&os.ModeSymlink == 0 {
				return errors.New("existing messenger is not an owned installation link; refusing replacement")
			}
			target, err := os.Readlink(link)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(binDir, target)
			}
			resolved, err := filepath.EvalSymlinks(target)
			if err != nil {
				return errors.New("existing installation link is invalid; refusing replacement")
			}
			if !strings.HasPrefix(resolved, library+string(filepath.Separator)) || filepath.Base(resolved) != "messenger" {
				return errors.New("existing messenger link belongs to another installation")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		versionDir := filepath.Join(library, info.Version+"-"+digest)
		if err = os.Mkdir(versionDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		st, err := os.Lstat(versionDir)
		if err != nil {
			return err
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("version directory is not a real directory")
		}
		dest := filepath.Join(versionDir, "messenger")
		if old, err := os.Lstat(dest); err == nil {
			if !old.Mode().IsRegular() {
				return errors.New("version binary is not a regular file")
			}
			actual, err := fileDigest(dest)
			if err != nil {
				return err
			}
			if actual != digest {
				return errors.New("existing version failed checksum; refusing replacement")
			}
		} else if errors.Is(err, os.ErrNotExist) {
			temp, err := os.CreateTemp(versionDir, ".install-*")
			if err != nil {
				return err
			}
			defer os.Remove(temp.Name())
			if _, err = io.Copy(temp, input); err == nil {
				err = temp.Chmod(0o755)
			}
			if err == nil {
				err = temp.Sync()
			}
			closeErr := temp.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			actual, err := fileDigest(temp.Name())
			if err != nil {
				return err
			}
			if actual != digest {
				return errors.New("source changed during copy")
			}
			if err = os.Rename(temp.Name(), dest); err != nil {
				return err
			}
			if err = syncDir(versionDir); err != nil {
				return err
			}
		} else {
			return err
		}
		if err = syncDir(library); err != nil {
			return err
		}
		temp, err := os.CreateTemp(binDir, ".messenger-link-*")
		if err != nil {
			return err
		}
		name := temp.Name()
		if err = temp.Close(); err != nil {
			return err
		}
		if err = os.Remove(name); err != nil {
			return err
		}
		defer os.Remove(name)
		if err = os.Symlink(dest, name); err != nil {
			return err
		}
		if err = os.Rename(name, link); err != nil {
			return err
		}
		if err = syncDir(binDir); err != nil {
			return fmt.Errorf("installation link changed but directory sync failed: %w", err)
		}
		installed = link
		return nil
	})
	return installed, err
}
func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func syncDir(path string) error { return platform.SyncDir(path) }

// InstallPrefix returns the prefix of an installation made by
// `messenger install`, or "" for a plain program file.
func InstallPrefix(executable string) string { return supervisor.InstallPrefix(executable) }
