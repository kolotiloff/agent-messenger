package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
