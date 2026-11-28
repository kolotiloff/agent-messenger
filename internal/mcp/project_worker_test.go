package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestWorkerMCPReadsFromItsProjectFile(t *testing.T) {
	st := newCoordinatorStore(t)
	id, _, err := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "split-worker", Provider: "claude", Text: "Only the Game worker gets this task"})
	if err != nil {
		t.Fatal(err)
	}
	runs, err := st.NextTaskRunIntents(2)
	if err != nil || len(runs) != 1 {
		t.Fatalf("fixture run: %v %v", runs, err)
	}
	run, task, err := st.BeginTaskRun(runs[0], "fixture-reservation")
	if err != nil || task.ID != id {
		t.Fatalf("fixture launch: %q %v", task.ID, err)
	}
	if err := st.RecordTaskRunProcess(run.ID, platform.Observation{PID: 9, PGID: 9, StartToken: "fixture", BootID: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := st.BindTaskSession(run.ID, task.SessionID); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "layout")
	if err := core.StageProjectMigration(context.Background(), filepath.Join(filepath.Dir(task.Folder), "queue.sqlite"), dir); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "project-*.sqlite"))
	if err != nil || len(files) != 2 {
		t.Fatalf("project files: %v %v", files, err)
	}
	const request = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_task","arguments":{}}}
`
	var accepted, rejected int
	for _, file := range files {
		var output bytes.Buffer
		err := ServeTask(file, run.ID, run.Receipt, strings.NewReader(request), &output)
		if errors.Is(err, core.ErrRejected) {
			rejected++
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), `"name":"kotel-task"`) || !strings.Contains(output.String(), task.Text) || !strings.Contains(output.String(), `"isError":false`) {
			t.Fatalf("worker did not read its task: %s", &output)
		}
		accepted++
		output.Reset()
		if err := ServeTask(file, run.ID, "wrong-receipt", strings.NewReader(request), &output); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), task.Text) || !strings.Contains(output.String(), `"isError":true`) {
			t.Fatalf("worker receipt was not enforced: %s", &output)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted %d project files, rejected %d", accepted, rejected)
	}
}

type attachmentResponseWriter func([]byte) (int, error)

func (write attachmentResponseWriter) Write(data []byte) (int, error) { return write(data) }

func TestWorkerAttachmentsNeedReceiptAndAreRemovedOnEOF(t *testing.T) {
	st := newCoordinatorStore(t)
	payload := []byte{0, 1, 255}
	_, _, err := st.StartTask(core.StartTaskRequest{
		Project: "Game", Provider: "claude", OpID: "worker-attachment", Text: "Read the attachment",
		Attachments: []core.TaskAttachment{{Path: "bundle/item.bin", Data: payload}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runs, err := st.NextTaskRunIntents(2)
	if err != nil || len(runs) != 1 {
		t.Fatalf("fixture runs: %v %v", runs, err)
	}
	run, task, err := st.BeginTaskRun(runs[0], "fixture-reservation")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTaskRunProcess(run.ID, platform.Observation{PID: 9, PGID: 9, StartToken: "fixture", BootID: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := st.BindTaskSession(run.ID, task.SessionID); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(filepath.Dir(task.Folder), "queue.sqlite")
	const request = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_task","arguments":{}}}` + "\n"
	var denied bytes.Buffer
	if err := ServeTask(db, run.ID, "wrong-receipt", strings.NewReader(request), &denied); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(denied.String(), `"isError":true`) || strings.Contains(denied.String(), "attachment_paths") {
		t.Fatalf("unexpected denial: %s", &denied)
	}
	var root string
	output := attachmentResponseWriter(func(data []byte) (int, error) {
		var response struct {
			Result struct {
				Content []struct{ Text string } `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return 0, err
		}
		if len(response.Result.Content) == 0 {
			return 0, io.ErrUnexpectedEOF
		}
		var content struct {
			Paths []string `json:"attachment_paths"`
		}
		if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &content); err != nil {
			return 0, err
		}
		if len(content.Paths) != 1 {
			t.Fatalf("attachment paths: %v", content.Paths)
		}
		root = content.Paths[0]
		actual, err := os.ReadFile(filepath.Join(root, "item.bin"))
		if err != nil || !bytes.Equal(actual, payload) {
			t.Fatalf("attachment data %v, err %v", actual, err)
		}
		return len(data), nil
	})
	if err := ServeTask(db, run.ID, run.Receipt, strings.NewReader(request), output); err != nil {
		t.Fatal(err)
	}
	if root == "" {
		t.Fatal("worker did not receive attachment path")
	}
	if _, err := os.Stat(filepath.Dir(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("attachment root survived worker EOF: %v", err)
	}
}
