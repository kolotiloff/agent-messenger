package core

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// The task model (D-021, D-025, D-026). A task belongs to a project and runs
// in that project's folder on one computer. Each task has its own worker
// conversation; a question ends a run, and the answer resumes the same
// conversation by exact ID in a new run. A new task always gets a new
// conversation. Nothing is retried automatically.
//
// Task states:
//
//	running           a run is recorded (intent), launching or running
//	waiting_coordinator  the last verified run asked the coordinator
//	waiting_owner     the last verified run asked the owner
//	done              the last verified run reported; never resumed again
//	failed            a run exited cleanly but was invalid (no disposition,
//	                  non-zero exit, provider error, wrong identity)
//	interrupted       the client restarted while a run was launching or
//	                  running; the outcome is unknown
//	blocked           the evidence after a run is unclear (the process group
//	                  is not empty, or it could not be observed)
//	cancelled         cancelled before or between runs, or the cancelled
//	                  run's process was observed to exit
//
// Run states: intent (recorded, nothing launched), launching (committed
// before exec), running (process recorded), then finished, failed,
// interrupted, blocked or cancelled. A run's receipt works only while the run
// is running and its conversation identity is bound.
const taskSchema = `
CREATE TABLE IF NOT EXISTS projects (
 name TEXT PRIMARY KEY,
 created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS project_folders (
 project TEXT NOT NULL REFERENCES projects(name),
 computer TEXT NOT NULL,
 folder TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY (project, computer)
);
CREATE TABLE IF NOT EXISTS computer_agents (
 computer TEXT NOT NULL,
 provider TEXT NOT NULL,
 cli_path TEXT NOT NULL,
 environment TEXT NOT NULL DEFAULT '',
 profile TEXT NOT NULL DEFAULT 'worker',
 updated_at TEXT NOT NULL,
 PRIMARY KEY (computer, provider)
);
CREATE TABLE IF NOT EXISTS tasks (
 id TEXT PRIMARY KEY,
 project TEXT NOT NULL REFERENCES projects(name),
 op_id TEXT NOT NULL,
 computer TEXT NOT NULL,
 provider TEXT NOT NULL,
 cli_path TEXT NOT NULL,
 environment TEXT NOT NULL,
 profile TEXT NOT NULL,
 folder TEXT NOT NULL,
 title TEXT NOT NULL,
 text TEXT NOT NULL,
 state TEXT NOT NULL,
 session_id TEXT,
 detail TEXT NOT NULL DEFAULT '',
 cancel_requested INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 finished_at TEXT,
 UNIQUE (project, op_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS tasks_session ON tasks(provider, environment, session_id) WHERE session_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS task_runs (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 ordinal INTEGER NOT NULL,
 kind TEXT NOT NULL,
 input TEXT NOT NULL,
 receipt TEXT NOT NULL UNIQUE,
 state TEXT NOT NULL,
 reservation TEXT,
 pid INTEGER,
 pgid INTEGER,
 start_token TEXT,
 boot_id TEXT,
 identity_bound INTEGER NOT NULL DEFAULT 0,
 read_seen INTEGER NOT NULL DEFAULT 0,
 exit_observed INTEGER NOT NULL DEFAULT 0,
 exit_code INTEGER,
 group_empty INTEGER NOT NULL DEFAULT 0,
 provider_ok INTEGER NOT NULL DEFAULT 0,
 disposition TEXT,
 disposition_text TEXT,
 detail TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 finished_at TEXT,
 UNIQUE (task_id, ordinal)
);
CREATE TABLE IF NOT EXISTS task_questions (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 run_id TEXT NOT NULL UNIQUE REFERENCES task_runs(id),
 audience TEXT NOT NULL,
 text TEXT NOT NULL,
 state TEXT NOT NULL,
 answer TEXT,
 answer_op TEXT UNIQUE,
 answered_by TEXT,
 created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS task_owner_notes (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 op_id TEXT NOT NULL UNIQUE,
 text TEXT NOT NULL,
 state TEXT NOT NULL,
 run_id TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS task_owner_notes_task ON task_owner_notes(task_id, state);
CREATE TABLE IF NOT EXISTS task_events (
 seq INTEGER PRIMARY KEY AUTOINCREMENT,
 project TEXT NOT NULL,
 task_id TEXT NOT NULL,
 audience TEXT NOT NULL,
 kind TEXT NOT NULL,
 text TEXT NOT NULL,
 question_id TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS task_events_project ON task_events(project, seq);
CREATE TABLE IF NOT EXISTS coordinator_connections (
 project TEXT PRIMARY KEY,
 last_seen TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS coordinator_chats (
 project TEXT PRIMARY KEY,
 chat TEXT NOT NULL,
 since TEXT NOT NULL
);
`

// Bounds on stored text. A report is bounded so wait and the window stay
// responsive; the limit is far above the old 2000-character reply.
const (
	MaxTaskText        = 20000
	MaxDispositionText = 20000
	MaxTaskTitle       = 120
	maxProjectName     = 64
)

// Dispositions a worker run can end with (exactly one per run).
const (
	DispositionReport   = "report"
	DispositionAsk      = "ask"
	DispositionAskOwner = "ask_owner"
)

// Audiences of task events. The coordinator's wait returns only coordinator
// events of its project; owner questions go to the window (D-024).
// AudienceLog holds events the coordinator caused itself (a start, its
// answer); they are history for the window and never wake its wait.
const (
	AudienceCoordinator = "coordinator"
	AudienceOwner       = "owner"
	AudienceLog         = "log"
)

// TaskProviders are the agents a task can run.
var TaskProviders = map[string]bool{"claude": true, "codex": true, "cursor": true}

var projectNamePattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._-]*$`)

// TemporaryFolder reports whether a folder is in a system temporary
// directory, which macOS empties on reboot. Tests replace it.
var TemporaryFolder = func(path string) bool {
	candidates := []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders", "/private/var/folders", "/var/tmp", "/private/var/tmp"}
	for _, dir := range candidates {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			candidates = append(candidates, resolved)
		}
	}
	for _, dir := range candidates {
		dir = filepath.Clean(dir)
		if dir == "" || dir == "/" {
			continue
		}
		if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Task is one row of tasks with its newest run.
type Task struct {
	ID, Project, Computer, Provider, Profile, Folder, Title, Text, State string
	SessionID, Detail, CreatedAt, UpdatedAt, FinishedAt                  string
	CLI, Environment                                                     string
	Model                                                                string
	// FailureReason and FailureReset explain a failed task (T-034): the
	// failure class the provider reported and its limit reset time (RFC 3339).
	// They never cause a retry.
	FailureReason, FailureReset string
	CancelRequested             bool
	// FromNetwork marks a task another computer started (T-037). This
	// computer runs it; only the other computer's coordinator manages it.
	FromNetwork bool
	// FullAccess lifts the agent's own permission checks and sandbox for
	// this task's runs (T-054, D-065), copied from the agent at start.
	FullAccess   bool
	Version      int
	OpenQuestion *TaskQuestion
}

// TaskQuestion is a question that a verified run asked.
type TaskQuestion struct {
	ID       string `json:"id"`
	TaskID   string `json:"task_id"`
	Audience string `json:"audience"`
	Text     string `json:"text"`
	State    string `json:"state"`
}

// TaskRun is one worker run of a task.
type TaskRun struct {
	ID, TaskID, Kind, Input, Receipt, State, Reservation string
	Ordinal                                              int
	Observation                                          platform.Observation
	Disposition, DispositionText, Detail                 string
}

// TaskEvent is one replayable event. Seq is the cursor.
type TaskEvent struct {
	Seq        int64  `json:"seq"`
	TaskID     string `json:"task_id"`
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	QuestionID string `json:"question_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ComputerAgent is an agent program registered on a computer.
type ComputerAgent struct {
	Computer, Provider, CLI, Environment, Profile string
	// Models are the model names the owner listed for this agent (T-043).
	Models []string
}

func randomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// NewSessionUUID returns a random version 4 UUID for a new Claude conversation.
func NewSessionUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

// LocalComputer returns this data directory's computer ID, created once.
func (s *Store) LocalComputer() (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		id, err = localComputer(tx)
		return err
	})
	return id, err
}

func localComputer(tx *sql.Tx) (string, error) {
	var id string
	err := tx.QueryRow(`SELECT value FROM meta WHERE key='computer_id'`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = randomID()
	if err != nil {
		return "", err
	}
	id = "computer-" + id[:12]
	_, err = tx.Exec(`INSERT INTO meta(key,value) VALUES ('computer_id',?)`, id)
	return id, err
}

// ValidProjectName accepts short human names: letters, digits, spaces, dots,
// dashes and underscores.
func ValidProjectName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name && utf8.RuneCountInString(name) <= maxProjectName && projectNamePattern.MatchString(name)
}

// CanonicalFolder resolves a project folder: an existing absolute directory,
// not a temporary one.
func CanonicalFolder(folder string) (string, error) {
	if !filepath.IsAbs(folder) {
		return "", fmt.Errorf("%w: the folder must be an absolute path", ErrRejected)
	}
	resolved, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return "", fmt.Errorf("%w: the folder does not exist", ErrRejected)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: not a folder", ErrRejected)
	}
	if TemporaryFolder(resolved) {
		return "", fmt.Errorf("%w: temporary folders are emptied on reboot; choose another folder", ErrRejected)
	}
	if resolved == "/" {
		return "", fmt.Errorf("%w: choose a project folder, not the disk root", ErrRejected)
	}
	return resolved, nil
}

