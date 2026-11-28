package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const windowHistorySize = 10

const finishedWindowStates = `('done','failed','cancelled')`

// A task sent in an earlier network belongs to no computer of the current
// one: its peer id there may name another computer now (linkComputer).
const windowHistoryWhere = `project=? AND hidden_at IS NULL AND archive_state<>'removed' AND state IN ` + finishedWindowStates + `
 AND (?='' OR provider=?) AND (?='' OR CASE WHEN remote_peer IS NULL THEN 'local' WHEN remote_broker=? THEN remote_peer ELSE '' END=?)`

// windowHistoryArgs are the arguments of windowHistoryWhere.
func windowHistoryArgs(project, agent, computer, broker string) []any {
	return []any{project, agent, agent, computer, broker, computer}
}

type TaskHistoryPage struct {
	Tasks []WindowTask `json:"tasks"`
	Next  string       `json:"next,omitempty"`
}

func (s *Store) windowHistoryVersion(project, agent, computer, broker string) (string, error) {
	var revision string
	if err := s.db.QueryRow(`SELECT COALESCE((SELECT value FROM meta WHERE key=?),'0')`, taskHistoryEditKey(project)).Scan(&revision); err != nil {
		return "", err
	}
	rows, err := s.db.Query(`SELECT id,version,updated_at,COALESCE(ran_model,'') FROM tasks WHERE `+windowHistoryWhere+`
 ORDER BY created_at DESC,rowid DESC LIMIT ?`, append(windowHistoryArgs(project, agent, computer, broker), windowHistorySize+1)...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hash := sha256.New()
	fmt.Fprintln(hash, revision)
	for rows.Next() {
		var id, updated, model string
		var version int64
		if err := rows.Scan(&id, &version, &updated, &model); err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%q %d %q %q\n", id, version, updated, model)
	}
	return hex.EncodeToString(hash.Sum(nil)), rows.Err()
}

// WindowTaskHistory reads completed cards older than before. The saved
// position also works if that card was hidden between requests.
func (s *Store) WindowTaskHistory(project, before, agent, computer string) (TaskHistoryPage, error) {
	broker, err := currentBroker(s.db)
	if err != nil {
		return TaskHistoryPage{}, err
	}
	scoped, err := s.projectStore(project)
	if err != nil {
		return TaskHistoryPage{}, err
	}
	return scoped.windowTaskHistory(project, before, agent, computer, broker)
}

func (s *Store) windowTaskHistory(project, before, agent, computer, broker string) (TaskHistoryPage, error) {
	where := `WHERE ` + windowHistoryWhere
	args := windowHistoryArgs(project, agent, computer, broker)
	if before != "" {
		where += ` AND (created_at,rowid)<(SELECT created_at,rowid FROM tasks WHERE project=? AND id=?)`
		args = append(args, project, before)
	}
	rows, err := s.windowTaskRows(where+` ORDER BY created_at DESC,rowid DESC LIMIT ?`, append(args, windowHistorySize+1)...)
	page := TaskHistoryPage{Tasks: rows}
	if len(rows) > windowHistorySize {
		page.Tasks = rows[:windowHistorySize]
		page.Next = page.Tasks[len(page.Tasks)-1].ID
	}
	return page, err
}

// HideWindowHistory clears the completed list for the selected filter in
// one operation, including cards that have not been fetched by the window.
func (s *Store) HideWindowHistory(project, agent, computer string) error {
	broker, err := currentBroker(s.db)
	if err != nil {
		return err
	}
	scoped, err := s.projectStore(project)
	if err != nil {
		return err
	}
	_, err = scoped.db.Exec(`UPDATE tasks SET hidden_at=? WHERE archive_state='' AND `+windowHistoryWhere, append([]any{now()}, windowHistoryArgs(project, agent, computer, broker)...)...)
	return err
}
