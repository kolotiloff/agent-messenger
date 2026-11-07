//go:build unix

package platform

import "os"

// ReplaceFile renames a fully written temporary file over path.
func ReplaceFile(tmp, path string) error { return os.Rename(tmp, path) }

// SyncDir makes a rename in dir durable.
func SyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
