package updatefeed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadProgress(t *testing.T) {
	c := Channel{Directory: t.TempDir()}
	if _, ok := c.Progress(); ok {
		t.Fatal("progress without a download")
	}
	p := c.newProgress("0.0.0-preview.74", 1000)
	p.Write(make([]byte, 400))
	p.save()
	got, ok := c.Progress()
	if !ok || got.Done != 400 || got.Total != 1000 || got.Version != "0.0.0-preview.74" {
		t.Fatalf("progress %+v %v", got, ok)
	}
	// A file left by a downloader that stopped long ago is not a download.
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(filepath.Join(c.Directory, progressFile), old, old); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Progress(); ok {
		t.Fatal("stale progress shown")
	}
	p.end()
	if _, err := os.Stat(filepath.Join(c.Directory, progressFile)); !os.IsNotExist(err) {
		t.Fatalf("progress file left: %v", err)
	}
}
