package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestTaskAttachmentRetryAndValidation(t *testing.T) {
	f := newTaskFixture(t)
	req := StartTaskRequest{Project: "Game", OpID: "owner-attachments", Provider: "claude", Text: "Read the files", ByOwner: true}
	req.Attachments = []TaskAttachment{
		{Path: "assets/payload.dat", Data: []byte{0, 1, 255}},
		{Path: "assets/empty", Dir: true},
	}
	id, created, err := f.st.StartTask(req)
	if err != nil || !created {
		t.Fatalf("create: %s, %t, %v", id, created, err)
	}
	req.Attachments[0], req.Attachments[1] = req.Attachments[1], req.Attachments[0]
	repeated, created, err := f.st.StartTask(req)
	if err != nil || created || repeated != id {
		t.Fatalf("retry: %s, %t, %v", repeated, created, err)
	}
	req.Attachments[1].Data = []byte{0, 2, 255}
	if _, _, err := f.st.StartTask(req); !errors.Is(err, ErrRejected) {
		t.Fatalf("changed retry: %v", err)
	}
	for _, files := range [][]TaskAttachment{
		{{Path: "../escape"}}, {{Path: "safe/NUL.txt"}}, {{Path: "a"}, {Path: "A/f.txt"}},
		{{Path: "safe/a"}, {Path: "Safe/b"}}, {{Path: "huge", Data: make([]byte, MaxTaskAttachmentBytes+1)}},
	} {
		if _, err := normalizeTaskAttachments(files); !errors.Is(err, ErrRejected) {
			t.Fatalf("invalid attachment accepted: %v", err)
		}
	}
}

func TestSplitRemoteStartStoresAttachments(t *testing.T) {
	f := newSplitRunFixture(t)
	attachments := []TaskAttachment{{Path: "folder/model.bin", Data: []byte{0, 1, 255}}}
	body, err := json.Marshal(netBody{
		Kind: "start_with_attachments", Task: "task-remote-files", Project: "Game", Agent: "codex",
		Text: "Read the files", Computer: "origin-machine", Attachments: attachments,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := NetIncoming{Seq: 1, From: "dev-a", OpID: "start-remote-files", Body: body}
	for range 2 {
		if err := applySplitRemoteStart(context.Background(), f.computer, f.project, testBroker, msg); err != nil {
			t.Fatal(err)
		}
	}
	var raw []byte
	project := f.projects[projectMigrationFile("Game")]
	if err := project.QueryRow("SELECT payload FROM task_attachments").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored []TaskAttachment
	if err := json.Unmarshal(raw, &stored); err != nil || !sameTaskAttachments(attachments, stored) {
		t.Fatalf("stored bytes: %v, %v", stored, err)
	}
}