// CreateProject creates a project with its folder on this computer. Creating
// an existing project with the same folder is a no-op.
func (s *Store) CreateProject(name, folder string) error {
	if !ValidProjectName(name) {
		return fmt.Errorf("%w: invalid project name", ErrRejected)
	}
	folder, err := CanonicalFolder(folder)
	if err != nil {
		return err
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		var existing int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, name).Scan(&existing); err != nil {
			return err
		}
		// A new project may not take a name another project shows (T-065).
		var shown int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name<>? AND removed_at IS NULL AND title<>'' AND lower(title)=lower(?)`, name, name).Scan(&shown); err != nil {
			return err
		}
		if shown != 0 {
			return fmt.Errorf("%w: проект с таким названием уже есть", ErrRejected)
		}
		if existing != 0 {
			var old string
			err := tx.QueryRow(`SELECT folder FROM project_folders WHERE project=? AND computer=?`, name, computer).Scan(&old)
			if err == nil && old == folder {
				return nil
			}
			return fmt.Errorf("%w: a project with this name already exists", ErrRejected)
		}
		at := now()
		// A removed project of the same name comes back with its history.
		res, err := tx.Exec(`UPDATE projects SET removed_at=NULL WHERE name=? AND removed_at IS NOT NULL`, name)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := tx.Exec(`INSERT INTO projects(name,created_at) VALUES (?,?)`, name, at); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`INSERT INTO project_folders(project,computer,folder,updated_at) VALUES (?,?,?,?)`, name, computer, folder, at)
		return err
	})
}

// RemoveProject takes a project out of the window (T-055, D-066). It refuses
// while any task of the project is unfinished or has a run that is not
// closed, so nothing that may still run or wait is hidden. The tasks and
// their runs stay in the database; only the project's folders and its
// coordinator mark are dropped. Creating the project again brings it back.
func (s *Store) RemoveProject(name string) error {
	return s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: unknown project", ErrRejected)
		}
		var open int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE t.project=? AND (t.state IN ('running','waiting_coordinator','waiting_owner','blocked','interrupted')
 OR EXISTS (SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state IN ('intent','launching','running','blocked','interrupted')))`, name).Scan(&open); err != nil {
			return err
		}
		if open != 0 {
			return fmt.Errorf("%w: у проекта есть незавершённые задачи (%d); отмените их или дождитесь конца", ErrRejected, open)
		}
		if _, err := tx.Exec(`UPDATE projects SET removed_at=? WHERE name=?`, now(), name); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM project_folders WHERE project=?`, name); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM coordinator_connections WHERE project=?`, name)
		return err
	})
}

// SetProjectFolder changes the project's folder on this computer. Only new
// tasks use it; each task keeps the folder it recorded (D-025).
func (s *Store) SetProjectFolder(name, folder string) error {
	folder, err := CanonicalFolder(folder)
	if err != nil {
		return err
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: unknown project", ErrRejected)
		}
		_, err = tx.Exec(`INSERT INTO project_folders(project,computer,folder,updated_at) VALUES (?,?,?,?) ON CONFLICT(project,computer) DO UPDATE SET folder=excluded.folder,updated_at=excluded.updated_at`, name, computer, folder, now())
		return err
	})
}

// SetProjectTitle renames a project in the window (T-065). Only the shown
// name changes: tasks, the coordinator chat and other computers keep the name
// the project was created with, so nothing linked to it breaks. A title may
// not repeat another project's name or title; the project's own name clears
// the rename.
func (s *Store) SetProjectTitle(name, title string) error {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxProjectName || !utf8.ValidString(title) || strings.IndexFunc(title, unicode.IsControl) >= 0 {
		return fmt.Errorf("%w: название проекта — от 1 до %d символов, в одну строку", ErrRejected, maxProjectName)
	}
	return s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: unknown project", ErrRejected)
		}
		rows, err := tx.Query(`SELECT name,title FROM projects WHERE name<>? AND removed_at IS NULL`, name)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var other, otherTitle string
			if err := rows.Scan(&other, &otherTitle); err != nil {
				return err
			}
			if strings.EqualFold(other, title) || strings.EqualFold(otherTitle, title) {
				return fmt.Errorf("%w: проект с таким названием уже есть", ErrRejected)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if title == name {
			title = ""
		}
		_, err = tx.Exec(`UPDATE projects SET title=? WHERE name=?`, title, name)
		return err
	})
}

// MaxWorkerRules bounds the owner's rules for a project's workers: they are
// read on every run, so they stay short.
const MaxWorkerRules = 2000

// SetProjectWorkerRules stores the owner's rules for the project's workers
// (T-050). Tasks started afterwards carry them; an empty text removes them.
func (s *Store) SetProjectWorkerRules(name, rules string) error {
	rules = strings.TrimSpace(rules)
	if utf8.RuneCountInString(rules) > MaxWorkerRules || strings.ContainsRune(rules, 0) || !utf8.ValidString(rules) {
		return fmt.Errorf("%w: указания исполнителям — до %d символов", ErrRejected, MaxWorkerRules)
	}
	return s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE projects SET worker_rules=? WHERE name=?`, rules, name)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: unknown project", ErrRejected)
		}
		return nil
	})
}

// ProjectWorkerRules returns the project's worker rules.
func (s *Store) ProjectWorkerRules(name string) (string, error) {
	var rules string
	err := s.db.QueryRow(`SELECT worker_rules FROM projects WHERE name=?`, name).Scan(&rules)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: unknown project", ErrRejected)
	}
	return rules, err
}

// Project is a project with its folder on this computer.
type Project struct {
	Name string `json:"name"`
	// Title is the name shown in the window: the owner's rename, or Name.
	Title  string `json:"title"`
	Folder string `json:"folder"`
}

// Projects lists the projects and their folders on this computer.
func (s *Store) Projects() ([]Project, error) {
	var out []Project
	err := s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT p.name,CASE WHEN p.title='' THEN p.name ELSE p.title END,COALESCE(f.folder,'') FROM projects p LEFT JOIN project_folders f ON f.project=p.name AND f.computer=? WHERE p.removed_at IS NULL ORDER BY p.created_at,p.name`, computer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p Project
			if err := rows.Scan(&p.Name, &p.Title, &p.Folder); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

// SetComputerAgent registers an agent program on this computer. profile is
// "worker" (normal tools, D-023) or "messenger-only" (automated tests).
func (s *Store) SetComputerAgent(provider, cli, environment, profile string) error {
	if !TaskProviders[provider] {
		return fmt.Errorf("%w: unsupported agent", ErrRejected)
	}
	if profile != "worker" && profile != "messenger-only" {
		return fmt.Errorf("%w: unknown profile", ErrRejected)
	}
	cli, err := canonicalCLI(cli)
	if err != nil {
		return fmt.Errorf("%w: the agent program must be an absolute path to an executable", ErrRejected)
	}
	if environment != "" {
		if environment, err = CanonicalEnvironment(environment); err != nil {
			return err
		}
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		// A newly connected worker agent may run any command (owner, D-067);
		// saving it again keeps the owner's later choice.
		full := 0
		if profile == "worker" {
			full = 1
		}
		// Starting model lists, so the coordinator sees them from the start
		// (T-061, T-098). A list the owner saved is never replaced here.
		models := ""
		switch provider {
		case "claude":
			models = strings.Join(DefaultClaudeModels, "\n")
		case "codex":
			models = strings.Join(DefaultCodexModels, "\n")
		case "cursor":
			models = strings.Join(DefaultCursorModels, "\n")
		}
		_, err = tx.Exec(`INSERT INTO computer_agents(computer,provider,cli_path,environment,profile,updated_at,full_access,models) VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(computer,provider) DO UPDATE SET cli_path=excluded.cli_path,environment=excluded.environment,profile=excluded.profile,updated_at=excluded.updated_at`,
			computer, provider, cli, environment, profile, now(), full, models)
		return err
	})
}

// ComputerAgents lists the agents registered on this computer.
func (s *Store) ComputerAgents() ([]ComputerAgent, error) {
	var out []ComputerAgent
	err := s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT computer,provider,cli_path,environment,profile,models FROM computer_agents WHERE computer=? ORDER BY provider`, computer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a ComputerAgent
			var models string
			if err := rows.Scan(&a.Computer, &a.Provider, &a.CLI, &a.Environment, &a.Profile, &models); err != nil {
				return err
			}
			a.Models = splitModels(models)
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

func insertTaskEvent(tx *sql.Tx, project, task, audience, kind, text, question string) error {
	link, err := taskLink(tx, task)
	if err != nil {
		return err
	}
	// The event of a task another computer started is history here: this
	// computer's coordinator neither started nor manages it. The other
	// computer gets it with its real audience.
	sent := audience
	if link.origin != nil {
		audience = AudienceLog
	}
	if _, err := tx.Exec(`INSERT INTO task_events(project,task_id,audience,kind,text,question_id,created_at) VALUES (?,?,?,?,?,?,?)`,
		project, task, audience, kind, text, nullString(sql.NullString{String: question, Valid: question != ""}), now()); err != nil {
		return err
	}
	// The coordinator also learns of a worker's question to the owner and
	// of the owner's answer, for reading only: the owner answers it
	// (T-099, coordinator's item 13). It is recorded where the task is
	// managed, so a task of another computer gets it on that computer.
	if link.origin == nil && audience == AudienceOwner && (kind == "question" || kind == "answered") {
		copyKind := "owner_question"
		if kind == "answered" {
			copyKind = "owner_answered"
		}
		if _, err := tx.Exec(`INSERT INTO task_events(project,task_id,audience,kind,text,question_id,created_at) VALUES (?,?,?,?,?,?,?)`,
			project, task, AudienceCoordinator, copyKind, text, nullString(sql.NullString{String: question, Valid: question != ""}), now()); err != nil {
			return err
		}
	}
	// An event of a task another computer started also goes back to it,
	// in the same transaction (T-037). History events and answers stay
	// here: the other computer records its own.
	if link.origin == nil || sent == AudienceLog || kind == "answered" {
		return nil
	}
	return forwardTaskEvent(tx, *link.origin, task, sent, kind, text, question)
}

func taskTitle(title, text string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	}
	if utf8.RuneCountInString(title) > MaxTaskTitle {
		r := []rune(title)
		title = string(r[:MaxTaskTitle-1]) + "…"
	}
	return title
}

// StartTaskRequest is one start_task call. OpID makes it idempotent: a
// repeated call with the same OpID returns the same task and launches nothing
// new; the same OpID with different arguments is refused (D-022).
type StartTaskRequest struct {
	Project, OpID, Computer, Provider, Title, Text string
	// Model is the worker's model; empty means the CLI's own default.
	Model string
	// Rules are the project's worker rules of the computer that started the
	// task (T-050). StartTask fills them; they travel with a remote task.
	Rules string
	// Lock names a resource such as «Unity editor» (T-072): a task with a
	// lock does not launch while an earlier open task with the same lock
	// runs on the same computer. Empty means no lock.
	Lock string
}

// taskModelPattern accepts model names such as "opus", "claude-sonnet-5-5",
// "gpt-5.1-codex" or "sonnet[1m]". It never starts with "-", so a model can
// not be read as a CLI flag, and has no quotes, spaces or "=".
var taskModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`)

// ValidTaskModel reports whether name may be passed to a worker CLI.
func ValidTaskModel(name string) bool { return name == "" || taskModelPattern.MatchString(name) }

