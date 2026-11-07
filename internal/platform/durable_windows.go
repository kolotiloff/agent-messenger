//go:build windows

package platform

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// While another process or goroutine has the file open (for example to read
// it), Windows refuses to replace or open it for a moment. Such a refusal is
// definite: the file was not changed. ReplaceFile and ReadFile try again for
// up to this long before they report the error.
const busyWait = 2 * time.Second

func busy(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

// ReplaceFile renames a fully written temporary file over path. The rename
// is written through to disk before it returns, which is what SyncDir does on
// Unix.
func ReplaceFile(tmp, path string) error {
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(busyWait)
	for {
		err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err == nil || !busy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ReadFile reads a file that ReplaceFile may be replacing at the same time.
// It opens the file with delete sharing, so an open reader does not block a
// rename over the file; it sees either the old or the new contents.
func ReadFile(path string) ([]byte, error) {
	deadline := time.Now().Add(busyWait)
	for {
		raw, err := readShared(path)
		if err == nil || !busy(err) || time.Now().After(deadline) {
			return raw, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readShared(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}

// SyncDir has nothing to do on Windows: a folder cannot be flushed, and
// ReplaceFile already wrote the rename through.
func SyncDir(string) error { return nil }
