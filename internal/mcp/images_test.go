package mcp

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// T-175: the worker names a picture with its report; the coordinator sees
// it as an image.
func TestWorkerPictureReachesTheCoordinatorAsAnImage(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	c.send(0, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	c.expect(0, 2*time.Second)
	var started struct {
		TaskID string `json:"task_id"`
	}
	if text, ok := c.tool("start_task", map[string]any{"op_id": "o1", "agent": "claude", "text": "draw"}, &started); !ok {
		t.Fatal(text)
	}
	runs, err := st.NextTaskRunIntents(10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs %v %v", runs, err)
	}
	run := runs[0]
	r, _ := st.TaskRun(run)
	if _, _, err := st.BeginTaskRun(run, "res"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTaskRunProcess(run, platform.Observation{PID: 9, PGID: 9, StartToken: "s", BootID: "b"}); err != nil {
		t.Fatal(err)
	}
	tk, _ := st.TaskByID(started.TaskID)
	if err := st.BindTaskSession(run, tk.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadTask(run, r.Receipt, false); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tk.Folder, "shots"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tk.Folder, "shots", "level.png"), buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tk.Folder, "notes.txt"), []byte("not a picture"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := callTaskTool(st, run, r.Receipt, map[string]any{"name": "report", "arguments": map[string]any{"text": "done", "images": []any{"notes.txt"}}})
	if bad["isError"] != true {
		t.Fatalf("a text file was taken as a picture: %v", bad)
	}
	good := callTaskTool(st, run, r.Receipt, map[string]any{"name": "report", "arguments": map[string]any{"text": "Level drawn", "images": []any{"shots/level.png"}}})
	if good["isError"] != false {
		t.Fatalf("report with a picture refused: %v", good)
	}
	if _, err := st.FinishTaskRun(run, core.RunEvidence{ExitObserved: true, GroupEmpty: true, ProviderOK: true}); err != nil {
		t.Fatal(err)
	}
	c.next++
	c.send(c.next, "tools/call", map[string]any{"name": "task_images", "arguments": map[string]any{"task_id": started.TaskID}})
	resp, _ := c.expect(c.next, 5*time.Second)
	content, _ := resp["result"].(map[string]any)["content"].([]any)
	if len(content) < 2 {
		t.Fatalf("no image in %v", resp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	img, _ := content[1].(map[string]any)
	data, _ := base64.StdEncoding.DecodeString(img["data"].(string))
	if img["type"] != "image" || img["mimeType"] != "image/png" || !bytes.Equal(data, buf.Bytes()) || !strings.Contains(text, "level.png") {
		t.Fatalf("image %v %q", img["mimeType"], text)
	}
}