// StartTask records a task and its first run's launch intent in one
// transaction, before anything is launched. The task runs in the project's
// folder on the chosen computer; a computer without a folder for the project,
// or without the agent, refuses it. A task for another computer of the
// network is sent to it (network.go). It returns the task ID and whether it
// was newly created.
func (s *Store) StartTask(r StartTaskRequest) (string, bool, error) {
	r.Text = strings.TrimSpace(r.Text)
	if r.OpID == "" || len(r.OpID) > 128 || r.Text == "" || utf8.RuneCountInString(r.Text) > MaxTaskText {
		return "", false, fmt.Errorf("%w: start_task needs an operation id and a task text of 1..%d characters", ErrRejected, MaxTaskText)
	}
	if !TaskProviders[r.Provider] {
		return "", false, fmt.Errorf("%w: unsupported agent %q", ErrRejected, r.Provider)
	}
	r.Model = strings.TrimSpace(r.Model)
	if !ValidTaskModel(r.Model) {
		return "", false, fmt.Errorf("%w: model must be a plain model name such as opus or gpt-5.1-codex", ErrRejected)
	}
	lock, ok := normalizeLock(r.Lock)
	if !ok {
		return "", false, fmt.Errorf("%w: lock must be a short one-line name such as unity-editor (at most %d characters)", ErrRejected, MaxTaskLock)
	}
	r.Lock = lock
	var id string
	var created bool
	err := s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		if r.Computer == "" {
			r.Computer = computer
		}
		var old struct{ computer, provider, title, text, model, lock string }
		err = tx.QueryRow(`SELECT id,computer,provider,title,text,COALESCE(model,''),resource FROM tasks WHERE project=? AND op_id=?`, r.Project, r.OpID).Scan(&id, &old.computer, &old.provider, &old.title, &old.text, &old.model, &old.lock)
		if err == nil {
			if old.computer != r.Computer || old.provider != r.Provider || old.text != r.Text || old.title != taskTitle(r.Title, r.Text) || old.model != r.Model || old.lock != r.Lock {
				return fmt.Errorf("%w: this operation id was used for a different task", ErrRejected)
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var projects int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, r.Project).Scan(&projects); err != nil {
			return err
		}
		if projects == 0 {
			return fmt.Errorf("%w: unknown project", ErrRejected)
		}
		if err := tx.QueryRow(`SELECT worker_rules FROM projects WHERE name=?`, r.Project).Scan(&r.Rules); err != nil {
			return err
		}
		if r.Computer != computer {
			id, err = startRemoteTask(tx, r)
		} else {
			id, err = startLocalTask(tx, r, computer, nil)
		}
		created = err == nil
		return err
	})
	return id, created, err
}

// startLocalTask records a task of this computer and its first run's launch
// intent. origin is set for a task another computer started (T-037).
func startLocalTask(tx *sql.Tx, r StartTaskRequest, computer string, origin *netLink) (string, error) {
	var folder string
	if err := tx.QueryRow(`SELECT folder FROM project_folders WHERE project=? AND computer=?`, r.Project, computer).Scan(&folder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: this computer has no folder for the project; the owner chooses it in the window", ErrRejected)
		}
		return "", err
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: the project folder is missing on this computer", ErrRejected)
	}
	var agent ComputerAgent
	var models string
	var fullAccess int
	var paused int
	if err := tx.QueryRow(`SELECT cli_path,environment,profile,models,full_access,paused FROM computer_agents WHERE computer=? AND provider=?`, computer, r.Provider).Scan(&agent.CLI, &agent.Environment, &agent.Profile, &models, &fullAccess, &paused); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: agent %s is not set up on this computer", ErrRejected, r.Provider)
		}
		return "", err
	}
	if paused != 0 {
		return "", errAgentPaused(r.Provider)
	}
	if err := modelAllowed(r.Provider, r.Model, splitModels(models)); err != nil {
		return "", err
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	id = "task-" + id[:16]
	var session sql.NullString
	if r.Provider == "claude" {
		// Claude takes a caller-chosen ID, recorded before launch. Codex and
		// Cursor issue their own; it is bound once from the first run's events.
		uuid, err := NewSessionUUID()
		if err != nil {
			return "", err
		}
		session = sql.NullString{String: uuid, Valid: true}
	}
	at := now()
	if _, err := tx.Exec(`INSERT INTO tasks(id,project,op_id,computer,provider,cli_path,environment,profile,folder,title,text,state,session_id,created_at,updated_at,model) VALUES (?,?,?,?,?,?,?,?,?,?,?,'running',?,?,?,?)`,
		id, r.Project, r.OpID, computer, r.Provider, agent.CLI, agent.Environment, agent.Profile, folder, taskTitle(r.Title, r.Text), r.Text, nullString(session), at, at, r.Model); err != nil {
		return "", err
	}
	if fullAccess != 0 && origin != nil {
		// T-094: a task of another computer gets «Любые команды» only when
		// the owner allowed it for network tasks too.
		if on, err := netFullAccess(tx); err != nil {
			return "", err
		} else if !on {
			fullAccess = 0
		}
	}
	if fullAccess != 0 {
		if _, err := tx.Exec(`UPDATE tasks SET full_access=1 WHERE id=?`, id); err != nil {
			return "", err
		}
	}
	if r.Rules != "" {
		if _, err := tx.Exec(`UPDATE tasks SET rules=? WHERE id=?`, r.Rules, id); err != nil {
			return "", err
		}
	}
	if r.Lock != "" {
		if _, err := tx.Exec(`UPDATE tasks SET resource=? WHERE id=?`, r.Lock, id); err != nil {
			return "", err
		}
	}
	if origin != nil {
		if _, err := tx.Exec(`UPDATE tasks SET origin_broker=?,origin_peer=?,origin_task=? WHERE id=?`, origin.broker, origin.peer, origin.task, id); err != nil {
			return "", err
		}
	}
	if err := insertTaskRun(tx, id, 1, "start", r.Text); err != nil {
		return "", err
	}
	return id, insertTaskEvent(tx, r.Project, id, AudienceLog, "started", taskTitle(r.Title, r.Text), "")
}

func insertTaskRun(tx *sql.Tx, task string, ordinal int, kind, input string) error {
	id, err := randomID()
	if err != nil {
		return err
	}
	receipt, err := randomID()
	if err != nil {
		return err
	}
	run := "run-" + id[:16]
	if _, err = tx.Exec(`INSERT INTO task_runs(id,task_id,ordinal,kind,input,receipt,state,created_at) VALUES (?,?,?,?,?,?,'intent',?)`, run, task, ordinal, kind, input, receipt, now()); err != nil {
		return err
	}
	// T-085: what the owner wrote while the task worked goes with the next
	// run of the same conversation, whatever started it.
	if ordinal > 1 {
		_, err = tx.Exec(`UPDATE task_owner_notes SET state='delivered',run_id=? WHERE task_id=? AND state='pending'`, run, task)
	}
	return err
}

const taskColumns = `id,project,computer,provider,profile,folder,title,text,state,COALESCE(session_id,''),detail,created_at,updated_at,COALESCE(finished_at,''),cli_path,environment,cancel_requested,version,COALESCE(model,''),COALESCE(failure_reason,''),COALESCE(failure_reset,''),origin_peer IS NOT NULL,full_access`

func scanTask(scan func(...any) error) (Task, error) {
	var t Task
	var cancel, origin, full int
	err := scan(&t.ID, &t.Project, &t.Computer, &t.Provider, &t.Profile, &t.Folder, &t.Title, &t.Text, &t.State, &t.SessionID, &t.Detail, &t.CreatedAt, &t.UpdatedAt, &t.FinishedAt, &t.CLI, &t.Environment, &cancel, &t.Version, &t.Model, &t.FailureReason, &t.FailureReset, &origin, &full)
	t.CancelRequested, t.FromNetwork, t.FullAccess = cancel != 0, origin != 0, full != 0
	return t, err
}

func openQuestion(tx *sql.Tx, task string) (*TaskQuestion, error) {
	var q TaskQuestion
	err := tx.QueryRow(`SELECT id,task_id,audience,text,state FROM task_questions WHERE task_id=? AND state='open' ORDER BY rowid DESC LIMIT 1`, task).Scan(&q.ID, &q.TaskID, &q.Audience, &q.Text, &q.State)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// TaskByID reads one task with its open question.
func (s *Store) TaskByID(id string) (Task, error) {
	var t Task
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		t, err = scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, id).Scan)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown task", ErrRejected)
		}
		if err != nil {
			return err
		}
		t.OpenQuestion, err = openQuestion(tx, id)
		return err
	})
	return t, err
}

// Tasks lists a project's tasks, newest first. An empty project lists all.
func (s *Store) Tasks(project string, limit int) ([]Task, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var out []Task
	err := s.tx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT `+taskColumns+` FROM tasks WHERE (?='' OR project=?) ORDER BY created_at DESC, rowid DESC LIMIT ?`, project, project, limit)
		if err != nil {
			return err
		}
		for rows.Next() {
			t, err := scanTask(rows.Scan)
			if err != nil {
				rows.Close()
				return err
			}
			out = append(out, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for i := range out {
			if out[i].OpenQuestion, err = openQuestion(tx, out[i].ID); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

const runColumns = `id,task_id,ordinal,kind,input,receipt,state,COALESCE(reservation,''),COALESCE(pid,0),COALESCE(pgid,0),COALESCE(start_token,''),COALESCE(boot_id,''),COALESCE(disposition,''),COALESCE(disposition_text,''),detail`

func scanRun(scan func(...any) error) (TaskRun, error) {
	var r TaskRun
	err := scan(&r.ID, &r.TaskID, &r.Ordinal, &r.Kind, &r.Input, &r.Receipt, &r.State, &r.Reservation, &r.Observation.PID, &r.Observation.PGID, &r.Observation.StartToken, &r.Observation.BootID, &r.Disposition, &r.DispositionText, &r.Detail)
	return r, err
}

// TaskRuns lists a task's runs in order.
func (s *Store) TaskRuns(task string) ([]TaskRun, error) {
	var out []TaskRun
	err := s.tx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT `+runColumns+` FROM task_runs WHERE task_id=? ORDER BY ordinal`, task)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			r, err := scanRun(rows.Scan)
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// TaskRun reads one run.
func (s *Store) TaskRun(id string) (TaskRun, error) {
	var r TaskRun
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		r, err = scanRun(tx.QueryRow(`SELECT `+runColumns+` FROM task_runs WHERE id=?`, id).Scan)
		return err
	})
	return r, err
}

