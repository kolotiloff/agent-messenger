package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const agentModelsEvent = "agent_models_updated"
const agentModelsRequestTTL = 2 * time.Minute

const netModelsSchema = `
CREATE TABLE IF NOT EXISTS net_model_changes (
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 agent TEXT NOT NULL,
 id TEXT NOT NULL UNIQUE,
 project TEXT NOT NULL,
 expires TEXT NOT NULL,
 PRIMARY KEY(broker,peer,agent)
);`

// RequestAgentModels applies a local change or queues a remote request.
// A remote change is confirmed only by the designated recipient.
func (s *Store) RequestAgentModels(project, computer, provider string, models []string) (string, error) {
	if models == nil || len(models) > MaxAgentModels {
		return "", fmt.Errorf("%w: supply an explicit models array with up to %d entries", ErrRejected, MaxAgentModels)
	}
	list, err := normalizeAgentModels(provider, models)
	if err != nil {
		return "", err
	}
	var id string
	err = s.tx(func(tx *sql.Tx) error {
		self, err := localComputer(tx)
		if err != nil {
			return err
		}
		if computer == "" || computer == self {
			return setAgentModels(tx, provider, list)
		}
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		peer, ok, err := netPeer(tx, computer)
		if err != nil {
			return err
		}
		if broker == "" || !ok || peer.Age < 0 || peer.Age > PeerFresh || peer.Reconnecting {
			return fmt.Errorf("%w: choose an online computer from list_agents", ErrRejected)
		}
		build, _ := strconv.Atoi(strings.TrimPrefix(peer.Version, "0.0.0-preview."))
		if build < 82 && peer.Version != "dev" {
			return fmt.Errorf("%w: %s needs Messenger 0.82 or newer", ErrRejected, peer.Name)
		}
		if peer.Ready == nil || !peer.Ready.Accept {
			return fmt.Errorf("%w: %s does not accept remote work", ErrRejected, peer.Name)
		}
		if !ValidProjectName(project) || !hasModelChangeProject(peer.Ready, project) {
			return fmt.Errorf("%w: %s does not have this project", ErrRejected, peer.Name)
		}
		if _, err := tx.Exec(`DELETE FROM net_model_changes WHERE julianday(expires)<=julianday(?)`, now()); err != nil {
			return err
		}
		for _, agent := range peer.Agents {
			if agent.Agent != provider {
				continue
			}
			random, err := randomID()
			if err != nil {
				return err
			}
			id = "models-" + random
			expires := time.Now().Add(agentModelsRequestTTL).UTC().Format(time.RFC3339)
			result, err := tx.Exec(`INSERT INTO net_model_changes(broker,peer,agent,id,project,expires) VALUES (?,?,?,?,?,?) ON CONFLICT(broker,peer,agent) DO NOTHING`,
				broker, computer, provider, id, project, expires)
			if err != nil {
				return err
			}
			if n, _ := result.RowsAffected(); n == 0 {
				return fmt.Errorf("%w: a change for this agent is already awaiting confirmation", ErrRejected)
			}
			return enqueueNet(tx, broker, computer, id, id, netBody{
				Kind: "models_request", Task: id, Project: project, Agent: provider,
				Text: list, Detail: strings.Join(agent.Models, "\n"), Reset: expires,
			})
		}
		return fmt.Errorf("%w: %s has no active %s agent", ErrRejected, peer.Name, provider)
	})
	return id, err
}

func hasModelChangeProject(ready *NetReady, project string) bool {
	for _, name := range ready.Projects {
		if name == project {
			return true
		}
	}
	return false
}

func applyModelChange(tx *sql.Tx, from netLink, request netBody) error {
	reason := checkModelChange(tx, request)
	if reason != nil && !errors.Is(reason, ErrRejected) {
		return reason
	}
	reply := netBody{Kind: "models_result", Task: from.task, Agent: request.Agent, State: "applied"}
	if reason != nil {
		reply.State = "refused"
		reply.Text = clip(refusalText(reason), 300)
	}
	return enqueueNet(tx, from.broker, from.peer, "result."+from.task, from.task, reply)
}

func checkModelChange(tx *sql.Tx, request netBody) error {
	expires, err := time.Parse(time.RFC3339, request.Reset)
	at := time.Now()
	if err != nil || !expires.After(at) || expires.After(at.Add(3*time.Minute)) {
		return fmt.Errorf("%w: request expired or the computers' clocks disagree", ErrRejected)
	}
	accept, err := metaValue(tx, "net_accept")
	if err != nil {
		return err
	}
	if accept == "0" {
		return fmt.Errorf("%w: this computer does not accept remote work", ErrRejected)
	}
	self, err := localComputer(tx)
	if err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects p JOIN project_folders f ON f.project=p.name WHERE p.name=? AND p.removed_at IS NULL AND f.computer=?)`, request.Project, self).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: this computer does not have the project", ErrRejected)
	}
	list, err := normalizeAgentModels(request.Agent, splitModels(request.Text))
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE computer_agents SET models=?,updated_at=? WHERE computer=? AND provider=? AND paused=0 AND models=?`,
		list, now(), self, request.Agent, request.Detail)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: the agent is unavailable or its model list changed; refresh list_agents", ErrRejected)
	}
	return nil
}

func finishModelChange(tx *sql.Tx, from netLink, state, reason string) error {
	if state != "applied" && state != "refused" {
		return nil
	}
	var project, agent string
	err := tx.QueryRow(`DELETE FROM net_model_changes WHERE id=? AND broker=? AND peer=? RETURNING project,agent`,
		from.task, from.broker, from.peer).Scan(&project, &agent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	message := fmt.Sprintf("%s model list on %s: %s (request %s)", agent, peerName(tx, from.peer), state, from.task)
	if reason != "" {
		message += ". " + clip(reason, 300)
	}
	return insertCoordinatorNotice(tx, project, agentModelsEvent, message)
}
