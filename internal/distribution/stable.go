package distribution

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const stableOldPrefix = ".old-"

// ErrStableBusy means another start holds the stable file right now.
var ErrStableBusy = errors.New("stable runtime is busy")

// StableRuntime puts the bytes of pinned (a CacheRuntime copy) at one path
// that never changes between versions, directory/messenger(.exe), and
// returns it with a release function. The caller keeps it until the client
// started from that path has confirmed its start, so two starts cannot
// swap the file under each other; a busy file returns ErrStableBusy.
//
// Windows Firewall remembers an allowed program by its path. A new path per
// version asks the owner again after every update and blocks the server
// until someone answers at that computer (D-118).
//
// A file a running process still uses is renamed aside, never overwritten
// or deleted; old copies are removed once nothing uses them.
func StableRuntime(pinned, directory string) (string, func(), error) {
	want, err := fileDigest(pinned)
	if err != nil {
		return "", nil, err
	}
	if err = os.MkdirAll(directory, 0o700); err != nil {
		return "", nil, err
	}
	if st, err := os.Lstat(directory); err != nil {
		return "", nil, err
	} else if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", nil, errors.New("runtime cache must be a directory")
	}
	lock, err := platform.Acquire(filepath.Join(directory, "stable.lock"))
	if err != nil {
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return "", nil, ErrStableBusy
		}
		return "", nil, err
	}
	release := func() { _ = lock.Release() }
	path, err := placeStable(pinned, directory, want)
	if err != nil {
		release()
		return "", nil, err
	}
	return path, release, nil
}

func placeStable(pinned, directory, want string) (string, error) {
	name := "messenger"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dest := filepath.Join(directory, name)
	removeOldStable(directory)
	valid := func() bool {
		st, err := os.Lstat(dest)
		if err != nil || !platform.IsExecutable(dest, st) {
			return false
		}
		got, err := fileDigest(dest)
		return err == nil && got == want
	}
	if valid() {
		return dest, nil
	}
	in, err := os.Open(pinned)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(directory, ".stable-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err = io.Copy(tmp, in); err != nil {
		return "", err
	}
	if err = tmp.Chmod(0o700); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if got, err := fileDigest(tmp.Name()); err != nil || got != want {
		return "", errors.New("runtime changed during copy")
	}
	// Windows refuses to replace a program a process still runs, but lets
	// it be renamed; the process keeps running from the new name.
	if _, err = os.Lstat(dest); err == nil {
		aside, err := os.CreateTemp(directory, stableOldPrefix)
		if err != nil {
			return "", err
		}
		asideName := aside.Name()
		aside.Close()
		if err = os.Rename(dest, asideName); err != nil {
			os.Remove(asideName)
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err = os.Rename(tmp.Name(), dest); err != nil {
		return "", err
	}
	if !valid() {
		return "", errors.New("stable runtime checksum mismatch")
	}
	if err = syncDir(directory); err != nil {
		return "", err
	}
	removeOldStable(directory)
	return dest, nil
}

// removeOldStable deletes earlier stable files. One a process still runs
// cannot be deleted on Windows and stays until a later start.
func removeOldStable(directory string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stableOldPrefix) && e.Type().IsRegular() {
			_ = os.Remove(filepath.Join(directory, e.Name()))
		}
	}
}
