package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitProjectCreationAndRestoreKeepTheSeparateHistory(t *testing.T) {
	f := newSplitRunFixture(t)
	s, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	folder := t.TempDir()
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_project_catalog BEFORE INSERT ON project_storage_stage
 WHEN NEW.project='New' BEGIN SELECT RAISE(ABORT,'fixture catalog interruption'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject("New", folder); err == nil || !strings.Contains(err.Error(), "fixture catalog interruption") {
		t.Fatalf("expected interrupted project creation: %v", err)
	}
	path := filepath.Join(f.root, projectMigrationFile("New"))
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.StartTask(StartTaskRequest{Project: "New", OpID: "work", Provider: "claude", Text: "work"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("an unpublished project accepted work: %v", err)
	}
	if _, err := f.computer.Exec("DROP TRIGGER fail_project_catalog"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject("New", folder); err != nil {
		t.Fatal(err)
	}
	if reopened, err := os.Stat(path); err != nil || !os.SameFile(first, reopened) {
		t.Fatalf("replay replaced the prepared file: %v", err)
	}
	id, _, err := s.StartTask(StartTaskRequest{Project: "New", OpID: "work", Provider: "claude", Text: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProject("New"); !errors.Is(err, ErrRejected) {
		t.Fatalf("project with open work was hidden: %v", err)
	}
	if _, err := s.CancelTask(id); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProject("New"); err != nil {
		t.Fatal(err)
	}
	list, err := s.Projects()
	if err != nil || len(list) != 2 {
		t.Fatalf("visible projects after removal: %+v %v", list, err)
	}
	if err := s.CreateProject("New", folder); err != nil {
		t.Fatal(err)
	}
	if history, err := s.Tasks("New", 10); err != nil || len(history) != 1 || history[0].ID != id {
		t.Fatalf("removed history was not restored: %+v %v", history, err)
	}
	if other, err := s.Tasks("Game", 10); err != nil || len(other) != 0 {
		t.Fatalf("creating another project altered Game: %+v %v", other, err)
	}
}
