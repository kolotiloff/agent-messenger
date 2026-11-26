package core

import (
	"context"
	"database/sql"
)

// ProjectActivity is the sidebar snapshot for an inactive project.
type ProjectActivity struct {
	Project     string            `json:"project"`
	Tasks       []WindowTask      `json:"tasks"`
	Shared      []NetPeerTasks    `json:"shared"`
	Coordinator windowCoordinator `json:"coordinator"`
}

func (s *Store) ProjectActivity(project string) (ProjectActivity, error) {
	out := ProjectActivity{Project: project}
	scoped, err := s.projectStore(project)
	if err != nil {
		return out, err
	}
	out.Tasks, err = scoped.windowTaskRows(`WHERE project=? AND hidden_at IS NULL AND state IN ('running','waiting_owner','waiting_coordinator')
 ORDER BY created_at DESC,rowid DESC`, project)
	if err != nil {
		return out, err
	}
	err = s.tx(func(tx *sql.Tx) error {
		var err error
		if out.Coordinator, err = readWindowCoordinator(tx, project); err != nil {
			return err
		}
		out.Shared, err = projectNetTasks(tx, project, s.split != nil)
		return err
	})
	if err == nil {
		out.Coordinator.checkActive()
	}
	return out, err
}

// splitWindowAgents combines machine registration with only the selected
// project's work. Global availability and worker presence are read by
// separate network operations.
func (s *Store) splitWindowAgents(machine *sql.Tx, project, today, computer string) ([]WindowAgent, error) {
	rows, err := machine.Query(`SELECT provider,cli_path,profile,environment,models,limit_state,full_access,paused
 FROM computer_agents WHERE computer=? ORDER BY provider`, computer)
	if err != nil {
		return nil, err
	}
	agents := []WindowAgent{}
	byName := map[string]int{}
	for rows.Next() {
		var a WindowAgent
		var models, limit string
		if err := rows.Scan(&a.Agent, &a.CLI, &a.Profile, &a.Env, &models, &limit, &a.FullAccess, &a.Paused); err != nil {
			rows.Close()
			return nil, err
		}
		a.Models, a.Limit = splitModels(models), parseAgentLimit(limit)
		byName[a.Agent] = len(agents)
		agents = append(agents, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if project == "" {
		return agents, nil
	}
	file, err := splitTaskProjectFile(context.Background(), machine, project)
	if err != nil {
		return nil, err
	}
	db, err := s.split.open(file)
	if err != nil {
		return nil, err
	}
	counts, err := db.Query(`SELECT provider,COUNT(CASE WHEN state='running' THEN 1 END),
 COUNT(CASE WHEN state='waiting_owner' THEN 1 END),COUNT(CASE WHEN created_at>=? THEN 1 END),
 COALESCE(SUM(CASE WHEN created_at>=? THEN usage_input+usage_output END),0),
 COALESCE(MAX(CASE WHEN state='done' THEN finished_at END),'')
 FROM tasks WHERE computer=? AND project=? GROUP BY provider`, today, today, computer, project)
	if err != nil {
		return nil, err
	}
	defer counts.Close()
	for counts.Next() {
		var provider string
		var a WindowAgent
		if err := counts.Scan(&provider, &a.ProjectRunning, &a.ProjectWaiting, &a.ProjectToday, &a.TodayTokens, &a.LastDone); err != nil {
			return nil, err
		}
		if i, ok := byName[provider]; ok {
			agents[i].Running, agents[i].Waiting = a.ProjectRunning, a.ProjectWaiting
			agents[i].ProjectRunning, agents[i].ProjectWaiting = a.ProjectRunning, a.ProjectWaiting
			agents[i].ProjectToday, agents[i].TodayTokens, agents[i].LastDone = a.ProjectToday, a.TodayTokens, a.LastDone
		}
	}
	return agents, counts.Err()
}

type taskAgentActivity struct{ Running, Waiting, WaitingOwner int }

func (s *Store) splitAgentActivity(machine *sql.Tx, project, computer string) (map[string]taskAgentActivity, error) {
	activity := map[string]taskAgentActivity{}
	query := `SELECT provider,COUNT(CASE WHEN state='running' THEN 1 END),
 COUNT(CASE WHEN state IN ('waiting_owner','waiting_coordinator') THEN 1 END),
 COUNT(CASE WHEN state='waiting_owner' THEN 1 END)
 FROM tasks WHERE computer=? AND state IN ('running','waiting_owner','waiting_coordinator') GROUP BY provider`
	err := s.visitProjectTaskRows(machine, project, query, []any{computer}, func(rows *sql.Rows) error {
		for rows.Next() {
			var name string
			var counts taskAgentActivity
			if err := rows.Scan(&name, &counts.Running, &counts.Waiting, &counts.WaitingOwner); err != nil {
				return err
			}
			prior := activity[name]
			prior.Running += counts.Running
			prior.Waiting += counts.Waiting
			prior.WaitingOwner += counts.WaitingOwner
			activity[name] = prior
		}
		return rows.Err()
	})
	return activity, err
}
