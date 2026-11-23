package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const windowHistorySize = 10

const finishedWindowStates = `('done','failed','cancelled')`

const windowHistoryWhere = `project=? AND hidden_at IS NULL AND state IN ` + finishedWindowStates + `
 AND (?='' OR provider=?) AND (?='' OR COALESCE(remote_peer,'local')=?)`

type TaskHistoryPage struct {
	Tasks []WindowTask `json:"tasks"`
	Next  string       `json:"next,omitempty"`
}

func (s *Store) windowHistoryVersion(project, agent, computer string) (string, error) {
	rows, err := s.db.Query(`SELECT id,version,updated_at,COALESCE(ran_model,'') FROM tasks WHERE `+windowHistoryWhere+`
 ORDER BY created_at DESC,rowid DESC LIMIT ?`, project, agent, agent, computer, computer, windowHistorySize+1)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hash := sha256.New()
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
	if s.split != nil {
		scoped, err := s.projectStore(project)
		if err != nil {
			return TaskHistoryPage{}, err
		}
		return scoped.WindowTaskHistory(project, before, agent, computer)
	}
	where := `WHERE ` + windowHistoryWhere
	args := []any{project, agent, agent, computer, computer}
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
	scoped, err := s.projectStore(project)
	if err != nil {
		return err
	}
	_, err = scoped.db.Exec(`UPDATE tasks SET hidden_at=? WHERE `+windowHistoryWhere, now(), project, agent, agent, computer, computer)
	return err
}
