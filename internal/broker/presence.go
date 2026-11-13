package broker

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// olderHost, when set by a test, makes the broker refuse reports the way a
// host before a field existed does.
var olderHost func(Presence) bool

// HostPeer is the ID under which the computer that hosts the broker reports
// its own agents. Devices have IDs of the form dev-….
const HostPeer = "host"

// Presence limits: a report is small and is sent every PresenceInterval.
const (
	maxPresenceAgents = 16
	maxAgentCount     = 9999
	maxPresenceBody   = 128 << 10
	maxPresenceModels = 12
	maxReadyProjects  = 64
	// A computer reports up to 31 days of usage for each of its agents.
	maxUsageRows   = 31 * maxPresenceAgents
	maxUsageTasks  = 1_000_000
	maxUsageTokens = 1_000_000_000_000_000
	maxUsageSeen   = 256
)

// PresenceInterval is how often a computer reports its agents.
const PresenceInterval = 15 * time.Second

// modelName matches core.ValidTaskModel's names.
var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`)

var agentName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// versionName is what a reported program version may look like.
var versionName = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)

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

// PresenceReady says whether a computer runs tasks from other computers and
// for which projects it has a folder (T-070). A host before it refuses a
// report that carries it; the client then reports without it.
type PresenceReady struct {
	Accept   bool     `json:"accept"`
	Projects []string `json:"projects"`
}

func (r *PresenceReady) valid() bool {
	if r == nil {
		return true
	}
	if len(r.Projects) > maxReadyProjects {
		return false
	}
	for _, p := range r.Projects {
		if !validDeviceName(p) {
			return false
		}
	}
	return true
}

// PresenceUsage is one local day of one agent's token usage on a computer
// (T-076): the tasks that ran there and finished that day.
type PresenceUsage struct {
	Day    string `json:"day"`
	Agent  string `json:"agent"`
	Tasks  int    `json:"tasks"`
	Input  int64  `json:"input_tokens"`
	Cached int64  `json:"cached_tokens"`
	Output int64  `json:"output_tokens"`
}

// PresenceUsageReport is a computer's usage of the last days. Hash names
// its version; the broker sets it. In a roster, Days is left out when the
// receiver already holds that version.
type PresenceUsageReport struct {
	Hash string          `json:"hash,omitempty"`
	Days []PresenceUsage `json:"days,omitempty"`
}

// UsageHash names a version of a usage report.
func UsageHash(days []PresenceUsage) string {
	if days == nil {
		days = []PresenceUsage{}
	}
	raw, _ := json.Marshal(days)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func (u *PresenceUsageReport) valid() bool {
	if u == nil {
		return true
	}
	if len(u.Days) > maxUsageRows || len(u.Hash) > 64 {
		return false
	}
	seen := map[[2]string]bool{}
	for _, d := range u.Days {
		if _, err := time.Parse("2006-01-02", d.Day); err != nil || !agentName.MatchString(d.Agent) {
			return false
		}
		if d.Tasks < 0 || d.Tasks > maxUsageTasks || d.Input < 0 || d.Cached < 0 || d.Output < 0 || d.Input > maxUsageTokens || d.Cached > maxUsageTokens || d.Output > maxUsageTokens {
			return false
		}
		k := [2]string{d.Day, d.Agent}
		if seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}

// PresenceOutbox is how many messages a computer has not yet handed to the
// server, and how many seconds the oldest of them has waited by its own
// clock (T-150). Messages for computers that are not listed are left out.
type PresenceOutbox struct {
	Waiting   int `json:"waiting"`
	OldestAge int `json:"oldest_age"`
}

// Outbox limits: a count and an age, nothing else.
const (
	maxOutboxWaiting = 1_000_000
	maxOutboxAge     = 400 * 24 * 3600
)

func (o *PresenceOutbox) valid() bool {
	return o == nil || (o.Waiting >= 0 && o.Waiting <= maxOutboxWaiting && o.OldestAge >= 0 && o.OldestAge <= maxOutboxAge)
}

// PresenceWorkers lists the worker processes a computer runs now for other
// computers' tasks (T-149): the giver's task id and how many seconds ago the
// worker last wrote output, by the running computer's clock. A task that is
// not listed has no worker process there now.
type PresenceWorkers struct {
	Runs []PresenceRun `json:"runs"`
}

// PresenceRun is one running worker of another computer's task.
type PresenceRun struct {
	Task      string `json:"task"`
	OutputAge int    `json:"output_age"`
}

const maxPresenceRuns = 64

// runTask matches core's rule for a task id that travels between computers.
var runTask = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,79}$`)