// NextTaskRunIntents lists runs that are recorded but not launched, oldest
// first. Reading them never launches anything.
func (s *Store) NextTaskRunIntents(limit int) ([]string, error) {
	var out []string
	err := s.tx(func(tx *sql.Tx) error {
		// A task with a lock waits while an earlier open task with the same
		// lock is on this computer (T-072).
		rows, err := tx.Query(`SELECT r.id FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE r.state='intent' AND t.state='running'
 AND NOT (t.resource<>'' AND EXISTS (`+lockHolderQuery+`))
 ORDER BY r.created_at,r.rowid LIMIT ?`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}

func runTask(tx *sql.Tx, run string) (TaskRun, Task, error) {
	r, err := scanRun(tx.QueryRow(`SELECT `+runColumns+` FROM task_runs WHERE id=?`, run).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return r, Task{}, fmt.Errorf("%w: unknown run", ErrRejected)
	}
	if err != nil {
		return r, Task{}, err
	}
	t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, r.TaskID).Scan)
	return r, t, err
}

func setRunState(tx *sql.Tx, run, state, detail string, from ...string) error {
	var current string
	if err := tx.QueryRow(`SELECT state FROM task_runs WHERE id=?`, run).Scan(&current); err != nil {
		return err
	}
	if !contains(from, current) {
		return fmt.Errorf("%w: run is %s", ErrRejected, current)
	}
	_, err := tx.Exec(`UPDATE task_runs SET state=?,detail=? WHERE id=?`, state, detail, run)
	return err
}

func setTaskState(tx *sql.Tx, task, state, detail string) error {
	// A task that stops keeps the owner's waiting notes as undelivered;
	// nothing sends them again by itself (T-085).
	switch state {
	case "done", "failed", "cancelled", "blocked", "interrupted":
		if _, err := tx.Exec(`UPDATE task_owner_notes SET state='undelivered' WHERE task_id=? AND state='pending'`, task); err != nil {
			return err
		}
	}
	finished := sql.NullString{}
	if state == "done" || state == "failed" || state == "cancelled" {
		finished = sql.NullString{String: now(), Valid: true}
	}
	_, err := tx.Exec(`UPDATE tasks SET state=?,detail=?,updated_at=?,version=version+1,finished_at=COALESCE(?,finished_at),failure_reason=NULL,failure_reset=NULL WHERE id=?`, state, detail, now(), nullString(finished), task)
	return err
}

// Failure reasons a run can report (T-034). They match the adapters'
// classes; anything else is stored as "other".
var failureReasons = map[string]bool{"usage_limit": true, "rate_limit": true, "overload": true, "sign_in": true, "other": true}

// failedRunWhy says in plain words why a run that ended was not accepted
// (T-091), so the owner knows what to do; the raw evidence follows it.
func failedRunWhy(ev RunEvidence, identity bool, disposition string) string {
	switch {
	case ev.Reason == "sign_in":
		return "the agent is not signed in on this computer"
	case ev.Reason == "usage_limit" || ev.Reason == "rate_limit" || ev.Reason == "overload":
		return "the agent stopped at a provider limit"
	case !ev.ExitObserved:
		return "the agent program's exit was not observed"
	case !identity:
		return "the agent answered in a different conversation than the one Messenger started, so its result cannot be trusted"
	case ev.ExitCode != 0:
		return fmt.Sprintf("the agent program stopped with an error (exit code %d)", ev.ExitCode)
	case !ev.ProviderOK:
		return "the agent did not report a successful end of its work; it may have crashed or been stopped"
	case disposition == "":
		return "the worker ended without sending its report or a question through Messenger (report, ask or ask_owner were never called); with the owner's consent continue_task asks the same conversation to send it"
	}
	return "the run was not accepted"
}

// failureNote is the coordinator line that explains a failure reason. It
// says what to tell the owner; nothing is ever started again by itself.
func failureNote(reason, reset string) string {
	switch reason {
	case "usage_limit":
		if reset != "" {
			return "Reason: usage limit; it resets at " + reset + ". Tell the owner the reset time; do not start this task again before it."
		}
		return "Reason: usage limit; no reset time was reported. Tell the owner; do not start this task again until they decide."
	case "rate_limit":
		return "Reason: rate limit. Tell the owner; do not start this task again by yourself."
	case "overload":
		return "Reason: the provider is overloaded. Tell the owner; do not start this task again by yourself."
	case "sign_in":
		return "Reason: the agent CLI is not signed in on this computer. Ask the owner to sign in."
	}
	return ""
}

// BeginTaskRun is committed BEFORE the reservation and exec. From here a
// crash leaves the run interrupted; it is never launched again.
func (s *Store) BeginTaskRun(run, reservation string) (TaskRun, Task, error) {
	var r TaskRun
	var t Task
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		r, t, err = runTask(tx, run)
		if err != nil {
			return err
		}
		if t.State != "running" {
			return fmt.Errorf("%w: task is %s", ErrRejected, t.State)
		}
		if p, err := isPaused(tx); err != nil || p {
			if err == nil {
				err = ErrPaused
			}
			return err
		}
		if t.CancelRequested {
			if err := setRunState(tx, run, "cancelled", "cancelled before launch", "intent"); err != nil {
				return err
			}
			if err := setTaskState(tx, t.ID, "cancelled", "cancelled before launch"); err != nil {
				return err
			}
			if err := insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "cancelled", "cancelled before the run started", ""); err != nil {
				return err
			}
			return ErrCancelled
		}
		// A follow-up or an answer is a new run the owner or coordinator
		// asked for now: it takes the agent's current «Любые команды», so
		// turning it on helps a task started before (T-056). Without the
		// agent row the task keeps what it had.
		if r.Kind == "answer" || r.Kind == "continue" || r.Kind == "note" {
			var full int
			err := tx.QueryRow(`SELECT full_access FROM computer_agents WHERE computer=? AND provider=?`, t.Computer, t.Provider).Scan(&full)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err == nil && full != 0 && t.FromNetwork {
				on, nerr := netFullAccess(tx)
				if nerr != nil {
					return nerr
				}
				if !on {
					full = 0
				}
			}
			if err == nil && (full != 0) != t.FullAccess {
				if _, err := tx.Exec(`UPDATE tasks SET full_access=? WHERE id=?`, full, t.ID); err != nil {
					return err
				}
				t.FullAccess = full != 0
			}
		}
		if err := setRunState(tx, run, "launching", "process may start; never launched again", "intent"); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE task_runs SET reservation=?, launched_at=? WHERE id=?`, reservation, now(), run)
		r.State, r.Reservation = "launching", reservation
		return err
	})
	return r, t, err
}

// ErrCancelled means the task was cancelled before the run launched.
var ErrCancelled = errors.New("task cancelled")

// TaskRunNotStarted is only for a definite OS refusal before the executable
// ran. The task fails; nothing is launched again automatically.
func (s *Store) TaskRunNotStarted(run, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		r, t, err := runTask(tx, run)
		if err != nil {
			return err
		}
		if r.Observation.PID != 0 {
			return fmt.Errorf("%w: a process was recorded", ErrRejected)
		}
		if err := setRunState(tx, run, "failed", "not started: "+reason, "launching"); err != nil {
			return err
		}
		if err := setTaskState(tx, t.ID, "failed", "the agent program did not start: "+reason); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "failed", "the agent program did not start: "+reason, "")
	})
}

// RecordTaskRunProcess stores the observed process. Tools stay closed until
// the conversation identity is bound.
func (s *Store) RecordTaskRunProcess(run string, obs platform.Observation) error {
	if obs.PID <= 0 || obs.PGID <= 1 || obs.StartToken == "" || obs.BootID == "" {
		return fmt.Errorf("%w: incomplete process observation", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		if err := setRunState(tx, run, "running", "process observed", "launching"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE task_runs SET pid=?,pgid=?,start_token=?,boot_id=? WHERE id=?`, obs.PID, obs.PGID, obs.StartToken, obs.BootID, run)
		return err
	})
}

// SetTaskRunModel records the model a run's CLI reported at its start
// (T-108). It is shown to people only and decides nothing.
func (s *Store) SetTaskRunModel(run, model string) error {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 120 {
		return nil
	}
	_, err := s.db.Exec(`UPDATE tasks SET ran_model=? WHERE id=(SELECT task_id FROM task_runs WHERE id=?)`, model, run)
	return err
}

