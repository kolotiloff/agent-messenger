package core

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Version requests share the task mailbox. Claim an inbound request before
// probing, so an interrupted probe is never repeated automatically.
const netAgentVersionsSchema = `
CREATE TABLE IF NOT EXISTS net_agent_versions (
 id TEXT PRIMARY KEY,
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 agent TEXT NOT NULL,
 version TEXT NOT NULL DEFAULT '',
 reason TEXT NOT NULL DEFAULT '',
 checked_at TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_agent_version_requests (
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 id TEXT NOT NULL,
 agent TEXT NOT NULL,
 claimed INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 PRIMARY KEY (broker, peer, id)
);`

type AgentVersion struct {
	ID        string `json:"request_id,omitempty"`
	Computer  string `json:"computer"`
	Agent     string `json:"agent"`
	State     string `json:"state"`
	Version   string `json:"version,omitempty"`
	Reason    string `json:"reason,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

type AgentVersionRequest struct {
	Broker, Peer, ID, Agent string
}

func versionRequestCutoff(age time.Duration) string {
	return time.Now().UTC().Add(-age).Format(time.RFC3339Nano)
}

func (s *Store) RequestAgentVersion(computer, agent string) (string, error) {
	if !TaskProviders[agent] {
		return "", fmt.Errorf("%w: unsupported agent", ErrRejected)
	}
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		peer, ok, err := netPeer(tx, computer)
		if err != nil {
			return err
		}
		if broker == "" || !ok {
			return fmt.Errorf("%w: select a computer id from list_agents", ErrRejected)
		}
		if peer.Age < 0 || peer.Age > PeerFresh {
			return fmt.Errorf("%w: the other computer is offline", ErrRejected)
		}
		if _, err := tx.Exec(`DELETE FROM net_agent_versions WHERE created_at<?`, versionRequestCutoff(24*time.Hour)); err != nil {
			return err
		}
		id, err = randomID()
		if err != nil {
			return err
		}
		id = "version-" + id
		if _, err := tx.Exec(`INSERT INTO net_agent_versions(id,broker,peer,agent,created_at) VALUES (?,?,?,?,?)`, id, broker, computer, agent, now()); err != nil {
			return err
		}
		return enqueueNet(tx, broker, computer, id, id, netBody{Kind: "agent_version_request", Task: id, Agent: agent})
	})
	return id, err
}

func (s *Store) AgentVersion(id string) (AgentVersion, error) {
	var v AgentVersion
	var peer string
	err := s.tx(func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT id,peer,agent,version,reason,checked_at FROM net_agent_versions WHERE id=?`, id).
			Scan(&v.ID, &peer, &v.Agent, &v.Version, &v.Reason, &v.CheckedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown version request", ErrRejected)
		}
		if err == nil {
			v.Computer = peerName(tx, peer)
		}
		return err
	})
	switch {
	case v.Version != "":
		v.State = "reported"
	case v.Reason != "":
		v.State = "unknown"
	default:
		v.State = "waiting"
	}
	return v, err
}

func (s *Store) ClaimAgentVersionRequest(broker string) (AgentVersionRequest, error) {
	r := AgentVersionRequest{Broker: broker}
	err := s.tx(func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT peer,id,agent FROM net_agent_version_requests
 WHERE broker=? AND claimed=0 AND created_at>=? ORDER BY created_at,id LIMIT 1`, broker, versionRequestCutoff(time.Minute)).
			Scan(&r.Peer, &r.ID, &r.Agent)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE net_agent_version_requests SET claimed=1 WHERE broker=? AND peer=? AND id=?`, r.Broker, r.Peer, r.ID)
		return err
	})
	return r, err
}

func (s *Store) ReplyAgentVersion(r AgentVersionRequest, version, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		return enqueueNet(tx, r.Broker, r.Peer, "reply."+r.ID, r.ID,
			netBody{Kind: "agent_version", Task: r.ID, Agent: r.Agent, Text: version, Reason: reason, CheckedAt: now()})
	})
}

func applyAgentVersionRequest(tx *sql.Tx, from netLink, b netBody) error {
	if !TaskProviders[b.Agent] {
		return nil
	}
	if _, err := tx.Exec(`DELETE FROM net_agent_version_requests WHERE created_at<?`, versionRequestCutoff(24*time.Hour)); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO net_agent_version_requests(broker,peer,id,agent,created_at) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`,
		from.broker, from.peer, from.task, b.Agent, now())
	return err
}

// Only bounded version-shaped values and known reason codes cross into the
// result. No raw stdout, paths or credentials are exposed by this tool.
var agentVersionValue = regexp.MustCompile(`^[0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9}(?:-(?:[a-f0-9]{7,40}|(?:alpha|beta|rc|dev)(?:\.[0-9]{1,9})?))?$`)

func applyAgentVersionReply(tx *sql.Tx, from netLink, b netBody) error {
	version, reason := b.Text, b.Reason
	switch reason {
	case "agent_unavailable", "executable_unavailable", "probe_directory_unavailable", "timeout_or_cancelled",
		"output_limit", "inherited_pipe_not_closed", "execution_failed", "unrecognized_version_output", "probe_failed":
		version = ""
	default:
		reason = ""
		if !agentVersionValue.MatchString(version) {
			version, reason = "", "unrecognized_version_output"
		}
	}
	at, err := time.Parse(time.RFC3339Nano, b.CheckedAt)
	if err != nil || at.IsZero() {
		b.CheckedAt = ""
	}
	_, err = tx.Exec(`UPDATE net_agent_versions SET version=?,reason=?,checked_at=?
 WHERE broker=? AND peer=? AND id=? AND agent=? AND version='' AND reason=''`,
		version, reason, b.CheckedAt, from.broker, from.peer, from.task, b.Agent)
	return err
}

func agentVersionUndeliverable(tx *sql.Tx, kind, id string) error {
	if kind != "agent_version_request" {
		return nil
	}
	_, err := tx.Exec(`UPDATE net_agent_versions SET reason='computer_unreachable' WHERE id=? AND version='' AND reason=''`, id)
	return err
}