func (w *PresenceWorkers) valid() bool {
	if w == nil {
		return true
	}
	if len(w.Runs) > maxPresenceRuns {
		return false
	}
	for _, r := range w.Runs {
		if !runTask.MatchString(r.Task) || r.OutputAge < 0 || r.OutputAge > maxOutboxAge {
			return false
		}
	}
	return true
}

// Presence is what a computer reports about itself. Name is the name the
// owner gave the computer; a device's name replaces the one it joined with.
type Presence struct {
	Name        string          `json:"name,omitempty"`
	Agents      []PresenceAgent `json:"agents"`
	Coordinator string          `json:"coordinator,omitempty"`
	Ready       *PresenceReady  `json:"ready,omitempty"`
	// Usage is sent only when it changed since the last report, or rarely
	// to refresh; a report without it keeps the stored one (T-076).
	Usage *PresenceUsageReport `json:"usage,omitempty"`
	// UsageSeen lists the usage versions the reporting computer holds, by
	// peer, so the roster leaves out what it already has. Not stored.
	UsageSeen map[string]string `json:"usage_seen,omitempty"`
	// Version is the program version of the reporting computer (T-078). A
	// host before it refuses a report that carries it; the client then
	// reports without it.
	Version string `json:"version,omitempty"`
	// Computer is the computer's lasting id, the same in every network
	// (T-132). A host before 0.61 refuses a report that carries it; the
	// client then reports without it.
	Computer string `json:"computer,omitempty"`
	// Outbox is what the computer has not sent yet (T-150). A host before
	// 0.67 refuses a report that carries it; the client then reports
	// without it.
	Outbox *PresenceOutbox `json:"outbox,omitempty"`
	// Workers are the running workers of other computers' tasks (T-149),
	// with the same fallback as Outbox.
	Workers *PresenceWorkers `json:"workers,omitempty"`
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
	Ready       *PresenceReady  `json:"ready,omitempty"`
	// Usage is the computer's token usage of the last days; nil if it
	// never reported any (T-076).
	Usage *PresenceUsageReport `json:"usage,omitempty"`
	// Version is the computer's program version; empty if it did not
	// report one (an older version, T-078).
	Version string `json:"version,omitempty"`
	// Computer is the computer's lasting id (T-132); empty if it did not
	// report one.
	Computer string `json:"computer,omitempty"`
	// Outbox is what the computer had not sent at its last report (T-150);
	// nil if it did not say.
	Outbox *PresenceOutbox `json:"outbox,omitempty"`
	// Workers is what it ran for other computers at its last report
	// (T-149); nil if it did not say.
	Workers *PresenceWorkers `json:"workers,omitempty"`
}

// rosterResponse answers a presence report.
type rosterResponse struct {
	BrokerID string `json:"broker_id"`
	DeviceID string `json:"device_id"`
	Peers    []Peer `json:"peers"`
}

