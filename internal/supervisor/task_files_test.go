package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestTaskFilesUseExecutorProject(t *testing.T) {
	h := newTaskHarness(t, "silent", nil)
	defer h.stop()
	id := h.startTask("report-file", "claude", "Review a document")
	docs := filepath.Join(h.folder, "Docs")
	if err := os.Mkdir(docs, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(docs, "Plan.md")
	if err := os.WriteFile(path, []byte("# Plan\n\nHello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	read := func(id, path string) control.Response {
		t.Helper()
		got, err := control.Call(h.root, control.Request{Cmd: "task-file", MessageID: id, Workspace: path, Action: "preview"})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, name := range []string{"Docs/Plan.md", path} {
		got := read(id, name)
		if !got.OK || !strings.Contains(string(got.Overview), `"format":"markdown"`) || !strings.Contains(string(got.Overview), "Hello") {
			t.Fatalf("wanted a scoped Markdown preview for %q, got %s (%s)", name, got.Overview, got.Error)
		}
	}
	if got := read("wrong-task", path); got.OK {
		t.Fatal("unrecognized task got a preview")
	}
	other := t.TempDir()
	if err := h.st.SetProjectFolder("Game", other); err != nil {
		t.Fatal(err)
	}
	if got := read(id, path); got.OK {
		t.Fatal("stale task folder accessed after moving project")
	}
}

func TestReadTaskFileScopesAndBounds(t *testing.T) {
	ctx := context.Background()
	folder := t.TempDir()
	root := filepath.Join(folder, "project")
	other := filepath.Join(folder, "outside")
	for _, dir := range []string{root, other} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	private := filepath.Join(other, "private.md")
	if err := os.WriteFile(private, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(root, "binary.bin")
	if err := os.WriteFile(native, []byte{1, 0, 255}, 0o600); err != nil {
		t.Fatal(err)
	}
	view, err := readTaskFile(ctx, root, "binary.bin", "preview")
	if err != nil || view.Format != "" || view.Text != "" || !view.CanSave {
		t.Fatalf("unexpected binary preview %+v, %v", view, err)
	}
	view, err = readTaskFile(ctx, root, "binary.bin", "download")
	if err != nil || len(view.Data) != 3 || view.Data[2] != 255 {
		t.Fatalf("unexpected binary download %+v, %v", view, err)
	}
	large := filepath.Join(root, "large.txt")
	if err := os.WriteFile(large, make([]byte, maxTaskFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTaskFile(ctx, root, "large.txt", "download"); err == nil {
		t.Fatal("oversized file accepted")
	}
	blocked := []string{"../outside/private.md", private, "."}
	link := filepath.Join(root, "elsewhere")
	if err := os.Symlink(other, link); err == nil {
		blocked = append(blocked, "elsewhere/private.md")
	}
	for _, path := range blocked {
		if _, err := readTaskFile(ctx, root, path, "preview"); err == nil {
			t.Errorf("file escaped its project: %q", path)
		}
	}
}
