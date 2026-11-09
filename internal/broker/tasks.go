package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"
)

// Shared task lists (T-047). Every computer posts the tasks it started in
// the last days and receives the lists of the other computers, read-only.
// The broker keeps only the latest list of each computer, like presence;
// task messages (mailbox.go) are not used, so their limits stay free for
// real work. The endpoint is separate from presence: a broker before
// preview.47 answers it with 404 and keeps accepting presence as before.

// Limits of one computer's list.
const (
	MaxSharedTasks  = 50
	MaxSharedTitle  = 200
	MaxSharedReport = 800
	maxSharedBody   = 256 << 10
)

// SharedTask is one task of a computer's list.
type SharedTask struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Title      string `json:"title"`
	Agent      string `json:"agent"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	// Report is the beginning of the worker's report.
	Report string `json:"report,omitempty"`
	Tokens int64  `json:"tokens,omitempty"`
}

// PeerTasks is another computer's latest list. Age is how many seconds ago
// it was posted, by the broker's clock.
type PeerTasks struct {
	Peer  string       `json:"peer"`
	Name  string       `json:"name"`
	Age   int          `json:"age"`
	Tasks []SharedTask `json:"tasks"`
}

type sharedRequest struct {
	Tasks []SharedTask `json:"tasks"`
}

type sharedResponse struct {
	BrokerID string      `json:"broker_id"`
	DeviceID string      `json:"device_id"`
	Peers    []PeerTasks `json:"peers"`
}

// A task state is one short word; the window names the ones it knows.
var sharedState = regexp.MustCompile(`^[a-z_]{1,32}$`)

func validSharedTasks(tasks []SharedTask) bool {
	if len(tasks) > MaxSharedTasks {
		return false
	}
	for _, t := range tasks {
		if t.ID == "" || len(t.ID) > 64 || !validDeviceName(t.Project) || !agentName.MatchString(t.Agent) || !sharedState.MatchString(t.State) || t.Tokens < 0 {
			return false
		}
		if !utf8.ValidString(t.Title) || utf8.RuneCountInString(t.Title) > MaxSharedTitle || !utf8.ValidString(t.Report) || utf8.RuneCountInString(t.Report) > MaxSharedReport {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, t.CreatedAt); err != nil {
			return false
		}
		if t.FinishedAt != "" {
			if _, err := time.Parse(time.RFC3339Nano, t.FinishedAt); err != nil {
				return false
			}
		}
	}
	return true
}

func (b *Broker) initSharedTasks() error {
	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS shared_tasks(peer TEXT PRIMARY KEY, tasks TEXT NOT NULL, posted_at TEXT NOT NULL)`)
	return err
}

// ShareTasks stores peer's list and returns the lists of the other
// computers that are not revoked.
func (b *Broker) ShareTasks(peer string, tasks []SharedTask) ([]PeerTasks, error) {
	if !validSharedTasks(tasks) {
		return nil, errors.New("список задач не прошёл проверку")
	}
	if tasks == nil {
		tasks = []SharedTask{}
	}
	raw, _ := json.Marshal(tasks)
	if _, err := b.db.Exec(`INSERT INTO shared_tasks(peer,tasks,posted_at) VALUES(?,?,?) ON CONFLICT(peer) DO UPDATE SET tasks=excluded.tasks, posted_at=excluded.posted_at`, peer, string(raw), b.now().UTC().Format(time.RFC3339)); err != nil {
		return nil, err
	}
	// The hosting computer's name comes from its presence; devices have
	// theirs in devices.
	hostName := "Сервер"
	if roster, err := b.Roster(nil); err == nil {
		for _, p := range roster {
			if p.Host && p.Name != "" {
				hostName = p.Name
			}
		}
	}
	rows, err := b.db.Query(`SELECT s.peer,COALESCE(d.name,''),s.tasks,s.posted_at FROM shared_tasks s LEFT JOIN devices d ON d.id=s.peer
 WHERE s.peer<>? AND (s.peer=? OR (d.id IS NOT NULL AND d.revoked_at IS NULL)) ORDER BY s.peer=? DESC, d.created_at, s.peer`, peer, HostPeer, HostPeer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := b.now().UTC()
	out := []PeerTasks{}
	for rows.Next() {
		var pt PeerTasks
		var list, at string
		if err := rows.Scan(&pt.Peer, &pt.Name, &list, &at); err != nil {
			return nil, err
		}
		if pt.Peer == HostPeer {
			pt.Name = hostName
		}
		when, err := time.Parse(time.RFC3339, at)
		// A stored list that cannot be read is left out.
		if err != nil || json.Unmarshal([]byte(list), &pt.Tasks) != nil {
			continue
		}
		if pt.Tasks == nil {
			pt.Tasks = []SharedTask{}
		}
		pt.Age = max(0, int(now.Sub(when)/time.Second))
		out = append(out, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *Broker) handleSharedTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	d, ok := b.authorized(w, r)
	if !ok {
		return
	}
	var in sharedRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxSharedBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || !validSharedTasks(in.Tasks) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	peers, err := b.ShareTasks(d.ID, in.Tasks)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, sharedResponse{BrokerID: b.id, DeviceID: d.ID, Peers: peers})
}

// ErrNoSharedTasks means the broker is older than shared task lists.
var ErrNoSharedTasks = errors.New("сервер не поддерживает общий список задач")

// PostSharedTasks sends this device's list and returns the others'.
func PostSharedTasks(ctx context.Context, c Connection, tasks []SharedTask) ([]PeerTasks, error) {
	if tasks == nil {
		tasks = []SharedTask{}
	}
	var out sharedResponse
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/tasks", c.DeviceKey, sharedRequest{Tasks: tasks}, &out)
	if err != nil {
		return nil, err
	}
	switch {
	case code == http.StatusUnauthorized:
		return nil, ErrRevoked
	case code == http.StatusNotFound:
		return nil, ErrNoSharedTasks
	case code != http.StatusOK || out.BrokerID != c.BrokerID || out.DeviceID != c.DeviceID:
		return nil, fmt.Errorf("сервер не принял список задач (код %d)", code)
	}
	if out.Peers == nil {
		out.Peers = []PeerTasks{}
	}
	return out.Peers, nil
}

// ShareTasks posts this computer's list to the network n and returns the
// lists of its other computers.
func (s *Service) ShareTasks(ctx context.Context, n Network, tasks []SharedTask) ([]PeerTasks, error) {
	s.mu.Lock()
	current, ok := s.networkLocked()
	if !ok || current != n {
		s.mu.Unlock()
		return nil, ErrNetworkChanged
	}
	if s.broker != nil {
		b := s.broker
		s.mu.Unlock()
		return b.ShareTasks(HostPeer, tasks)
	}
	c := *s.member
	s.mu.Unlock()
	call, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return PostSharedTasks(call, c, tasks)
}
