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

func TestOwnerMessageAttachmentsReachOnlyTheirRun(t *testing.T) {
	for _, remote := range []bool{false, true} {
		name := "local"
		if remote {
			name = "remote"
		}
		t.Run(name, func(t *testing.T) {
			req := StartTaskRequest{Project: "Game", OpID: "chat", Text: "Initial", Provider: "codex", ByOwner: true}
			var origin, worker taskFixture
			relay := func(t *testing.T, outward bool) {}
			var pair *netPair
			if remote {
				pair = newNetPair(t)
				origin, worker, req.Computer = pair.a, pair.b, "dev-b"
				if err := worker.st.SetNetAccept(true); err != nil {
					t.Fatal(err)
				}
				relay = func(t *testing.T, outward bool) { pair.carry(t, outward) }
			} else {
				origin = newTaskFixture(t)
				worker = origin
			}
			id, _, err := origin.st.StartTask(req)
			if err != nil {
				t.Fatal(err)
			}
			relay(t, true)
			relay(t, false)
			workerID := id
			if remote {
				workerID = pair.mirror(t, id).ID
			}
			const session = "019a0000-0000-7000-8000-0000000000b1"
			run := worker.launch(t, workerID, session)
			files := []TaskAttachment{{Path: "photo.png", Data: []byte{0, 1, 255}}}
			send := func(question, op string) error {
				return origin.st.AddOwnerMessageWithAttachments(id, question, op, "Look at this", files)
			}
			if remote {
				if err := send("", "note"); !errors.Is(err, ErrRejected) {
					t.Fatalf("old executor accepted uploaded bytes: %v", err)
				}
				if pending, err := origin.st.NetPending(testBroker, 20); err != nil || len(pending) != 0 {
					t.Fatalf("old executor left an outbox entry: %+v, %v", pending, err)
				}
				if err := origin.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Age: 1, Version: "1.0.17-preview.143"}}); err != nil {
					t.Fatal(err)
				}
			}
			if err := send("", "note"); err != nil {
				t.Fatal(err)
			}
			if err := send("", "note"); err != nil {
				t.Fatalf("identical retry: %v", err)
			}
			files[0].Data = []byte{2, 3}
			if err := send("", "note"); !errors.Is(err, ErrRejected) {
				t.Fatalf("changed retry: %v", err)
			}
			files[0].Data = []byte{0, 1, 255}
			relay(t, true)
			check := func(run TaskRun, expected []TaskAttachment) {
				t.Helper()
				brief, err := worker.st.ReadTask(run.ID, run.Receipt, false)
				if err != nil || !sameTaskAttachments(brief.Attachments, expected) {
					t.Fatalf("run %d files: %+v, %v", run.Ordinal, brief.Attachments, err)
				}
			}
			expected := []TaskAttachment{{Path: "message-attachments/1/photo.png", Data: files[0].Data}}
			check(run, nil)
			worker.dispose(t, run, DispositionReport, "Done")
			if _, err := worker.st.FinishTaskRun(run.ID, cleanExit); err != nil {
				t.Fatal(err)
			}
			relay(t, false)
			run = worker.launch(t, workerID, session)
			check(run, expected)
			worker.dispose(t, run, DispositionAskOwner, "Which photo?")
			if _, err := worker.st.FinishTaskRun(run.ID, cleanExit); err != nil {
				t.Fatal(err)
			}
			relay(t, false)
			task, err := origin.st.TaskByID(id)
			if err != nil || task.OpenQuestion == nil {
				t.Fatalf("owner question: %+v, %v", task, err)
			}
			if err := send(task.OpenQuestion.ID, "answer"); err != nil {
				t.Fatal(err)
			}
			relay(t, true)
			run = worker.launch(t, workerID, session)
			check(run, expected)
			worker.dispose(t, run, DispositionReport, "Answered")
			if _, err := worker.st.FinishTaskRun(run.ID, cleanExit); err != nil {
				t.Fatal(err)
			}
			relay(t, false)
			if err := send("", "follow-up"); err != nil {
				t.Fatal(err)
			}
			relay(t, true)
			run = worker.launch(t, workerID, session)
			check(run, expected)
			worker.dispose(t, run, DispositionReport, "Finished")
			if _, err := worker.st.FinishTaskRun(run.ID, cleanExit); err != nil {
				t.Fatal(err)
			}
			relay(t, false)
			if err := origin.st.AddOwnerNote(id, "plain", "With no files"); err != nil {
				t.Fatal(err)
			}
			relay(t, true)
			check(worker.launch(t, workerID, session), nil)
		})
	}
}