// BindTaskSession checks the conversation ID the CLI reported. A Claude task
// and every resumed run must match the recorded ID exactly. A new Codex task
// binds the first ID once; an ID already bound to another task is refused.
func (s *Store) BindTaskSession(run, session string) error {
	if session == "" || len(session) > 128 || strings.TrimSpace(session) != session {
		return fmt.Errorf("%w: invalid conversation id", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		r, t, err := runTask(tx, run)
		if err != nil {
			return err
		}
		if r.State != "running" {
			return fmt.Errorf("%w: run is %s", ErrRejected, r.State)
		}
		var bound int
		if err := tx.QueryRow(`SELECT identity_bound FROM task_runs WHERE id=?`, run).Scan(&bound); err != nil {
			return err
		}
		if bound != 0 {
			var current string
			_ = tx.QueryRow(`SELECT COALESCE(session_id,'') FROM tasks WHERE id=?`, t.ID).Scan(&current)
			if current == session {
				return nil
			}
			return fmt.Errorf("%w: a second conversation id was reported", ErrIdentity)
		}
		if t.SessionID == "" {
			if (t.Provider != "codex" && t.Provider != "cursor") || r.Ordinal != 1 {
				return fmt.Errorf("%w: no recorded conversation to resume", ErrIdentity)
			}
			var other int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE provider=? AND environment=? AND session_id=?`, t.Provider, t.Environment, session).Scan(&other); err != nil {
				return err
			}
			if other != 0 {
				return fmt.Errorf("%w: conversation already belongs to another task", ErrIdentity)
			}
			if _, err := tx.Exec(`UPDATE tasks SET session_id=? WHERE id=? AND session_id IS NULL`, session, t.ID); err != nil {
				return err
			}
		} else if t.SessionID != session {
			return fmt.Errorf("%w: expected the recorded conversation", ErrIdentity)
		}
		_, err = tx.Exec(`UPDATE task_runs SET identity_bound=1 WHERE id=?`, run)
		return err
	})
}

// ErrIdentity means the CLI used a different conversation than recorded.
var ErrIdentity = errors.New("conversation identity mismatch")

// taskRunForReceipt finds the run a worker tool call belongs to. Its tools
// work only while the run is running and its identity is bound; a receipt of
// a finished run is refused.
func taskRunForReceipt(tx *sql.Tx, run, receipt string) (TaskRun, Task, error) {
	r, t, err := runTask(tx, run)
	if err != nil {
		return r, t, err
	}
	var bound int
	if err := tx.QueryRow(`SELECT identity_bound FROM task_runs WHERE id=?`, run).Scan(&bound); err != nil {
		return r, t, err
	}
	if receipt == "" || receipt != r.Receipt || r.State != "running" || bound != 1 {
		return r, t, fmt.Errorf("%w: this run's tools are closed", ErrRejected)
	}
	return r, t, nil
}

// TaskBrief is what read_task returns to the worker.
type TaskBrief struct {
	TaskID   string `json:"task_id"`
	Project  string `json:"project"`
	Folder   string `json:"folder"`
	Title    string `json:"title"`
	Task     string `json:"task,omitempty"`
	Answer   string `json:"answer,omitempty"`
	Question string `json:"question,omitempty"`
	Run      int    `json:"run"`
	// Note is set on a resumed run whose task text was left out (T-033).
	Note string `json:"note,omitempty"`
	// Rules are the owner's standing rules for this project's workers
	// (T-050), given on every run.
	Rules string `json:"rules,omitempty"`
	// FollowUp is the coordinator's new request on a run that continues a
	// finished task (T-051).
	FollowUp string `json:"follow_up,omitempty"`
	// OwnerNotes is what the owner wrote while the task worked (T-085);
	// HeldReport is the report that was not sent because of them.
	OwnerNotes string `json:"owner_notes,omitempty"`
	HeldReport string `json:"held_report,omitempty"`
}

// continuedTaskNote tells a continued worker what this run is.
const continuedTaskNote = "This run continues the same conversation after you finished the task. Do what follow_up asks; the task text is the one from your first read_task."

// ownerNoteTaskNote tells a worker why its run goes on after its report.
const ownerNoteTaskNote = "This run continues the same conversation. The owner (or the coordinator, where a note starts with «From the coordinator») wrote to you while you worked (owner_notes), " +
	"so your report (held_report) was not sent yet. Take what the owner wrote into account, then end the run as before " +
	"with exactly one report, ask or ask_owner; the report should cover the whole task."

// ownerNotesAlsoNote is added when owner notes come with an answer or a
// follow-up.
const ownerNotesAlsoNote = "The owner or the coordinator also wrote to you while you worked: see owner_notes."

// resumedTaskNote tells a resumed worker where its task text is.
const resumedTaskNote = "This run continues the same conversation, which already has the task from your first read_task. " +
	"Call read_task with include_task: true only if you no longer have it."

// ReadTask returns the task and, for a resumed run, the question and answer.
// A resumed run continues the same conversation, so the task text is left
// out unless includeTask asks for it (T-033).
func (s *Store) ReadTask(run, receipt string, includeTask bool) (TaskBrief, error) {
	var b TaskBrief
	err := s.tx(func(tx *sql.Tx) error {
		r, t, err := taskRunForReceipt(tx, run, receipt)
		if err != nil {
			return err
		}
		b = TaskBrief{TaskID: t.ID, Project: t.Project, Folder: t.Folder, Title: t.Title, Task: t.Text, Run: r.Ordinal}
		if err := tx.QueryRow(`SELECT rules FROM tasks WHERE id=?`, t.ID).Scan(&b.Rules); err != nil {
			return err
		}
		if r.Kind == "answer" {
			if !includeTask {
				b.Task, b.Note = "", resumedTaskNote
			}
			b.Answer = r.Input
			_ = tx.QueryRow(`SELECT text FROM task_questions WHERE task_id=? AND state='answered' ORDER BY rowid DESC LIMIT 1`, t.ID).Scan(&b.Question)
		}
		if r.Kind == "continue" {
			if !includeTask {
				b.Task = ""
			}
			b.Note, b.FollowUp = continuedTaskNote, r.Input
		}
		if r.Kind == "note" {
			if !includeTask {
				b.Task = ""
			}
			b.Note = ownerNoteTaskNote
			_ = tx.QueryRow(`SELECT COALESCE(disposition_text,'') FROM task_runs WHERE task_id=? AND ordinal=?`, t.ID, r.Ordinal-1).Scan(&b.HeldReport)
		}
		if b.OwnerNotes, err = ownerNotesOf(tx, r.ID); err != nil {
			return err
		}
		if b.OwnerNotes != "" && r.Kind != "note" {
			b.Note = strings.TrimSpace(b.Note + " " + ownerNotesAlsoNote)
		}
		_, err = tx.Exec(`UPDATE task_runs SET read_seen=1 WHERE id=?`, run)
		return err
	})
	return b, err
}

// RecordDisposition stores the run's one terminal disposition provisionally.
// It is published only when the run is verified (FinishTaskRun). A second
// disposition is refused; repeating the identical one is a no-op.
func (s *Store) RecordDisposition(run, receipt, kind, text string) error {
	text = strings.TrimSpace(text)
	if kind != DispositionReport && kind != DispositionAsk && kind != DispositionAskOwner {
		return fmt.Errorf("%w: unknown disposition", ErrRejected)
	}
	if text == "" || utf8.RuneCountInString(text) > MaxDispositionText {
		return fmt.Errorf("%w: text must be 1..%d characters", ErrRejected, MaxDispositionText)
	}
	return s.tx(func(tx *sql.Tx) error {
		r, _, err := taskRunForReceipt(tx, run, receipt)
		if err != nil {
			return err
		}
		var read int
		if err := tx.QueryRow(`SELECT read_seen FROM task_runs WHERE id=?`, run).Scan(&read); err != nil {
			return err
		}
		if read != 1 {
			return fmt.Errorf("%w: call read_task first", ErrRejected)
		}
		if r.Disposition != "" {
			if r.Disposition == kind && r.DispositionText == text {
				return nil
			}
			return fmt.Errorf("%w: this run already ended with %s; only one report or question per run", ErrRejected, r.Disposition)
		}
		_, err = tx.Exec(`UPDATE task_runs SET disposition=?,disposition_text=? WHERE id=?`, kind, text, run)
		return err
	})
}

// RunEvidence is what the supervisor observed after a run's process ended.
type RunEvidence struct {
	ExitObserved bool
	ExitCode     int
	// GroupEmpty: the process group was signalled after the exit and was then
	// observed empty. False when it could not be observed.
	GroupEmpty bool
	ProviderOK bool
	Detail     string
	// Reason is the failure class the provider reported (T-034) and ResetAt
	// its limit reset time in RFC 3339; both are only explanations.
	Reason, ResetAt string
	// Cancelled: the run was stopped on request.
	Cancelled bool
	// LastText is the worker's last message; it is shown only when the run
	// failed without a report (T-056).
	LastText string
	// FinalText is the worker's last message in full (T-087).
	FinalText string
	// Usage is what the run spent, when its provider reported it (T-058).
	// It only informs the owner.
	Usage *TaskUsage
}

// FinishTaskRun verifies a run in one transaction: it revokes the receipt,
// and publishes the disposition only when identity, provider completion, exit
// 0, an empty group and exactly one disposition are all present. A cleanly
// observed but invalid run fails the task; unclear evidence blocks it. It
// returns the run's final state; "finished", "failed" and "cancelled" mean the
// process group was observed empty, so the caller may retire the reservation.
func (s *Store) FinishTaskRun(run string, ev RunEvidence) (string, error) {
	var final string
	err := s.tx(func(tx *sql.Tx) error {
		r, t, err := runTask(tx, run)
		if err != nil {
			return err
		}
		if r.State != "running" && r.State != "launching" {
			return fmt.Errorf("%w: run is %s", ErrRejected, r.State)
		}
		var bound int
		if err := tx.QueryRow(`SELECT identity_bound FROM task_runs WHERE id=?`, run).Scan(&bound); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE task_runs SET exit_observed=?,exit_code=?,group_empty=?,provider_ok=?,finished_at=? WHERE id=?`,
			boolInt(ev.ExitObserved), sql.NullInt64{Int64: int64(ev.ExitCode), Valid: ev.ExitObserved}, boolInt(ev.GroupEmpty), boolInt(ev.ProviderOK), now(), run); err != nil {
			return err
		}
		clean := ev.ExitObserved && ev.GroupEmpty
		valid := clean && bound == 1 && ev.ExitCode == 0 && ev.ProviderOK && r.Disposition != ""
		var detail string
		switch {
		case !clean:
			final = "blocked"
			detail = "process exit or empty process group not observed; nothing is retried"
		case ev.Cancelled:
			final = "cancelled"
			detail = "cancelled; the process exited"
		case valid:
			final = "finished"
			detail = "verified: identity, completion, exit 0, empty group, one " + r.Disposition
		default:
			final = "failed"
			detail = failedRunWhy(ev, bound == 1, r.Disposition) + fmt.Sprintf(" (exit %d, identity %t, provider %t, disposition %q)", ev.ExitCode, bound == 1, ev.ProviderOK, r.Disposition)
		}
		if ev.Detail != "" {
			detail += "; " + ev.Detail
		}
		if final == "failed" && r.Disposition == "" && ev.LastText != "" {
			detail += "; no report; last worker message: " + ev.LastText
		}
		if err := setRunState(tx, run, final, detail, "running", "launching"); err != nil {
			return err
		}
		// T-058: add what the run spent before any event goes out, so a
		// task of another computer reports the new total with it.
		if err := addTaskUsage(tx, t.ID, ev.Usage); err != nil {
			return err
		}
		// T-087: keep the worker's full last message before any event, so
		// a task of another computer sends it ahead of its report.
		if final == "finished" || final == "failed" {
			if err := saveTaskOutput(tx, t.ID, r.Ordinal, ev.FinalText); err != nil {
				return err
			}
		}
		switch final {
		case "finished":
			if held, err := holdForOwnerNotes(tx, t, r); err != nil || held {
				return err
			}
			return publishDisposition(tx, t, r)
		case "cancelled":
			if err := setTaskState(tx, t.ID, "cancelled", "cancelled by request"); err != nil {
				return err
			}
			return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "cancelled", "the task was cancelled; its report, if any, was not published", "")
		case "failed":
			if err := setTaskState(tx, t.ID, "failed", detail); err != nil {
				return err
			}
			reason, reset := ev.Reason, ev.ResetAt
			if !failureReasons[reason] {
				reason = ""
			}
			if _, err := time.Parse(time.RFC3339, reset); err != nil || reason != "usage_limit" {
				reset = ""
			}
			if reason != "" {
				if _, err := tx.Exec(`UPDATE tasks SET failure_reason=?,failure_reset=? WHERE id=?`, reason, nullString(sql.NullString{String: reset, Valid: reset != ""}), t.ID); err != nil {
					return err
				}
			}
			text := "the run was not verified: " + detail
			if note := failureNote(reason, reset); note != "" {
				text += "\n" + note
			}
			if r.DispositionText != "" {
				text += "\nUnverified text from the worker:\n" + r.DispositionText
			}
			if text, err = withOutputNote(tx, t.ID, text); err != nil {
				return err
			}
			if text, err = withUsageNote(tx, t.ID, text); err != nil {
				return err
			}
			return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "failed", bound4000(text), "")
		default:
			if err := setTaskState(tx, t.ID, "blocked", detail); err != nil {
				return err
			}
			return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "blocked", detail, "")
		}
	})
	return final, err
}

