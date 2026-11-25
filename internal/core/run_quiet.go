package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const TaskEventWorkerStillRunning = "worker_still_running"
const TaskEventWorkerQuiet = "worker_quiet"

// T-172: a worker can write its final message and still not end: a child
// it left (a diff window, a server) or the CLI itself stays up and keeps the
// process group. The run then stays running, notes wait for its end, and
// nobody can tell why. These records only describe such a run. They never
// close it: by D-021 a run ends only on an observed exit, an empty group and
// a revoked receipt (Q-030 keeps the cautious answer).

// RunProcess is one process of a running worker's group.
type RunProcess struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	// Worker: the worker CLI process Messenger started.
	Worker bool `json:"worker,omitempty"`
}

// RunQuiet is what is known about a running task's live run.
type RunQuiet struct {
	// OutputAt is when the worker last wrote output, to the nearest
	// runOutputStep.
	OutputAt string `json:"output_at,omitempty"`
	// FinalAt is when the worker wrote its final message (its CLI said the
	// turn ended) while its process kept running.
	FinalAt string `json:"final_at,omitempty"`
	// Processes are the processes of its group, seen at ProcessesAt, once
	// the final message is out and the run still has not ended.
	Processes   []RunProcess `json:"processes,omitempty"`
	ProcessesAt string       `json:"processes_at,omitempty"`
}

// runOutputStep is how often the last-output time is written while a
// worker writes, so a talkative worker does not write on every line.
const runOutputStep = 30 * time.Second

// maxRunProcesses bounds the recorded processes of one run.
const maxRunProcesses = 16

// RunOutputStep is runOutputStep for the supervisor.
func RunOutputStep() time.Duration { return runOutputStep }

// NoteRunOutput records that a running worker wrote output now.
func (s *Store) NoteRunOutput(run string) error {
	if s.split != nil {
		project, err := s.storeForRun(run)
		if err != nil {
			return err
		}
		return project.NoteRunOutput(run)
	}
	_, err := s.db.Exec(`UPDATE task_runs SET output_at=? WHERE id=? AND state='running'`, now(), run)
	return err
}

// NoteRunFinal records the worker's final message, once per run.
func (s *Store) NoteRunFinal(run string) error {
	if s.split != nil {
		project, err := s.storeForRun(run)
		if err != nil {
			return err
		}
		return project.NoteRunFinal(run)
	}
	_, err := s.db.Exec(`UPDATE task_runs SET final_at=COALESCE(final_at,?) WHERE id=? AND state='running'`, now(), run)
	return err
}

// NotifyCodexStillRunning is called once by the run's watcher after it
// observes the same Codex process still alive after the exit grace period.
func (s *Store) NotifyCodexStillRunning(run string) error {
	return s.notifyCodexRun(run, TaskEventWorkerStillRunning, true, "finished its turn, but the worker process is still running. No report has been published and the task still holds its locks.")
}

// NotifyCodexQuiet describes a live run whose turn has not ended.
func (s *Store) NotifyCodexQuiet(run string, elapsed time.Duration) error {
	return s.notifyCodexRun(run, TaskEventWorkerQuiet, false, fmt.Sprintf("is still running. No new output was observed for %s and no running command is tracked. The task still holds its locks.", elapsed.Round(time.Second)))
}

func (s *Store) notifyCodexRun(run, kind string, completed bool, detail string) error {
	store, err := s.storeForRun(run)
	if err != nil {
		return err
	}
	var name string
	if s.split != nil {
		name, err = s.ComputerName()
		if err != nil {
			return err
		}
	}
	return store.tx(func(tx *sql.Tx) error {
		var task, project string
		err := tx.QueryRow(`SELECT t.id,t.project FROM task_runs r JOIN tasks t ON t.id=r.task_id
WHERE r.id=? AND r.state='running' AND (r.final_at IS NOT NULL)=?
AND t.state='running' AND t.provider='codex' AND t.cancel_requested=0`, run, completed).Scan(&task, &project)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if s.split == nil {
			name, err = computerName(tx)
			if err != nil {
				return err
			}
		}
		return insertTaskEvent(tx, project, task, AudienceCoordinator, kind, fmt.Sprintf("Codex on %s %s", name, detail), "")
	})
}

// NoteRunProcesses records the processes still in a running worker's group.
func (s *Store) NoteRunProcesses(run string, procs []RunProcess) error {
	if s.split != nil {
		project, err := s.storeForRun(run)
		if err != nil {
			return err
		}
		return project.NoteRunProcesses(run, procs)
	}
	if len(procs) > maxRunProcesses {
		procs = procs[:maxRunProcesses]
	}
	clean := make([]RunProcess, 0, len(procs))
	for _, p := range procs {
		clean = append(clean, RunProcess{PID: p.PID, Name: clip(p.Name, 80), Worker: p.Worker})
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE task_runs SET processes=?, processes_at=? WHERE id=? AND state='running'`, string(raw), now(), run)
	return err
}

// TaskRunQuiet describes the running run of a task of this computer, or nil
// when it has none or nothing is recorded about it yet.
func (s *Store) TaskRunQuiet(task string) (*RunQuiet, error) {
	if s.split != nil {
		project, err := s.storeForTask(task)
		if err != nil {
			return nil, err
		}
		return project.TaskRunQuiet(task)
	}
	var output, final, procs, procsAt sql.NullString
	err := s.db.QueryRow(`SELECT output_at,final_at,processes,processes_at FROM task_runs WHERE task_id=? AND state='running' ORDER BY ordinal DESC LIMIT 1`, task).Scan(&output, &final, &procs, &procsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	q := &RunQuiet{OutputAt: output.String, FinalAt: final.String}
	if final.String != "" && procs.String != "" {
		if json.Unmarshal([]byte(procs.String), &q.Processes) == nil {
			q.ProcessesAt = procsAt.String
		}
	}
	if q.OutputAt == "" && q.FinalAt == "" {
		return nil, nil
	}
	return q, nil
}
