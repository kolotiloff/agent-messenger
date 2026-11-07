//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Lock is the single supervisor lock. SQLite remains a second line of defense.
type Lock struct {
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var ol windows.Overlapped
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err != nil {
		file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("%w: %v", ErrAlreadyRunning, err)
	}
	return &Lock{file: file}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	var ol windows.Overlapped
	err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &ol)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