func bound4000(text string) string {
	if utf8.RuneCountInString(text) > MaxDispositionText+1000 {
		return string([]rune(text)[:MaxDispositionText+1000])
	}
	return text
}

func publishDisposition(tx *sql.Tx, t Task, r TaskRun) error {
	switch r.Disposition {
	case DispositionReport:
		if err := setTaskState(tx, t.ID, "done", "reported"); err != nil {
			return err
		}
		text, err := withOutputNote(tx, t.ID, r.DispositionText)
		if err != nil {
			return err
		}
		if text, err = withUsageNote(tx, t.ID, text); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "report", text, "")
	case DispositionAsk, DispositionAskOwner:
		audience, state := AudienceCoordinator, "waiting_coordinator"
		if r.Disposition == DispositionAskOwner {
			audience, state = AudienceOwner, "waiting_owner"
		}
		id, err := randomID()
		if err != nil {
			return err
		}
		id = "q-" + id[:16]
		if _, err := tx.Exec(`UPDATE task_questions SET state='superseded' WHERE task_id=? AND state='open'`, t.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO task_questions(id,task_id,run_id,audience,text,state,created_at) VALUES (?,?,?,?,?,'open',?)`, id, t.ID, r.ID, audience, r.DispositionText, now()); err != nil {
			return err
		}
		if err := setTaskState(tx, t.ID, state, "waiting for an answer"); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, audience, "question", r.DispositionText, id)
	}
	return fmt.Errorf("%w: unknown disposition", ErrRejected)
}

// AnswerTask answers the task's open question and records the next run's
// launch intent, which resumes the same conversation. opID makes it
// idempotent: repeating the same answer returns without a second run. The
// question must be the newest open one and addressed to by (coordinator or
// owner); a late or stale answer is refused.
func (s *Store) AnswerTask(task, question, opID, text, by string) error {
	return s.tx(func(tx *sql.Tx) error { return answerTask(tx, task, question, opID, text, by, false, false) })
}

// answerTask answers in tx. fromNetwork is set when the computer that
// started the task sends the answer; nobody else answers such a task.
// ownerOver lets the owner answer a question to the coordinator (T-125):
// the owner comes first, and the coordinator learns of it. The coordinator
// may answer a question to the owner, usually with the owner's own words
// from its chat (T-130).
func answerTask(tx *sql.Tx, task, question, opID, text, by string, fromNetwork, ownerOver bool) error {
	text = strings.TrimSpace(text)
	if opID == "" || len(opID) > 128 || text == "" || utf8.RuneCountInString(text) > MaxTaskText {
		return fmt.Errorf("%w: answer needs an operation id and 1..%d characters", ErrRejected, MaxTaskText)
	}
	if by != AudienceCoordinator && by != AudienceOwner {
		return fmt.Errorf("%w: unknown answerer", ErrRejected)
	}
	{
		var qTask, audience, state string
		var oldOp, oldAnswer sql.NullString
		err := tx.QueryRow(`SELECT task_id,audience,state,answer_op,answer FROM task_questions WHERE id=?`, question).Scan(&qTask, &audience, &state, &oldOp, &oldAnswer)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && qTask != task) {
			return fmt.Errorf("%w: unknown question for this task", ErrRejected)
		}
		if err != nil {
			return err
		}
		if state == "answered" {
			if oldOp.String == opID && oldAnswer.String == text {
				return nil
			}
			return fmt.Errorf("%w: this question was already answered", ErrRejected)
		}
		if state != "open" {
			return fmt.Errorf("%w: this question is no longer open", ErrRejected)
		}
		// The coordinator may answer a question to the owner: the owner often
		// answers it in the coordinator's chat (T-130, D-112). The task shows
		// who answered.
		if audience != by && !(ownerOver && by == AudienceOwner && audience == AudienceCoordinator) && !(by == AudienceCoordinator && audience == AudienceOwner) {
			return fmt.Errorf("%w: this question is for the %s", ErrRejected, audience)
		}
		t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, task).Scan)
		if err != nil {
			return err
		}
		want := "waiting_coordinator"
		if audience == AudienceOwner {
			want = "waiting_owner"
		}
		if t.State != want || t.CancelRequested {
			return fmt.Errorf("%w: task is %s", ErrRejected, t.State)
		}
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		if link.origin != nil && !fromNetwork {
			return fmt.Errorf("%w: another computer started this task; it is answered there", ErrRejected)
		}
		var used int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM task_questions WHERE answer_op=?`, opID).Scan(&used); err != nil {
			return err
		}
		if used != 0 {
			return fmt.Errorf("%w: this operation id was used for another answer", ErrRejected)
		}
		if link.remote != nil {
			return answerRemoteTask(tx, t, *link.remote, question, opID, text, by)
		}
		if t.SessionID == "" {
			return fmt.Errorf("%w: no recorded conversation to resume", ErrRejected)
		}
		var ordinal int
		if err := tx.QueryRow(`SELECT MAX(ordinal) FROM task_runs WHERE task_id=?`, task).Scan(&ordinal); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE task_questions SET state='answered',answer=?,answer_op=?,answered_by=? WHERE id=?`, text, opID, by, question); err != nil {
			return err
		}
		if err := insertTaskRun(tx, task, ordinal+1, "answer", text); err != nil {
			return err
		}
		if err := setTaskState(tx, task, "running", "answered; the same conversation resumes"); err != nil {
			return err
		}
		answeredFor := AudienceLog
		if by == AudienceOwner {
			answeredFor = AudienceOwner
		}
		return insertTaskEvent(tx, t.Project, task, answeredFor, "answered", text, question)
	}
}

// ContinueTask gives a finished task a follow-up: the next run resumes the
// same conversation with it (T-051). Only a done or failed task of this
// computer whose conversation was recorded is continued; a cancelled or
// blocked one is not, and nothing tries to recover a conversation the agent
// no longer has: such a run fails like any other. opID makes it idempotent.
func (s *Store) ContinueTask(task, opID, text, by string) error {
	return s.ContinueTaskChecked(task, opID, text, by, nil)
}

// WorkerGone tells whether the recorded worker of an interrupted run is
// certainly gone, and why; an error means it is not certain (D-108).
type WorkerGone func(platform.Observation) (string, error)

// ContinueTaskChecked is ContinueTask that may also continue an interrupted
// task of this computer (T-118, D-108): only when gone confirms, for every
// interrupted run of the task, that its worker is gone. Those runs are then
// closed as failed, their reservations stay as they are, and the same
// conversation goes on. Anything unclear leaves the task as it was.
func (s *Store) ContinueTaskChecked(task, opID, text, by string, gone WorkerGone) error {
	text = strings.TrimSpace(text)
	if opID == "" || len(opID) > 128 || text == "" || utf8.RuneCountInString(text) > MaxTaskText {
		return fmt.Errorf("%w: continue needs an operation id and 1..%d characters", ErrRejected, MaxTaskText)
	}
	if by != AudienceCoordinator {
		return fmt.Errorf("%w: only the coordinator continues a task", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		if link.remote != nil {
			return continueRemoteTask(tx, task, *link.remote, opID, text)
		}
		return continueTaskTx(tx, task, opID, text, gone, false)
	})
}

// continueTaskTx continues a task that runs on this computer. fromNet is
// set when the computer that started it asked (T-113).
func continueTaskTx(tx *sql.Tx, task, opID, text string, gone WorkerGone, fromNet bool) error {
	var oldTask, oldInput string
	err := tx.QueryRow(`SELECT task_id,input FROM task_runs WHERE op_id=?`, opID).Scan(&oldTask, &oldInput)
	if err == nil {
		if oldTask == task && oldInput == text {
			return nil
		}
		return fmt.Errorf("%w: this operation id was used for another follow-up", ErrRejected)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, task).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: unknown task", ErrRejected)
	}
	if err != nil {
		return err
	}
	interrupted := t.State == "interrupted" && gone != nil
	if t.State != "done" && t.State != "failed" && !interrupted {
		return fmt.Errorf("%w: only a finished task (done or failed) or an interrupted one can be continued; this one is %s", ErrRejected, t.State)
	}
	link, err := taskLink(tx, task)
	if err != nil {
		return err
	}
	if link.remote != nil || (link.origin != nil && !fromNet) {
		return fmt.Errorf("%w: another computer's coordinator manages this task", ErrRejected)
	}
	if interrupted {
		if err := closeInterruptedRuns(tx, task, gone); err != nil {
			return err
		}
	}
	if t.SessionID == "" {
		return fmt.Errorf("%w: no recorded conversation to continue; start a new task", ErrRejected)
	}
	var paused int
	if err := tx.QueryRow(`SELECT paused FROM computer_agents WHERE computer=? AND provider=?`, t.Computer, t.Provider).Scan(&paused); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if paused != 0 {
		return errAgentPaused(t.Provider)
	}
	var open int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM task_runs WHERE task_id=? AND state IN ('intent','launching','running','blocked','interrupted')`, task).Scan(&open); err != nil {
		return err
	}
	if open != 0 {
		return fmt.Errorf("%w: a run of this task is not finished or its outcome is unclear", ErrRejected)
	}
	var ordinal int
	if err := tx.QueryRow(`SELECT MAX(ordinal) FROM task_runs WHERE task_id=?`, task).Scan(&ordinal); err != nil {
		return err
	}
	if err := insertTaskRun(tx, task, ordinal+1, "continue", text); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE task_runs SET op_id=? WHERE task_id=? AND ordinal=?`, opID, task, ordinal+1); err != nil {
		return err
	}
	if err := setTaskState(tx, task, "running", "continued; the same conversation resumes"); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE tasks SET finished_at=NULL WHERE id=?`, task); err != nil {
		return err
	}
	return insertTaskEvent(tx, t.Project, task, AudienceLog, "continued", text, "")
}

