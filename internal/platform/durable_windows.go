//go:build windows

package platform

import "golang.org/x/sys/windows"

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
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// SyncDir has nothing to do on Windows: a folder cannot be flushed, and
// ReplaceFile already wrote the rename through.
func SyncDir(string) error { return nil }
