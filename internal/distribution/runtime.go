package distribution

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// CacheRuntime pins each desktop-started daemon to immutable bytes outside the
// replaceable .app. Its MCP subprocesses keep using that exact binary after an
// app update. No active daemon or previous cache entry is replaced or deleted.
func CacheRuntime(source, directory string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	s, err := in.Stat()
	if err != nil {
		return "", err
	}
	if !s.Mode().IsRegular() {
		return "", errors.New("runtime must be regular")
	}
	h := sha256.New()
	if _, err = io.Copy(h, in); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	versionDir := filepath.Join(directory, digest)
	if err = os.MkdirAll(versionDir, 0o700); err != nil {
		return "", err
	}
	for _, p := range []string{directory, versionDir} {
		st, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("runtime cache must be a directory")
		}
	}
	name := "messenger"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dest := filepath.Join(versionDir, name)
	validate := func() error {
		st, err := os.Lstat(dest)
		if err != nil {
			return err
		}
		if !platform.IsExecutable(dest, st) {
			return errors.New("invalid cached runtime")
		}
		got, err := fileDigest(dest)
		if err != nil {
			return err
		}
		if got != digest {
			return errors.New("cached runtime checksum mismatch")
		}
		return nil
	}
	if _, err = os.Lstat(dest); err == nil {
		return dest, validate()
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	tmp, err := os.CreateTemp(versionDir, ".runtime-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err = in.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if _, err = io.Copy(tmp, in); err != nil {
		return "", err
	}
	if err = tmp.Chmod(0o700); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	got, err := fileDigest(tmp.Name())
	if err != nil || got != digest {
		return "", errors.New("runtime changed during copy")
	}
	// Link publishes without replacing a concurrent writer's immutable version.
	if err = os.Link(tmp.Name(), dest); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err = validate(); err != nil {
		return "", err
	}
	if err = syncDir(versionDir); err != nil {
		return "", err
	}
	return dest, nil
}