// closeInterruptedRuns closes the task's interrupted runs as failed when
// their workers are certainly gone (D-108).
func closeInterruptedRuns(tx *sql.Tx, task string, gone WorkerGone) error {
	rows, err := tx.Query(`SELECT id,COALESCE(pid,0),COALESCE(pgid,0),COALESCE(start_token,''),COALESCE(boot_id,'') FROM task_runs WHERE task_id=? AND state='interrupted'`, task)
	if err != nil {
		return err
	}
	type run struct {
		id  string
		obs platform.Observation
	}
	var runs []run
	for rows.Next() {
		var r run
		if err := rows.Scan(&r.id, &r.obs.PID, &r.obs.PGID, &r.obs.StartToken, &r.obs.BootID); err != nil {
			rows.Close()
			return err
		}
		runs = append(runs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(runs) == 0 {
		return fmt.Errorf("%w: no interrupted run to close", ErrRejected)
	}
	whys := make([]string, len(runs))
	for i, r := range runs {
		why, err := gone(r.obs)
		if err != nil {
			return fmt.Errorf("%w: the interrupted worker may still run (%v); the task stays as it is", ErrRejected, err)
		}
		whys[i] = why
	}
	for i, r := range runs {
		if err := setRunState(tx, r.id, "failed", "interrupted; closed for a follow-up: "+whys[i], "interrupted"); err != nil {
			return err
		}
	}
	return nil
}

// CancelTask cancels a task. A task that is waiting, or whose run is only
// recorded, is cancelled at once. A launched run is stopped by the
// supervisor, which signals its process group; the task is cancelled when the
// exit is observed, or blocked if it cannot be. Finished tasks are refused.
// It returns the resulting state.
func (s *Store) CancelTask(task string) (string, error) {
	var state string
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		state, err = cancelTask(tx, task)
		return err
	})
	return state, err
}

// cancelTask cancels in tx and returns the resulting state.
func cancelTask(tx *sql.Tx, task string) (string, error) {
	var state string
	err := func() error {
		t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, task).Scan)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown task", ErrRejected)
		}
		if err != nil {
			return err
		}
		state = t.State
		if link, err := taskLink(tx, task); err != nil {
			return err
		} else if link.remote != nil && t.State != "cancelled" {
			state, err = cancelRemoteTask(tx, t, *link.remote)
			return err
		}
		switch t.State {
		case "cancelled":
			return nil
		case "waiting_coordinator", "waiting_owner":
			if _, err := tx.Exec(`UPDATE task_questions SET state='superseded' WHERE task_id=? AND state='open'`, task); err != nil {
				return err
			}
			state = "cancelled"
			if err := setTaskState(tx, task, state, "cancelled while waiting for an answer"); err != nil {
				return err
			}
			return insertTaskEvent(tx, t.Project, task, AudienceCoordinator, "cancelled", "the task was cancelled", "")
		case "running":
			if _, err := tx.Exec(`UPDATE tasks SET cancel_requested=1,updated_at=?,version=version+1 WHERE id=?`, now(), task); err != nil {
				return err
			}
			var launched int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM task_runs WHERE task_id=? AND state IN ('launching','running')`, task).Scan(&launched); err != nil {
				return err
			}
			if launched == 0 {
				if _, err := tx.Exec(`UPDATE task_runs SET state='cancelled',detail='cancelled before launch' WHERE task_id=? AND state='intent'`, task); err != nil {
					return err
				}
				state = "cancelled"
				if err := setTaskState(tx, task, state, "cancelled before launch"); err != nil {
					return err
				}
				return insertTaskEvent(tx, t.Project, task, AudienceCoordinator, "cancelled", "the task was cancelled before its run started", "")
			}
			state = "cancelling"
			return nil
		default:
			return fmt.Errorf("%w: task is %s", ErrRejected, t.State)
		}
	}()
	return state, err
}

// CancelRequestedRuns lists launched runs whose task asked to be cancelled.
func (s *Store) CancelRequestedRuns() ([]TaskRun, error) {
	var out []TaskRun
	err := s.tx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT r.id FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE t.cancel_requested=1 AND r.state='running'`)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			r, err := scanRun(tx.QueryRow(`SELECT `+runColumns+` FROM task_runs WHERE id=?`, id).Scan)
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// interruptInheritedTaskRuns runs at startup. A run that was launching or
// running belonged to the previous client: its outcome is unknown, its
// reservation stays, and its task is interrupted. It is never relaunched.
func interruptInheritedTaskRuns(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT r.id,r.task_id,t.project FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE r.state IN ('launching','running')`)
	if err != nil {
		return err
	}
	type item struct{ run, task, project string }
	var items []item
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.run, &i.task, &i.project); err != nil {
			rows.Close()
			return err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, i := range items {
		const why = "the client restarted during this run; the outcome is unknown and nothing is retried"
		if _, err := tx.Exec(`UPDATE task_runs SET state='interrupted',detail=?,finished_at=? WHERE id=?`, why, now(), i.run); err != nil {
			return err
		}
		if err := setTaskState(tx, i.task, "interrupted", why); err != nil {
			return err
		}
		if err := insertTaskEvent(tx, i.project, i.task, AudienceCoordinator, "interrupted", why, ""); err != nil {
			return err
		}
	}
	return nil
}

// InterruptInheritedTaskRuns marks the previous client's runs interrupted.
func (s *Store) InterruptInheritedTaskRuns() error {
	return s.tx(interruptInheritedTaskRuns)
}

// TaskEvents returns a project's events after the cursor for one audience,
// at most limit. Reading never moves anything: a caller that repeats the
// same cursor gets the same events again.
func (s *Store) TaskEvents(project, audience string, after int64, limit int) ([]TaskEvent, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var out []TaskEvent
	cursor := after
	err := s.tx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT seq,task_id,kind,text,COALESCE(question_id,''),created_at FROM task_events WHERE project=? AND audience=? AND seq>? ORDER BY seq LIMIT ?`, project, audience, after, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e TaskEvent
			if err := rows.Scan(&e.Seq, &e.TaskID, &e.Kind, &e.Text, &e.QuestionID, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, e)
			cursor = e.Seq
		}
		return rows.Err()
	})
	return out, cursor, err
}

// TaskListVersion changes whenever a project's task list or its events may
// have changed: a new event, a new task, or any task state change (every such
// update bumps tasks.version). It lets the coordinator skip an unchanged list
// (T-032).
func (s *Store) TaskListVersion(project string) (string, error) {
	var seq sql.NullInt64
	var count, sum int64
	err := s.db.QueryRow(`SELECT (SELECT MAX(seq) FROM task_events WHERE project=?), COUNT(*), COALESCE(SUM(version),0) FROM tasks WHERE project=?`, project, project).Scan(&seq, &count, &sum)
	return fmt.Sprintf("%d.%d.%d", seq.Int64, count, sum), err
}

// LatestTaskEvent returns the newest event sequence number of a project.
func (s *Store) LatestTaskEvent(project string) (int64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(seq) FROM task_events WHERE project=?`, project).Scan(&seq)
	return seq.Int64, err
}

// RetireTaskReservation forgets the run's reservation ID after the caller has
// removed it from the registry. It is only for runs whose exit and empty
// group were observed.
func (s *Store) TaskRunReservationRetired(run string) error {
	return s.tx(func(tx *sql.Tx) error {
		var state string
		if err := tx.QueryRow(`SELECT state FROM task_runs WHERE id=?`, run).Scan(&state); err != nil {
			return err
		}
		if state != "finished" && state != "failed" && state != "cancelled" {
			return fmt.Errorf("%w: run is %s", ErrRejected, state)
		}
		_, err := tx.Exec(`UPDATE task_runs SET detail=detail||'; reservation retired' WHERE id=?`, run)
		return err
	})
}

// TouchCoordinator records that a coordinator chat is connected to the
// project. The window shows how to connect one until this is set.
func (s *Store) TouchCoordinator(project string) error {
	_, err := s.db.Exec(`INSERT INTO coordinator_connections(project,last_seen) VALUES (?,?) ON CONFLICT(project) DO UPDATE SET last_seen=excluded.last_seen`, project, now())
	return err
}

// ClaimCoordinator makes chat the project's active coordinator chat (T-090)
// and returns the chat it replaced, or "". The newest chat that acts is the
// coordinator; the one before is told so by its wait.
func (s *Store) ClaimCoordinator(project, chat string) (string, error) {
	var old string
	err := s.tx(func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT chat FROM coordinator_chats WHERE project=?`, project).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old == chat {
			old = ""
			return nil
		}
		_, err = tx.Exec(`INSERT INTO coordinator_chats(project,chat,since) VALUES (?,?,?) ON CONFLICT(project) DO UPDATE SET chat=excluded.chat,since=excluded.since`, project, chat, now())
		return err
	})
	return old, err
}

// CoordinatorChat returns the project's active coordinator chat, or "".
func (s *Store) CoordinatorChat(project string) (string, error) {
	var chat string
	err := s.db.QueryRow(`SELECT chat FROM coordinator_chats WHERE project=?`, project).Scan(&chat)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return chat, err
}

// CoordinatorLastSeen returns when a coordinator last called this project, or "".
func (s *Store) CoordinatorLastSeen(project string) (string, error) {
	var at string
	err := s.db.QueryRow(`SELECT last_seen FROM coordinator_connections WHERE project=?`, project).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return at, err
}

// AgentSummary is one agent of a computer as list_agents shows it.
type AgentSummary struct {
	Agent   string `json:"agent"`
	Profile string `json:"profile"`
	// Running counts tasks of all projects that run on this agent now.
	Running int `json:"running"`
	// Waiting counts this agent's tasks waiting for an answer, all projects.
	Waiting int `json:"waiting"`
	// Models are the names the owner listed; for Codex and Cursor only
	// these may be passed to start_task (T-043).
	Models []string `json:"models"`
	// Limit is the last usage-limit state the agent reported (T-044).
	Limit *AgentLimit `json:"limit,omitempty"`
	// FullAccess: the owner allowed this agent any command (T-054). Only
	// known for this computer.
	FullAccess bool `json:"full_access,omitempty"`
	// ModelsNote and LimitNote explain an empty models list and a missing
	// limit (T-093).
	ModelsNote string `json:"models_note,omitempty"`
	LimitNote  string `json:"limit_note,omitempty"`
}

