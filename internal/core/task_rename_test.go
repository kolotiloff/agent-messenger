package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRenameReplicatesToExecutionMirror(t *testing.T) {
	for _, mode := range []string{"single", "split"} {
		t.Run(mode, func(t *testing.T) {
			p := newNetPair(t)
			if mode == "split" {
				for _, f := range []*taskFixture{&p.a, &p.b} {
					layout := filepath.Join(t.TempDir(), "layout")
					ctx := context.Background()
					if err := StageProjectMigration(ctx, f.path, layout); err != nil {
						t.Fatal(err)
					}
					store, err := openSplitStore(ctx, layout)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { store.Close() })
					f.st = store
				}
			}
			id := p.startRemote(t, "rename-task", "claude", "First title")
			p.carry(t, true)
			p.carry(t, false)
			before := p.mirror(t, id)
			if err := p.b.st.RenameTask(before.ID, before.Title, "Executor title"); !errors.Is(err, ErrRejected) {
				t.Fatalf("executor renamed its mirror: %v", err)
			}
			if err := p.a.st.RenameTask(id, "Stale title", "Second title"); !errors.Is(err, ErrRejected) {
				t.Fatalf("stale edit was accepted: %v", err)
			}
			if err := p.a.st.RenameTask(id, "First title", "Second title"); err != nil {
				t.Fatal(err)
			}
			if err := p.a.st.RenameTask(id, "Second title", "Final title"); err != nil {
				t.Fatal(err)
			}
			if err := p.a.st.RenameTask(id, "Second title", "Final title"); err != nil {
				t.Fatalf("retry of an already applied title: %v", err)
			}
			updates, err := p.a.st.NetPending(testBroker, 10)
			if err != nil || len(updates) != 2 {
				t.Fatalf("durable updates: %+v, %v", updates, err)
			}
			shared, err := p.a.st.SharedTasks(time.Now().Add(-time.Hour))
			if err != nil || len(shared) != 1 || shared[0].ID != id || shared[0].Title != "Final title" {
				t.Fatalf("shared snapshot: %+v, %v", shared, err)
			}
			var self string
			if err := p.a.st.tx(func(tx *sql.Tx) error {
				var err error
				self, err = localComputer(tx)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			forged, err := json.Marshal(netBody{Kind: "rename_task", Task: id, Title: "Forged title", TitleVersion: 999, Computer: self})
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range []NetIncoming{
				{From: "other-peer", OpID: updates[1].OpID, Body: forged},
				{From: HostPeerID, OpID: updates[1].OpID, Body: updates[1].Body},
				{From: HostPeerID, OpID: updates[0].OpID, Body: updates[0].Body},
				{From: HostPeerID, OpID: updates[1].OpID, Body: updates[1].Body},
			} {
				p.seq++
				message.Seq = p.seq
				if err := p.b.st.ApplyNet(testBroker, message); err != nil {
					t.Fatal(err)
				}
			}
			after := p.mirror(t, id)
			if after.Title != "Final title" || after.ID != before.ID || after.SessionID != before.SessionID || after.Version != before.Version+1 {
				t.Fatalf("mirror: before=%+v after=%+v", before, after)
			}
		})
	}
}
