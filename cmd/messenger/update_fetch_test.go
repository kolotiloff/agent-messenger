//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

func TestUpdateFetchReportsExistingDownloadWithoutError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root, err := platform.DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	channel := filepath.Join(root, "updates")
	if err := os.MkdirAll(channel, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := platform.Acquire(filepath.Join(channel, "download.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	original := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = original }()
	if err := runUpdateFetch("", nil); err != nil {
		t.Fatalf("busy download is not a network failure: %v", err)
	}
	if _, err := output.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var result updatefeed.SyncResult
	if err := json.NewDecoder(output).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.State != "busy" || result.Updated || result.Version != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	// The second process must not take over or release the first process's lock.
	if other, err := platform.Acquire(filepath.Join(channel, "download.lock")); err == nil {
		other.Release()
		t.Fatal("active downloader lost its lock")
	}
}