// explain fills the notes of an agent with no models or no limit (T-093).
func (a *AgentSummary) explain() {
	if len(a.Models) == 0 && a.Agent != "claude" {
		a.ModelsNote = "the owner listed no models for this agent on that computer (window, agent settings): leave model out to use the agent's own default"
	}
	if a.Limit == nil {
		a.LimitNote = "no limit reported yet: it appears after a worker task of this agent on that computer"
		if a.Agent == "codex" {
			a.LimitNote = "Codex does not report its limit in worker runs; ask the owner if it matters"
		}
	}
}

// ComputerSummary is one computer as list_agents shows it for a project.
type ComputerSummary struct {
	Computer string         `json:"computer"`
	Name     string         `json:"name"`
	This     bool           `json:"this"`
	Online   bool           `json:"online"`
	Folder   string         `json:"folder,omitempty"`
	Agents   []AgentSummary `json:"agents"`
	// Reconnecting: see NetPeer.Reconnecting (T-092).
	Reconnecting bool   `json:"reconnecting,omitempty"`
	Note         string `json:"note,omitempty"`
	// AcceptsTasks and HasProject are another computer's readiness as it
	// reported it (T-070); left out when it did not report them.
	AcceptsTasks *bool `json:"accepts_tasks,omitempty"`
	HasProject   *bool `json:"has_project,omitempty"`
}

// ProjectAgents lists the computers for a project: this one, then the other
// computers of its network with the agents they reported. A computer without
// the project's folder refuses the project's tasks (D-025); for another
// computer that is only known when it answers.
func (s *Store) ProjectAgents(project string) (map[string]any, error) {
	var c ComputerSummary
	err := s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		name, err := computerName(tx)
		if err != nil {
			return err
		}
		c = ComputerSummary{Computer: computer, Name: name, This: true, Online: true, Agents: []AgentSummary{}}
		err = tx.QueryRow(`SELECT folder FROM project_folders WHERE project=? AND computer=?`, project, computer).Scan(&c.Folder)
		if errors.Is(err, sql.ErrNoRows) {
			c.Note = "no folder for this project on this computer; it refuses the project's tasks"
		} else if err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT a.provider,a.profile,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state IN ('waiting_coordinator','waiting_owner')),
 a.models, a.limit_state, a.full_access
 FROM computer_agents a WHERE a.computer=? AND a.paused=0 ORDER BY a.provider`, computer)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a AgentSummary
			var models, limit string
			if err := rows.Scan(&a.Agent, &a.Profile, &a.Running, &a.Waiting, &models, &limit, &a.FullAccess); err != nil {
				return err
			}
			a.Models = splitModels(models)
			a.Limit = limitView(parseAgentLimit(limit), time.Now())
			c.Agents = append(c.Agents, a)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	list := []ComputerSummary{c}
	peers, err := s.NetPeers()
	if err != nil {
		return nil, err
	}
	for _, p := range peers {
		o := ComputerSummary{Computer: p.ID, Name: p.Name, Online: p.Age >= 0 && p.Age <= PeerFresh, Agents: []AgentSummary{},
			Note: "another computer of the network: it runs the task only if its owner allows tasks from other computers and it has this project's folder; otherwise the task fails with its reason"}
		if r := p.Ready; r != nil {
			has := false
			for _, name := range r.Projects {
				has = has || name == project
			}
			accept := r.Accept
			o.AcceptsTasks, o.HasProject = &accept, &has
			switch {
			case !accept:
				o.Note = "its owner turned off tasks from other computers: a task started there fails"
			case !has:
				o.Note = "it has no folder for this project: a task started there fails"
			default:
				o.Note = ""
			}
		}
		for _, a := range p.Agents {
			models := a.Models
			if models == nil {
				models = []string{}
			}
			as := AgentSummary{Agent: a.Agent, Running: a.Running, Waiting: a.Waiting, Models: models, Limit: limitView(a.Limit, time.Now())}
			as.explain()
			o.Agents = append(o.Agents, as)
		}
		if p.Ready == nil {
			o.Note = "it runs an older Messenger that does not say whether it takes tasks or has this project's folder; a task there fails with its reason if not. Its folder path is never shown here."
		}
		if p.Reconnecting {
			o.Reconnecting = true
			o.Note = strings.TrimSpace("reconnecting after a restart: a task for it is accepted and waits on the server until it is back. " + o.Note)
		}
		list = append(list, o)
	}
	return map[string]any{"project": project, "computers": list}, nil
}

// DefaultClaudeModels are the short model names Claude Code itself accepts;
// each always means the newest model of that line. Strongest first.
var DefaultClaudeModels = []string{"opus", "sonnet", "haiku"}

// DefaultCodexModels and DefaultCursorModels are the names the owner chose
// as the starting lists (T-098, D-101); the owner can change them on the
// agent's card.
var (
	DefaultCodexModels  = []string{"gpt-6-luna", "gpt-6.1-terra", "gpt-6.1-sol", "gpt-6-astra"}
	DefaultCursorModels = []string{"composer-2.5-fast", "grok-4.7-high-fast", "claude-opus-5-thinking-high"}
)

// fillDefaultModels gives agents that were set up before T-098 with an
// empty model list the starting list once (T-109, owner's request). A list
// the owner empties later stays empty: the meta key marks it done.
func fillDefaultModels(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var done string
	err = tx.QueryRow(`SELECT value FROM meta WHERE key='default_models_filled'`).Scan(&done)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for provider, models := range map[string][]string{"claude": DefaultClaudeModels, "codex": DefaultCodexModels, "cursor": DefaultCursorModels} {
		if _, err := tx.Exec(`UPDATE computer_agents SET models=? WHERE provider=? AND models=''`, strings.Join(models, "\n"), provider); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES ('default_models_filled','1')`); err != nil {
		return err
	}
	return tx.Commit()
}

// errAgentPaused refuses work for an agent the owner turned off on its card.
func errAgentPaused(provider string) error {
	return fmt.Errorf("%w: agent %s is turned off on this computer by its owner", ErrRejected, provider)
}

// SetAgentPaused turns an agent of this computer off or on from its card
// (T-061). A turned-off agent keeps its settings, takes no new tasks or
// follow-ups and is hidden from the coordinator and the other computers;
// tasks already running go on.
func (s *Store) SetAgentPaused(provider string, paused bool) error {
	v := 0
	if paused {
		v = 1
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE computer_agents SET paused=?,updated_at=? WHERE computer=? AND provider=?`, v, now(), computer, provider)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: agent %s is not set up on this computer", ErrRejected, provider)
		}
		return nil
	})
}

// SetAgentFullAccess turns «Разрешить любые команды» on or off for an agent
// of this computer (T-054, D-065). New tasks of the agent take it; started
// tasks keep what they started with.
func (s *Store) SetAgentFullAccess(provider string, on bool) error {
	v := 0
	if on {
		v = 1
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE computer_agents SET full_access=?,updated_at=? WHERE computer=? AND provider=?`, v, now(), computer, provider)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: agent %s is not set up on this computer", ErrRejected, provider)
		}
		return nil
	})
}

// SetAllAgentsFullAccess turns «Любые команды» on or off for every agent set
// up on this computer at once (T-056). Other computers keep their own setting:
// each computer decides for itself.
func (s *Store) SetAllAgentsFullAccess(on bool) error {
	v := 0
	if on {
		v = 1
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE computer_agents SET full_access=?,updated_at=? WHERE computer=?`, v, now(), computer)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: no agents are set up on this computer", ErrRejected)
		}
		return nil
	})
}

// MaxAgentModels bounds the model names the owner lists for one agent.
const MaxAgentModels = 12

// SetAgentModels records the model names the owner listed for an agent of
// this computer (T-043). Blank and repeated names are dropped; an empty list
// clears it.
func (s *Store) SetAgentModels(provider string, models []string) error {
	clean := []string{}
	seen := map[string]bool{}
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		if !ValidTaskModel(m) {
			return fmt.Errorf("%w: %q is not a plain model name", ErrRejected, m)
		}
		seen[m] = true
		clean = append(clean, m)
	}
	if len(clean) > MaxAgentModels {
		return fmt.Errorf("%w: at most %d models per agent", ErrRejected, MaxAgentModels)
	}
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE computer_agents SET models=?,updated_at=? WHERE computer=? AND provider=?`, strings.Join(clean, "\n"), now(), computer, provider)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: agent %s is not set up on this computer", ErrRejected, provider)
		}
		return nil
	})
}

func splitModels(s string) []string {
	out := []string{}
	for _, m := range strings.Split(s, "\n") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// modelAllowed checks a start_task model against the owner's list. Claude
// takes its own names (opus, sonnet, haiku) and full model names; Codex and
// Cursor take only a listed name, because an unknown one fails the run.
func modelAllowed(provider, model string, listed []string) error {
	if model == "" || provider == "claude" {
		return nil
	}
	for _, m := range listed {
		if m == model {
			return nil
		}
	}
	if len(listed) == 0 {
		return fmt.Errorf("%w: the owner listed no models for %s on this computer; leave model out to use its default", ErrRejected, provider)
	}
	return fmt.Errorf("%w: model %q is not in the owner's list for %s: %s; use one of these or leave model out", ErrRejected, model, provider, strings.Join(listed, ", "))
}

// RemoveComputerAgent disconnects an agent of this computer from the window
// (T-046). Its tasks keep their own copy of the program and profile; an
// agent with a running or waiting task is not removed.
func (s *Store) RemoveComputerAgent(provider string) error {
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		var busy int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE computer=? AND provider=? AND state IN ('running','waiting_coordinator','waiting_owner')`, computer, provider).Scan(&busy); err != nil {
			return err
		}
		if busy > 0 {
			return fmt.Errorf("%w: agent %s has %d unfinished tasks; stop or finish them first", ErrRejected, provider, busy)
		}
		res, err := tx.Exec(`DELETE FROM computer_agents WHERE computer=? AND provider=?`, computer, provider)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: agent %s is not set up on this computer", ErrRejected, provider)
		}
		return nil
	})
}

// NewChatID names one coordinator chat (T-090).
func NewChatID() (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	return "chat-" + id[:16], nil
}
