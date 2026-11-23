package core

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

type splitRunFixture struct {
	root     string
	computer *sql.DB
	projects map[string]*sql.DB
}

func newSplitRunFixture(t *testing.T) *splitRunFixture {
	t.Helper()
	legacy := newTaskFixture(t)
	if err := legacy.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	f := &splitRunFixture{root: filepath.Join(t.TempDir(), "layout"), projects: map[string]*sql.DB{}}
	if err := StageProjectMigration(context.Background(), legacy.path, f.root); err != nil {
		t.Fatal(err)
	}
	f.computer = f.open(t, migrationComputerFile)
	for _, project := range []string{"Game", "Site"} {
		file := projectMigrationFile(project)
		f.projects[file] = f.open(t, file)
	}
	return f
}

func (f *splitRunFixture) open(t *testing.T, file string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dsn(filepath.Join(f.root, file)))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func (f *splitRunFixture) project(file string) (*sql.DB, error) {
	if db := f.projects[file]; db != nil {
		return db, nil
	}
	return nil, errors.New("unknown fixture file")
}

func (f *splitRunFixture) runID(t *testing.T, name, task string) string {
	t.Helper()
	var id string
	if err := f.projects[projectMigrationFile(name)].QueryRow("SELECT id FROM task_runs WHERE task_id=?", task).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSplitLocalAdmissionRepairsRouteWithoutCreatingAnotherTask(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	req := StartTaskRequest{Project: "Game", OpID: "local-create", Provider: "codex", Text: "Do local work"}
	wrongFile := func(string) (*sql.DB, error) { return f.project(projectMigrationFile("Site")) }
	if _, _, err := startSplitLocalTask(ctx, f.computer, wrongFile, req); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "belongs to another project") {
		t.Fatalf("wrong project accepted: %v", err)
	}
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_route BEFORE INSERT ON task_storage_routes
 BEGIN SELECT RAISE(ABORT, 'fixture route write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := startSplitLocalTask(ctx, f.computer, f.project, req); err == nil || !strings.Contains(err.Error(), "fixture route write failed") {
		t.Fatalf("expected route failure: %v", err)
	}
	game := f.projects[projectMigrationFile("Game")]
	var saved string
	if err := game.QueryRow("SELECT id FROM tasks WHERE op_id=?", req.OpID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	run := f.runID(t, "Game", saved)
	if _, _, err := beginSplitLocalRun(ctx, f.computer, f.project, "Game", run, "not-yet"); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "route was not committed") {
		t.Fatalf("unregistered task was eligible for launch: %v", err)
	}

	if err := f.computer.Close(); err != nil {
		t.Fatal(err)
	}
	f.computer = f.open(t, migrationComputerFile)
	if _, err := f.computer.Exec("DROP TRIGGER fail_route"); err != nil {
		t.Fatal(err)
	}
	id, created, err := startSplitLocalTask(ctx, f.computer, f.project, req)
	if err != nil || created || id != saved {
		t.Fatalf("replayed create: id=%q created=%t err=%v", id, created, err)
	}
	var tasks, runs int
	if err := game.QueryRow("SELECT (SELECT COUNT(*) FROM tasks),(SELECT COUNT(*) FROM task_runs)").Scan(&tasks, &runs); err != nil || tasks != 1 || runs != 1 {
		t.Fatalf("duplicate task or run: %d %d %v", tasks, runs, err)
	}
	req.Text = "Different work"
	if _, _, err := startSplitLocalTask(ctx, f.computer, f.project, req); !errors.Is(err, ErrRejected) {
		t.Fatalf("operation id reused for different work: %v", err)
	}
}

func TestSplitAdmissionSerializesIndependentConnections(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	gate, err := beginSplitAdmission(ctx, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	other, err := sql.Open("sqlite", sqliteFileURL(filepath.Join(f.root, migrationComputerFile), url.Values{"_pragma": {"busy_timeout(1)"}}))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if concurrent, err := beginSplitAdmission(ctx, other); err == nil {
		concurrent.Rollback()
		t.Fatal("another connection took admission while it was held")
	}
	if err := gate.Rollback(); err != nil {
		t.Fatal(err)
	}
	resumed, err := beginSplitAdmission(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	resumed.Rollback()
}

func TestSplitProjectsShareOnlyExplicitlyNamedResources(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	start := func(project, op, resource string) (string, string) {
		t.Helper()
		id, created, err := startSplitLocalTask(ctx, f.computer, f.project, StartTaskRequest{Project: project, OpID: op, Provider: "codex", Text: "fixture", Lock: resource})
		if err != nil || !created {
			t.Fatalf("create %s: %s %t %v", project, id, created, err)
		}
		return id, f.runID(t, project, id)
	}
	launch := func(project, run string) error {
		t.Helper()
		_, _, err := beginSplitLocalRun(ctx, f.computer, f.project, project, run, "res-"+run)
		return err
	}

	_, independentGame := start("Game", "independent", "")
	_, independentSite := start("Site", "independent", "")
	if err := launch("Game", independentGame); err != nil {
		t.Fatal(err)
	}
	if err := launch("Site", independentSite); err != nil {
		t.Fatalf("a task without a lock blocked another project: %v", err)
	}

	first, firstRun := start("Game", "locked", "workspace-cache")
	second, secondRun := start("Site", "locked", " Workspace-CACHE ")
	gameFile, siteFile := projectMigrationFile("Game"), projectMigrationFile("Site")
	// Use equal timestamps to exercise the common enqueue order as well.
	for file, task := range map[string]string{gameFile: first, siteFile: second} {
		if _, err := f.projects[file].Exec("UPDATE tasks SET created_at=? WHERE id=?", "2026-01-01T00:00:00Z", task); err != nil {
			t.Fatal(err)
		}
	}
	firstResult, secondResult := make(chan error, 1), make(chan error, 1)
	// A separate connection, with deferred transactions, stands in for
	// another component opening the computer's database independently.
	otherComputer, err := sql.Open("sqlite", sqliteFileURL(filepath.Join(f.root, migrationComputerFile), url.Values{"_pragma": {sqliteBusyTimeout}}))
	if err != nil {
		t.Fatal(err)
	}
	defer otherComputer.Close()
	go func() { firstResult <- launch("Game", firstRun) }()
	go func() {
		_, _, err := beginSplitLocalRun(ctx, otherComputer, f.project, "Site", secondRun, "res-"+secondRun)
		secondResult <- err
	}()
	earlierErr, laterErr := <-firstResult, <-secondResult
	if !errors.Is(laterErr, ErrRejected) || !strings.Contains(laterErr.Error(), "workspace-cache") {
		t.Fatalf("later task took an explicitly shared lock: %v", laterErr)
	}
	if earlierErr != nil {
		t.Fatal(earlierErr)
	}

	game := f.projects[gameFile]
	if err := game.Close(); err != nil {
		t.Fatal(err)
	}
	game = f.open(t, gameFile)
	f.projects[gameFile] = game
	if err := launch("Site", secondRun); !errors.Is(err, ErrRejected) {
		t.Fatalf("reopening released a resource with an unresolved launch: %v", err)
	}
	if err := (&Store{db: game}).TaskRunNotStarted(firstRun, "fixture: executable refused before launch"); err != nil {
		t.Fatal(err)
	}
	if err := launch("Site", secondRun); err != nil {
		t.Fatalf("definite prelaunch failure did not let the next task proceed: %v", err)
	}
}
