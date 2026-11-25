package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJournalWritesAndRotates(t *testing.T) {
	root := t.TempDir()
	Printf("before open is dropped")
	Open(root)
	t.Cleanup(Close)
	old := maxSize
	maxSize = 200
	t.Cleanup(func() { maxSize = old })
	for i := 0; i < 10; i++ {
		Printf("run started task-%d\nwith a line break", i)
	}
	cur, err := os.ReadFile(filepath.Join(root, File))
	if err != nil {
		t.Fatal(err)
	}
	prev, err := os.ReadFile(filepath.Join(root, Previous))
	if err != nil {
		t.Fatal(err)
	}
	all := string(prev) + string(cur)
	if strings.Contains(all, "before open") || !strings.Contains(string(cur), "task-9 with a line break\n") {
		t.Fatalf("journal:\n%s", all)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(cur)), "\n") {
		if !strings.HasSuffix(strings.Fields(line)[0], "Z") {
			t.Fatalf("line without UTC time: %q", line)
		}
	}
}

func TestHelperLogCleanupKeepsActiveAndRecentLogs(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	cutoff := now.Add(-7 * 24 * time.Hour)
	old := cutoff.Add(-time.Hour)
	makeLog := func(name, content string, at time.Time) os.FileInfo {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	makeLog("background-1.log", "", old)
	active := makeLog("background-2.log", "", old)
	makeLog("background-3.log", "most recent useful startup", old)
	makeLog("background-4.log", "", now)
	makeLog("remote-update-1.log", "old diagnostic", old.Add(-time.Hour))
	makeLog("remote-update-2.log", "most recent update", old)
	makeLog("background-notes.log", "manual notes", old)
	makeLog(File, "current journal", old)
	makeLog(Previous, "rotated journal", old)

	pruneHelperLogs(root, cutoff, active)
	for _, name := range []string{"background-1.log", "remote-update-1.log"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was not cleaned: %v", name, err)
		}
	}
	for _, name := range []string{"background-2.log", "background-3.log", "background-4.log", "remote-update-2.log", "background-notes.log", File, Previous} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s was not retained: %v", name, err)
		}
	}
}
