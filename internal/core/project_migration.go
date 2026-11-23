package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
)

const (
	migrationBackupFile   = "legacy-backup.sqlite"
	migrationComputerFile = "computer.sqlite"
	migrationManifestFile = "manifest.json"
)

// A local task ID is recorded on outgoing task frames, except the acceptance
// or refusal of a remote start. Those rows carry the giver's identity and
// may share an ID with other givers. Frames with no reliable route are kept
// in the computer database for legacy handling.
const migrationOutboxRoutes = `SELECT n.seq, CASE
 WHEN n.kind IN ('accepted','refused') THEN (
  SELECT CASE WHEN COUNT(DISTINCT t.project)=1 THEN MIN(t.project) END FROM tasks t
  WHERE t.origin_peer=n.recipient AND t.origin_task=n.task_id
    AND (t.origin_broker=n.broker OR 'parked:'||t.origin_broker=n.broker))
 WHEN n.kind IN ('start','answer','answer_refused','continue','continue_refused','note','note_refused','cancel','event','progress','ran_model','output','image')
  THEN (SELECT t.project FROM tasks t WHERE t.id=n.task_id)
 END AS project FROM net_outbox n`

type migrationProject struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Cursor  int64  `json:"event_cursor"`
	Tasks   int    `json:"tasks"`
	Pending int    `json:"pending_messages"`
}

type projectMigrationManifest struct {
	Format          int                `json:"format"`
	Backup          string             `json:"legacy_backup"`
	Computer        string             `json:"computer_database"`
	ComputerPending int                `json:"computer_pending_messages"`
	Projects        []migrationProject `json:"projects"`
	Checksums       map[string]string  `json:"sha256"`
}

// StageProjectMigration produces an inspectable set of databases from one
// SQLite backup. It never changes the source, launches workers or activates
// the resulting layout. A missing manifest means publication did not finish.
func StageProjectMigration(ctx context.Context, source, output string) error {
	if output == "" {
		return fmt.Errorf("%w: an output directory is required", ErrRejected)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("publish project migration: %w", os.ErrExist)
	}
	scratch, err := os.MkdirTemp(filepath.Dir(output), ".messenger-migration-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	backup := filepath.Join(scratch, migrationBackupFile)
	if err := copySQLiteDatabase(ctx, source, backup); err != nil {
		return fmt.Errorf("back up the legacy database: %w", err)
	}
	projects, totalPending, err := migrationProjects(ctx, backup)
	if err != nil {
		return err
	}
	manifest := projectMigrationManifest{Format: 1, Backup: migrationBackupFile, Computer: migrationComputerFile, Projects: projects, ComputerPending: totalPending}
	for _, project := range projects {
		destination := filepath.Join(scratch, project.File)
		if err := snapshotProjectDatabase(ctx, backup, project.Name, destination, true); err != nil {
			return fmt.Errorf("stage project %q: %w", project.Name, err)
		}
		if err := verifyStagedProject(ctx, destination, project); err != nil {
			return fmt.Errorf("verify project %q: %w", project.Name, err)
		}
		manifest.ComputerPending -= project.Pending
	}
	computer := filepath.Join(scratch, migrationComputerFile)
	if err := stageComputerDatabase(ctx, backup, computer, manifest); err != nil {
		return fmt.Errorf("stage computer database: %w", err)
	}
	manifest.Checksums, err = migrationChecksums(ctx, scratch, manifest)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(scratch, migrationManifestFile), append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return publishProjectMigration(scratch, output, manifest)
}

func copySQLiteDatabase(ctx context.Context, source, output string) error {
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", sqliteFileURL(source, url.Values{"mode": {"ro"}, "_pragma": {sqliteBusyTimeout}}))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(driver any) error {
		api, ok := driver.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite backup is unavailable")
		}
		backup, err := api.NewBackup(sqliteFileURL(output, nil))
		if err != nil {
			return err
		}
		_, copyErr := backup.Step(-1)
		closeErr := backup.Finish()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func migrationProjects(ctx context.Context, backup string) ([]migrationProject, int, error) {
	db, err := sql.Open("sqlite", sqliteFileURL(backup, url.Values{"mode": {"ro"}}))
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `WITH routes AS (`+migrationOutboxRoutes+`)
 SELECT p.name, (SELECT COUNT(*) FROM tasks WHERE project=p.name),
 COALESCE((SELECT MAX(seq) FROM task_events WHERE project=p.name),0),
 (SELECT COUNT(*) FROM routes WHERE project=p.name)
 FROM projects p ORDER BY p.name`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	projects := []migrationProject{}
	for rows.Next() {
		var project migrationProject
		if err := rows.Scan(&project.Name, &project.Tasks, &project.Cursor, &project.Pending); err != nil {
			return nil, 0, err
		}
		project.File = projectMigrationFile(project.Name)
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	var total int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM net_outbox").Scan(&total)
	return projects, total, err
}

func verifyStagedProject(ctx context.Context, path string, project migrationProject) error {
	db, err := sql.Open("sqlite", sqliteFileURL(path, url.Values{"mode": {"ro"}}))
	if err != nil {
		return err
	}
	defer db.Close()
	var tasks, pending int
	var cursor int64
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM tasks),
 (SELECT COUNT(*) FROM net_outbox), COALESCE((SELECT MAX(seq) FROM task_events),0)`).Scan(&tasks, &pending, &cursor); err != nil {
		return err
	}
	if tasks != project.Tasks || cursor != project.Cursor || pending != project.Pending {
		return fmt.Errorf("task count, cursor or delivery count does not match the source")
	}
	return nil
}

func stageComputerDatabase(ctx context.Context, backup, output string, manifest projectMigrationManifest) error {
	if err := copySQLiteDatabase(ctx, backup, output); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", sqliteFileURL(output, url.Values{"_pragma": {"journal_mode(DELETE)"}}))
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
 CREATE TABLE project_storage_stage (project TEXT PRIMARY KEY REFERENCES projects(name), file TEXT NOT NULL UNIQUE);
 CREATE TABLE task_storage_routes AS
 SELECT id,project,origin_broker,origin_peer,origin_task,origin_computer,remote_broker,remote_peer,remote_task,remote_computer FROM tasks;
 CREATE UNIQUE INDEX task_storage_routes_id ON task_storage_routes(id);
 CREATE INDEX task_storage_routes_origin ON task_storage_routes(origin_broker,origin_peer,origin_task);
 DELETE FROM net_outbox WHERE seq IN (SELECT seq FROM (`+migrationOutboxRoutes+`)
 WHERE project IN (SELECT name FROM projects));
 DELETE FROM feedback WHERE project IN (SELECT name FROM projects);
 DROP TABLE task_images;
 DROP TABLE task_outputs;
 DROP TABLE remote_task_items;
 DROP TABLE task_run_items;
 DROP TABLE task_owner_notes;
 DROP TABLE task_questions;
 DROP TABLE task_events;
 DROP TABLE task_runs;
 DROP TABLE tasks;`); err != nil {
		return err
	}
	for _, project := range manifest.Projects {
		if _, err := tx.ExecContext(ctx, "INSERT INTO project_storage_stage VALUES (?,?)", project.Name, project.File); err != nil {
			return err
		}
	}
	var pending int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM net_outbox").Scan(&pending); err != nil {
		return err
	}
	if pending != manifest.ComputerPending {
		return fmt.Errorf("computer delivery count does not match the staged layout")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Strip unused pages from the computer copy; the source backup remains
	// separately available with all original data.
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return err
	}
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("computer database integrity check failed: %s", check)
	}
	return nil
}

