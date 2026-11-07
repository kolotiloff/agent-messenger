//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Readers that keep opening the file (far more often than the client does)
// do not make a replace fail, and never see a missing or partial file.
func TestWindowsReplaceWhileReading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "active-runs.json")
	if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	readErrs := make(chan error, 1)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				raw, err := ReadFile(path)
				if err == nil && string(raw) != "[]" {
					err = os.ErrInvalid
				}
				time.Sleep(5 * time.Millisecond)
				if err != nil {
					select {
					case readErrs <- err:
					default:
					}
					return
				}
			}
		}()
	}
	for i := 0; i < 300; i++ {
		tmp := filepath.Join(dir, "next.tmp")
		if err := os.WriteFile(tmp, []byte("[]"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ReplaceFile(tmp, path); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("replace %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()
	select {
	case err := <-readErrs:
		t.Fatalf("read: %v", err)
	default:
	}
}
