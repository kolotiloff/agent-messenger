package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestProjectActivationSealsLegacyWritersAndReopensTheSameFiles(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	game := f.start(t, "old-game", "claude", "Game before the update")
	ctx := context.Background()
	if _, err := f.st.db.Exec(`CREATE TRIGGER fail_generation BEFORE INSERT ON meta
 WHEN NEW.key='project_storage_generation' BEGIN SELECT RAISE(ABORT,'fixture cutover interruption'); END`); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(ctx, f.path); err == nil || !strings.Contains(err.Error(), "fixture cutover interruption") {
		t.Fatalf("activation did not stop before committing: %v", err)
	}
	if _, err := f.st.db.Exec("DROP TRIGGER fail_generation"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Site", OpID: "old-site", Provider: "claude", Text: "Site before successful activation"}); err != nil {
		t.Fatalf("legacy queue stopped before activation: %v", err)
	}
	published, err := filepath.Glob(filepath.Join(filepath.Dir(f.path), projectGenerationPrefix+"*"))
	if err != nil || len(published) != 0 {
		t.Fatalf("interrupted candidate was exposed: %v %v", published, err)
	}
	if err := ActivateProjectStorage(ctx, f.path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "old-write", Provider: "claude", Text: "Outdated writer"}); err == nil || !strings.Contains(err.Error(), "legacy queue was sealed") {
		t.Fatalf("a legacy connection could still write: %v", err)
	}
	active, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	newID, _, err := active.StartTask(StartTaskRequest{Project: "Game", OpID: "new-write", Provider: "claude", Text: "First new task"})
	if err != nil {
		t.Fatal(err)
	}
	if err := active.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(ctx, f.path); err != nil {
		t.Fatalf("repeated activation: %v", err)
	}
	active, err = Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	gameRows, err := active.Tasks("Game", 10)
	if err != nil || len(gameRows) != 2 || gameRows[0].ID != newID || gameRows[1].ID != game {
		t.Fatalf("Game history after restart: %+v %v", gameRows, err)
	}
	siteRows, err := active.Tasks("Site", 10)
	if err != nil || len(siteRows) != 1 || siteRows[0].Text != "Site before successful activation" {
		t.Fatalf("Site history after restart: %+v %v", siteRows, err)
	}
	generations, err := filepath.Glob(filepath.Join(filepath.Dir(f.path), projectGenerationPrefix+"*"))
	if err != nil || len(generations) != 1 {
		t.Fatalf("activation did not reuse one generation: %v %v", generations, err)
	}
	if _, err := os.Stat(filepath.Join(generations[0], migrationBackupFile)); err != nil {
		t.Fatal(err)
	}
}

func TestProjectActivationRefusesAnUnfinishedLaunch(t *testing.T) {
	f := newTaskFixture(t)
	task := f.start(t, "in-progress", "claude", "A worker still owns the run")
	runs, err := f.st.TaskRuns(task)
	if err != nil || len(runs) != 1 {
		t.Fatalf("launch intent: %v %v", runs, err)
	}
	if _, _, err := f.st.BeginTaskRun(runs[0].ID, "unresolved-fixture-reservation"); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(context.Background(), f.path); !errors.Is(err, ErrRejected) {
		t.Fatalf("activation should preserve the unresolved worker: %v", err)
	}
	generation, err := readProjectGeneration(f.st.db)
	if err != nil || generation != "" {
		t.Fatalf("an unresolved run selected another generation: %q %v", generation, err)
	}
}

func TestProjectActivationWaitsForAnOpenCoordinator(t *testing.T) {
	f := newTaskFixture(t)
	ctx := context.Background()
	coordinator, err := platform.ReadObservation(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.ClaimCoordinator("Game", "fixture", coordinator); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(ctx, f.path); !errors.Is(err, ErrProjectStorageBusy) {
		t.Fatalf("activation must defer while a coordinator owns the queue: %v", err)
	}
	if _, err := f.st.db.Exec(`UPDATE coordinator_chats SET boot_id='old-fixture-boot'`); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(ctx, f.path); err != nil {
		t.Fatal(err)
	}
	st, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	project, err := st.projectStore("Game")
	if err != nil {
		t.Fatal(err)
	}
	for _, queue := range []*Store{st, project} {
		var mode string
		if err := queue.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Fatalf("journal mode after startup: %q %v", mode, err)
		}
	}
}