func projectMigrationFile(name string) string {
	digest := sha256.Sum256([]byte(name))
	return "project-" + hex.EncodeToString(digest[:]) + ".sqlite"
}

// VerifyProjectMigration checks a published generation before any consumer
// uses it. Names are fixed or derived locally and must agree with the
// manifest, so verification never follows a path supplied by a manifest.
func VerifyProjectMigration(ctx context.Context, dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, migrationManifestFile))
	if err != nil {
		return err
	}
	var manifest projectMigrationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if manifest.Format != 1 || manifest.Backup != migrationBackupFile || manifest.Computer != migrationComputerFile {
		return fmt.Errorf("unsupported project migration manifest")
	}
	for _, project := range manifest.Projects {
		if !ValidProjectName(project.Name) || project.File != projectMigrationFile(project.Name) {
			return fmt.Errorf("invalid project file in the migration manifest")
		}
	}
	expected, err := migrationChecksums(ctx, dir, manifest)
	if err != nil {
		return err
	}
	if !maps.Equal(expected, manifest.Checksums) {
		return fmt.Errorf("a migration database is incomplete or changed since staging")
	}
	return nil
}

func migrationChecksums(ctx context.Context, dir string, manifest projectMigrationManifest) (map[string]string, error) {
	checksums := map[string]string{}
	for _, name := range manifestDatabaseFiles(manifest) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("duplicate migration file %q", name)
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("migration database %q is not a regular file", name)
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		_ = file.Close()
		if readErr != nil {
			return nil, readErr
		}
		checksums[name] = hex.EncodeToString(hash.Sum(nil))
	}
	return checksums, nil
}

func manifestDatabaseFiles(manifest projectMigrationManifest) []string {
	files := []string{migrationBackupFile, migrationComputerFile}
	for _, project := range manifest.Projects {
		files = append(files, projectMigrationFile(project.Name))
	}
	return files
}

func publishProjectMigration(scratch, output string, manifest projectMigrationManifest) error {
	if err := os.Mkdir(output, 0o700); err != nil {
		return err
	}
	files := manifestDatabaseFiles(manifest)
	// The manifest is always published last. A partial directory after a
	// process crash has no activation marker and must not be consumed.
	files = append(files, migrationManifestFile)
	for index, name := range files {
		if err := os.Link(filepath.Join(scratch, name), filepath.Join(output, name)); err != nil {
			for _, file := range files[:index] {
				_ = os.Remove(filepath.Join(output, file))
			}
			_ = os.Remove(output)
			return err
		}
	}
	return nil
}
