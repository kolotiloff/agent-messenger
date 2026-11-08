package broker

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// HostPeer is the ID under which the computer that hosts the broker reports
// its own agents. Devices have IDs of the form dev-….
const HostPeer = "host"

// Presence limits: a report is small and is sent every PresenceInterval.
const (
	maxPresenceAgents = 16
	maxAgentCount     = 9999
	maxPresenceBody   = 32 << 10
	maxPresenceModels = 12
)

// PresenceInterval is how often a computer reports its agents.
const PresenceInterval = 15 * time.Second

// modelName matches core.ValidTaskModel's names.
var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`)

var agentName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// PresenceAgent is one agent program of a computer.
type PresenceAgent struct {
	Agent   string `json:"agent"`
	Running int    `json:"running"`
	Waiting int    `json:"waiting_owner"`
	// Models are the model names the owner listed for the agent (T-043).
	// Left out when empty, so a host before preview.37 still accepts the
	// report of a computer without a list.
	Models []string `json:"models,omitempty"`
	// Limit is the agent's last reported usage-limit state (T-044).
	Limit *PresenceLimit `json:"limit,omitempty"`
}

// PresenceLimit mirrors core.AgentLimit.
type PresenceLimit struct {
	State       string   `json:"state"`
	UsedPercent *float64 `json:"used_percent,omitempty"`
	Window      string   `json:"window,omitempty"`
	ResetsAt    string   `json:"resets_at,omitempty"`
	ObservedAt  string   `json:"observed_at"`
}

func (l *PresenceLimit) valid() bool {
	if l == nil {
		return true
	}
	if l.State != "ok" && l.State != "warning" && l.State != "exhausted" || len(l.Window) > 32 || l.ObservedAt == "" {
		return false
	}
	if l.UsedPercent != nil && (*l.UsedPercent < 0 || *l.UsedPercent > 1000) {
		return false
	}
	for _, at := range []string{l.ResetsAt, l.ObservedAt} {
		if _, err := time.Parse(time.RFC3339, at); at != "" && err != nil {
			return false
		}
	}
	return true
}

// Presence is what a computer reports about itself. Name is used only for
// the hosting computer; a device keeps the name it joined with.
type Presence struct {
	Name        string          `json:"name,omitempty"`
	Agents      []PresenceAgent `json:"agents"`
	Coordinator string          `json:"coordinator,omitempty"`
}

// Peer is one computer of the broker as others see it.
type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Host marks the computer that runs the broker.
	Host     bool   `json:"host,omitempty"`
	LastSeen string `json:"last_seen,omitempty"`
	// Age is how many seconds ago the computer last reported its agents,
	// by the broker's clock; -1 if it never did. Computers' clocks may
	// differ, so freshness is never judged from the timestamps alone.
	Age         int             `json:"age"`
	Agents      []PresenceAgent `json:"agents"`
	Coordinator string          `json:"coordinator,omitempty"`
}

// rosterResponse answers a presence report.
type rosterResponse struct {
	BrokerID string `json:"broker_id"`
	DeviceID string `json:"device_id"`
	Peers    []Peer `json:"peers"`
}

func (p Presence) valid() bool {
	if len(p.Agents) > maxPresenceAgents || len(p.Coordinator) > 64 || (p.Name != "" && !validDeviceName(p.Name)) {
		return false
	}
	if p.Coordinator != "" && !validDeviceName(p.Coordinator) {
		return false
	}
	seen := map[string]bool{}
	for _, a := range p.Agents {
		if !agentName.MatchString(a.Agent) || seen[a.Agent] || a.Running < 0 || a.Waiting < 0 || a.Running > maxAgentCount || a.Waiting > maxAgentCount {
			return false
		}
		seen[a.Agent] = true
		if len(a.Models) > maxPresenceModels || !a.Limit.valid() {
			return false
		}
		for _, m := range a.Models {
			if !modelName.MatchString(m) {
				return false
			}
		}
	}
	return true
}

func (b *Broker) initPresence() error {
	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS presence(peer TEXT PRIMARY KEY, report TEXT NOT NULL, reported_at TEXT NOT NULL)`)
	return err
}

// Report stores a computer's latest presence. The hosting computer reports
// under HostPeer.
func (b *Broker) Report(peer string, p Presence) error {
	if !p.valid() {
		return errors.New("сведения об агентах не прошли проверку")
	}
	if p.Agents == nil {
		p.Agents = []PresenceAgent{}
	}
	raw, _ := json.Marshal(p)
	_, err := b.db.Exec(`INSERT INTO presence(peer,report,reported_at) VALUES(?,?,?) ON CONFLICT(peer) DO UPDATE SET report=excluded.report, reported_at=excluded.reported_at`, peer, string(raw), b.now().UTC().Format(time.RFC3339))
	return err
}

// Roster lists the hosting computer (if it reported) and every device that
// is not revoked, with their latest presence.
func (b *Broker) Roster() ([]Peer, error) {
	now := b.now().UTC()
	out := []Peer{}
	var report, at string
	err := b.db.QueryRow(`SELECT report,reported_at FROM presence WHERE peer=?`, HostPeer).Scan(&report, &at)
	switch {
	case err == nil:
		peer := Peer{ID: HostPeer, Host: true, LastSeen: at}
		fill(&peer, report, at, now)
		if peer.Name == "" {
			peer.Name = "Сервер"
		}
		out = append(out, peer)
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}
	rows, err := b.db.Query(`SELECT d.id,d.name,COALESCE(d.last_seen,''),COALESCE(p.report,''),COALESCE(p.reported_at,'') FROM devices d LEFT JOIN presence p ON p.peer=d.id WHERE d.revoked_at IS NULL ORDER BY d.created_at, d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var peer Peer
		var report, at string
		if err := rows.Scan(&peer.ID, &peer.Name, &peer.LastSeen, &report, &at); err != nil {
			return nil, err
		}
		name := peer.Name
		fill(&peer, report, at, now)
		peer.Name = name
		out = append(out, peer)
	}
	return out, rows.Err()
}

// fill copies a stored report into peer. A report that cannot be read is
// shown as no report.
func fill(peer *Peer, report, at string, now time.Time) {
	peer.Age, peer.Agents = -1, []PresenceAgent{}
	var p Presence
	when, err := time.Parse(time.RFC3339, at)
	if report == "" || err != nil || json.Unmarshal([]byte(report), &p) != nil {
		return
	}
	if p.Name != "" {
		peer.Name = p.Name
	}
	peer.Coordinator = p.Coordinator
	if p.Agents != nil {
		peer.Agents = p.Agents
	}
	peer.Age = max(0, int(now.Sub(when)/time.Second))
}

// authorized returns the device that sent r, or answers 401 itself.
func (b *Broker) authorized(w http.ResponseWriter, r *http.Request) (Device, bool) {
	key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(key) != 64 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "device"})
		return Device{}, false
	}
	d, err := b.device(key)
	if errors.Is(err, ErrRevoked) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "device"})
		return Device{}, false
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return Device{}, false
	}
	return d, true
}

// handlePresence stores the calling device's presence and answers with the
// roster.
func (b *Broker) handlePresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	d, ok := b.authorized(w, r)
	if !ok {
		return
	}
	var in Presence
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxPresenceBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || !in.valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	// A device keeps the name it joined with.
	in.Name = ""
	if err := b.Report(d.ID, in); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	peers, err := b.Roster()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, rosterResponse{BrokerID: b.id, DeviceID: d.ID, Peers: peers})
}