func (p Presence) valid() bool {
	if len(p.Agents) > maxPresenceAgents || len(p.Coordinator) > 64 || (p.Name != "" && !validDeviceName(p.Name)) || !p.Ready.valid() || !p.Usage.valid() || len(p.UsageSeen) > maxUsageSeen || (p.Version != "" && !versionName.MatchString(p.Version)) || (p.Computer != "" && !versionName.MatchString(p.Computer)) || !p.Outbox.valid() || !p.Workers.valid() {
		return false
	}
	for peer, hash := range p.UsageSeen {
		if len(peer) > 64 || len(hash) > 64 {
			return false
		}
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
// under HostPeer. A report without usage keeps the stored usage.
func (b *Broker) Report(peer string, p Presence) error {
	if !p.valid() {
		return errors.New("сведения об агентах не прошли проверку")
	}
	if p.Agents == nil {
		p.Agents = []PresenceAgent{}
	}
	p.UsageSeen = nil
	if p.Usage != nil {
		p.Usage = &PresenceUsageReport{Hash: UsageHash(p.Usage.Days), Days: p.Usage.Days}
	} else {
		var old string
		err := b.db.QueryRow(`SELECT report FROM presence WHERE peer=?`, peer).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var stored Presence
		if old != "" && json.Unmarshal([]byte(old), &stored) == nil {
			p.Usage = stored.Usage
		}
	}
	raw, _ := json.Marshal(p)
	_, err := b.db.Exec(`INSERT INTO presence(peer,report,reported_at) VALUES(?,?,?) ON CONFLICT(peer) DO UPDATE SET report=excluded.report, reported_at=excluded.reported_at`, peer, string(raw), b.now().UTC().Format(time.RFC3339))
	return err
}

// Roster lists the hosting computer (if it reported) and every device that
// is not revoked, with their latest presence. A peer's usage days are left
// out when seen already holds that version (T-076).
func (b *Broker) Roster(seen map[string]string) ([]Peer, error) {
	out, err := b.roster()
	if err != nil {
		return nil, err
	}
	for i, p := range out {
		if p.Usage != nil && seen[p.ID] == p.Usage.Hash {
			out[i].Usage = &PresenceUsageReport{Hash: p.Usage.Hash}
		}
	}
	return out, nil
}

func (b *Broker) roster() ([]Peer, error) {
	now := b.now().UTC()
	out := []Peer{}
	var report, at string
	err := b.db.QueryRow(`SELECT report,reported_at FROM presence WHERE peer=?`, HostPeer).Scan(&report, &at)
	switch {
	case err == nil:
		peer := Peer{ID: HostPeer, Host: true, LastSeen: at}
		fill(&peer, report, at, now)
		if peer.Name == "" {
			peer.Name = hostName
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return withoutReplaced(out), nil
}

// withoutReplaced leaves out a device that the same computer replaced by
// joining again: an older device with the name of a newer one that reported
// more recently. Nothing is revoked; the older device shows again if it
// reports again.
func withoutReplaced(peers []Peer) []Peer {
	out := peers[:0:0]
	for i, p := range peers {
		replaced := false
		for _, later := range peers[i+1:] {
			if p.Host || later.Host || !strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(later.Name)) {
				continue
			}
			if later.Age >= 0 && (p.Age < 0 || later.Age < p.Age) {
				replaced = true
			}
		}
		if !replaced {
			out = append(out, p)
		}
	}
	return out
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
	peer.Coordinator, peer.Ready, peer.Usage, peer.Version, peer.Computer, peer.Outbox, peer.Workers = p.Coordinator, p.Ready, p.Usage, p.Version, p.Computer, p.Outbox, p.Workers
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

// hostName is the name the roster shows for the hosting computer.
const hostName = "Сервер"

// nameTaken reports whether name, ignoring case and surrounding spaces, is
// the host's name or the name of an active device other than id.
func (b *Broker) nameTaken(id, name string) (bool, error) {
	same := func(other string) bool { return strings.EqualFold(strings.TrimSpace(other), name) }
	host := hostName
	var report string
	err := b.db.QueryRow(`SELECT report FROM presence WHERE peer=?`, HostPeer).Scan(&report)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	var p Presence
	if report != "" && json.Unmarshal([]byte(report), &p) == nil && p.Name != "" {
		host = p.Name
	}
	if same(host) {
		return true, nil
	}
	rows, err := b.db.Query(`SELECT name FROM devices WHERE revoked_at IS NULL AND id<>?`, id)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var other string
		if err := rows.Scan(&other); err != nil {
			return false, err
		}
		if same(other) {
			return true, nil
		}
	}
	return false, rows.Err()
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
	if decoder.Decode(&in) != nil || !in.valid() || (olderHost != nil && olderHost(in)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	// A device may rename itself: the owner changes the name on that
	// computer, and every computer of the network sees the new one (D-061).
	// A name the host or another computer already has is not taken: the
	// report is kept with the old name, which the roster shows the device.
	if name := strings.TrimSpace(in.Name); name != "" && name != d.Name {
		taken, err := b.nameTaken(d.ID, name)
		if err == nil && !taken {
			_, err = b.db.Exec(`UPDATE devices SET name=? WHERE id=? AND revoked_at IS NULL`, name, d.ID)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
			return
		}
	}
	// The stored report keeps no name: the device's name is in devices.
	in.Name = ""
	seen := in.UsageSeen
	if err := b.Report(d.ID, in); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	peers, err := b.Roster(seen)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, rosterResponse{BrokerID: b.id, DeviceID: d.ID, Peers: peers})
}
