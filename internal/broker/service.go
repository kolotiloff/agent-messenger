package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// DefaultPort is the broker's port unless the owner picks another.
const DefaultPort = 47100

// Scopes of a hosted broker: "local" listens on 127.0.0.1 only, "lan" on all
// interfaces. Access beyond this computer is always the owner's choice.
const (
	ScopeLocal = "local"
	ScopeLAN   = "lan"
)

// Settings is the hosted broker's saved configuration.
type Settings struct {
	Enabled bool   `json:"enabled"`
	Scope   string `json:"scope"`
	Port    int    `json:"port"`
}

// HostStatus is what the window shows about the broker on this computer.
type HostStatus struct {
	Enabled     bool     `json:"enabled"`
	Running     bool     `json:"running"`
	Scope       string   `json:"scope"`
	Port        int      `json:"port"`
	Addresses   []string `json:"addresses"`
	BrokerID    string   `json:"broker_id,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Devices     []Device `json:"devices"`
	// OpenInvitations are the IDs of invitations not used yet and not
	// expired, so the window can hide a code once it is spent.
	OpenInvitations []string `json:"open_invitations"`
	Error           string   `json:"error,omitempty"`
}

// MemberStatus is what the window shows about this device's connection. It
// never contains the device key.
type MemberStatus struct {
	Connected   bool   `json:"connected"`
	Address     string `json:"address,omitempty"`
	BrokerID    string `json:"broker_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	DeviceName  string `json:"device_name,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
	Reachable   bool   `json:"reachable"`
	Error       string `json:"error,omitempty"`
}

// Status is the whole server state for the window.
type Status struct {
	Host   HostStatus   `json:"host"`
	Member MemberStatus `json:"member"`
	// Peers are the other computers of this computer's network: the devices
	// of the hosted broker, or the computers of the broker this computer
	// joined. This computer is not among them.
	Peers []Peer `json:"peers"`
	// Accept is whether this computer runs tasks that other computers of
	// its network start (T-037); the client fills it in.
	Accept bool `json:"accept"`
	// Network is the last problem with sending or reading task messages,
	// if any; the client fills it in.
	Network string `json:"network_error,omitempty"`
}

// Service runs the broker hosted by the background client and keeps this
// device's connection to a broker. Its files live in the client's data
// directory: broker/ for the hosted broker, server-connection.json for the
// connection (private to the owner, holds the device key).
type Service struct {
	root string
	base context.Context

	// op serializes starting and stopping the hosted broker; mu guards the
	// fields and is released while a stopping server finishes.
	op       sync.Mutex
	mu       sync.Mutex
	settings Settings
	broker   *Broker
	stop     context.CancelFunc
	done     chan struct{}
	hostErr  string

	member     *Connection
	checkedAt  string
	reachable  bool
	memberErr  string
	damaged    bool // the connection file exists but cannot be read
	connecting bool

	// presence reports this computer's agents; peers is the member's last
	// list of computers from its broker.
	presence func() (Presence, error)
	peers    []Peer
	peersAt  time.Time

	// interfaces lists this computer's addresses; replaced in tests.
	interfaces func() ([]net.Addr, error)
}

// NewService loads the saved settings and connection. It starts nothing and
// never fails: an unreadable file is shown in the window instead, so server
// settings can never keep the client from starting.
func NewService(ctx context.Context, root string) *Service {
	s := &Service{root: root, base: ctx, settings: Settings{Scope: ScopeLocal, Port: DefaultPort}, interfaces: net.InterfaceAddrs}
	raw, err := os.ReadFile(s.settingsPath())
	if err == nil {
		var saved Settings
		if json.Unmarshal(raw, &saved) == nil && validScope(saved.Scope) && validPort(saved.Port) {
			s.settings = saved
		} else {
			s.hostErr = "файл настроек сервера повреждён, сервер остановлен: " + s.settingsPath()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.hostErr = "не удалось прочитать настройки сервера: " + err.Error()
	}
	raw, err = os.ReadFile(s.connectionPath())
	if err == nil {
		var c Connection
		if json.Unmarshal(raw, &c) != nil || !c.Valid() {
			// Kept as it is; the client still starts and the window says so.
			s.memberErr = "файл подключения к серверу повреждён: " + s.connectionPath() + ". Нажмите «Отключить», чтобы удалить его, или исправьте файл"
			s.damaged = true
		} else {
			s.member = &c
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.memberErr = "не удалось прочитать подключение к серверу: " + err.Error()
		s.damaged = true
	}
	return s
}

func (s *Service) settingsPath() string   { return filepath.Join(s.root, "broker", "settings.json") }
func (s *Service) connectionPath() string { return filepath.Join(s.root, "server-connection.json") }

func validScope(scope string) bool { return scope == ScopeLocal || scope == ScopeLAN }
func validPort(port int) bool      { return port >= 1024 && port <= 65535 }

// Start starts the hosted broker if the owner enabled it. A failure is kept
// for the window; it does not stop the client.
func (s *Service) Start() {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings.Enabled {
		if err := s.startLocked(); err != nil {
			s.hostErr = err.Error()
		}
	}
}

func (s *Service) startLocked() error {
	if s.broker != nil {
		return nil
	}
	b, err := Open(filepath.Join(s.root, "broker"))
	if err != nil {
		return err
	}
	listen := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.settings.Port))
	if s.settings.Scope == ScopeLAN {
		listen = net.JoinHostPort("", strconv.Itoa(s.settings.Port))
	}
	ln, err := b.Listen(listen)
	if err != nil {
		b.Close()
		var op *net.OpError
		if errors.As(err, &op) {
			return fmt.Errorf("порт %d занят или недоступен: %v", s.settings.Port, op.Err)
		}
		return err
	}
	ctx, cancel := context.WithCancel(s.base)
	done := make(chan struct{})
	s.broker, s.stop, s.done, s.hostErr = b, cancel, done, ""
	go func() {
		// Serve returns after every request has ended; only then are the
		// database and the lock released, and only then is the broker
		// marked stopped, so a restart never meets its own lock.
		err := b.Serve(ctx, ln)
		failed := err != nil && ctx.Err() == nil
		cancel()
		b.Close()
		s.mu.Lock()
		if failed {
			s.hostErr = "сервер остановился: " + err.Error()
		}
		if s.broker == b {
			s.broker, s.stop, s.done = nil, nil, nil
		}
		s.mu.Unlock()
		close(done)
	}()
	return nil
}

// stopLocked stops the hosted broker and waits until it has released its
// data. It drops mu while waiting.
func (s *Service) stopLocked() {
	if s.done == nil {
		return
	}
	stop, done := s.stop, s.done
	s.mu.Unlock()
	stop()
	<-done
	s.mu.Lock()
}

// Enable saves the settings and (re)starts the hosted broker with them. The
// settings are saved only if the broker started.
func (s *Service) Enable(scope string, port int) error {
	if !validScope(scope) {
		return errors.New("доступ: только этот компьютер или локальная сеть")
	}
	if port == 0 {
		port = DefaultPort
	}
	if !validPort(port) {
		return errors.New("порт должен быть от 1024 до 65535")
	}
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.settings
	s.stopLocked()
	s.settings = Settings{Enabled: true, Scope: scope, Port: port}
	if err := s.startLocked(); err != nil {
		s.settings = previous
		if previous.Enabled {
			if again := s.startLocked(); again != nil {
				s.hostErr = again.Error()
			}
		}
		return err
	}
	if err := s.saveSettingsLocked(); err != nil {
		// Not remembered means not on: go back to what was saved.
		s.stopLocked()
		s.settings = previous
		if previous.Enabled {
			if again := s.startLocked(); again != nil {
				s.hostErr = again.Error()
			}
		}
		return fmt.Errorf("не удалось сохранить настройки сервера: %w", err)
	}
	return nil
}

// Disable stops the hosted broker. Its data (identity, devices) is kept, so
// turning it on again keeps the devices.
func (s *Service) Disable() error {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	s.settings.Enabled = false
	s.hostErr = ""
	return s.saveSettingsLocked()
}

func (s *Service) saveSettingsLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.settingsPath()), 0o700); err != nil {
		return err
	}
	raw, _ := json.Marshal(s.settings)
	return writeFile(s.settingsPath(), raw)
}

// Close stops the hosted broker.
func (s *Service) Close() {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

// addressesLocked lists the addresses other devices may use.
func (s *Service) addressesLocked() []string {
	port := strconv.Itoa(s.settings.Port)
	if s.settings.Scope != ScopeLAN {
		return []string{net.JoinHostPort("127.0.0.1", port)}
	}
	out := []string{}
	addrs, _ := s.interfaces()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil || !ipnet.IP.IsPrivate() {
			continue
		}
		out = append(out, net.JoinHostPort(ipnet.IP.String(), port))
	}
	return out
}

// Invite creates an invitation for one of the broker's addresses.
func (s *Service) Invite(address string) (InviteCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broker == nil {
		return InviteCode{}, errors.New("сначала запустите сервер")
	}
	allowed := false
	for _, a := range s.addressesLocked() {
		allowed = allowed || a == address
	}
	if !allowed {
		return InviteCode{}, errors.New("выберите один из адресов этого сервера")
	}
	return s.broker.Invite(address)
}

// Revoke revokes a device of the hosted broker.
func (s *Service) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broker == nil {
		return errors.New("сначала запустите сервер")
	}
	return s.broker.Revoke(id)
}

// Connect joins a broker with an invitation. A device belongs to one broker:
// joining another one needs an explicit disconnect first, so nothing is ever
// sent to a new broker silently.
func (s *Service) Connect(ctx context.Context, code, name string) error {
	inv, err := ParseInvitation(code)
	if err != nil {
		return err
	}
	s.mu.Lock()
	switch {
	case s.member != nil:
		s.mu.Unlock()
		return errors.New("этот компьютер уже подключён к серверу; сначала отключите его")
	case s.damaged:
		s.mu.Unlock()
		return errors.New("файл подключения к серверу повреждён; сначала нажмите «Отключить»")
	case s.connecting:
		s.mu.Unlock()
		return errors.New("подключение уже выполняется")
	}
	// Only one registration at a time, so no second device is left on a
	// broker with a key nobody keeps.
	s.connecting = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.connecting = false
		s.mu.Unlock()
	}()
	c, err := Register(ctx, inv, name)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeFile(s.connectionPath(), raw); err != nil {
		return err
	}
	s.member, s.checkedAt, s.reachable, s.memberErr = &c, time.Now().UTC().Format(time.RFC3339), true, ""
	return nil
}

// CheckConnection asks the broker whether this device is still a member.
func (s *Service) CheckConnection(ctx context.Context) error {
	s.mu.Lock()
	if s.member == nil {
		s.mu.Unlock()
		return errors.New("этот компьютер не подключён к серверу")
	}
	c := *s.member
	s.mu.Unlock()
	_, err := Check(ctx, c)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.member == nil || s.member.DeviceID != c.DeviceID {
		return err
	}
	s.checkedAt, s.reachable, s.memberErr = time.Now().UTC().Format(time.RFC3339), err == nil, ""
	if err != nil {
		s.memberErr = err.Error()
	}
	return err
}

// Disconnect forgets this device's connection. The device stays listed on
// the broker until its owner revokes it there.
func (s *Service) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.connectionPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.member, s.checkedAt, s.reachable, s.memberErr, s.damaged, s.peers = nil, "", false, "", false, nil
	return nil
}

// MemberName is the name this computer has on the server it joined; empty
// when it joined none.
func (s *Service) MemberName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.member == nil {
		return ""
	}
	return s.member.DeviceName
}

// Status reports the hosted broker and the connection.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := HostStatus{Enabled: s.settings.Enabled, Running: s.broker != nil, Scope: s.settings.Scope, Port: s.settings.Port, Devices: []Device{}, OpenInvitations: []string{}, Error: s.hostErr}
	if s.broker != nil {
		h.Addresses = s.addressesLocked()
		h.BrokerID, h.Fingerprint = s.broker.ID(), s.broker.Fingerprint()
		if devices, err := s.broker.Devices(); err == nil {
			h.Devices = devices
		} else if h.Error == "" {
			h.Error = "список устройств не читается: " + err.Error()
		}
		if open, err := s.broker.OpenInvitations(); err == nil {
			h.OpenInvitations = open
		}
	}
	if h.Addresses == nil {
		h.Addresses = []string{}
	}
	m := MemberStatus{CheckedAt: s.checkedAt, Reachable: s.reachable, Error: s.memberErr}
	if s.member != nil {
		m.Connected, m.Address, m.BrokerID, m.DeviceID, m.DeviceName, m.ConnectedAt = true, s.member.Address, s.member.BrokerID, s.member.DeviceID, s.member.DeviceName, s.member.ConnectedAt
	}
	peers := []Peer{}
	if s.broker != nil {
		if roster, err := s.broker.Roster(); err == nil {
			for _, p := range roster {
				// A computer that joined its own server is not its own peer.
				if p.ID != HostPeer && (s.member == nil || p.ID != s.member.DeviceID) {
					peers = append(peers, p)
				}
			}
		}
	} else if s.member != nil {
		// Ages grow while no new list arrives, so a lost connection shows
		// every computer as not reporting.
		late := int(time.Since(s.peersAt) / time.Second)
		for _, p := range s.peers {
			if p.ID != s.member.DeviceID {
				if p.Age >= 0 {
					p.Age += late
				}
				peers = append(peers, p)
			}
		}
	}
	return Status{Host: h, Member: m, Peers: peers}
}

// Summary is the one-word server state for the status command:
// not_configured, hosting, connected or hosting+connected.
func (s *Service) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.broker != nil && s.member != nil:
		return "hosting+connected"
	case s.broker != nil:
		return "hosting"
	case s.member != nil:
		return "connected"
	}
	return "not_configured"
}

// SetPresence sets how this computer describes its agents to its network.
func (s *Service) SetPresence(fn func() (Presence, error)) {
	s.mu.Lock()
	s.presence = fn
	s.mu.Unlock()
}

// RunPresence reports this computer's agents every PresenceInterval until
// ctx ends: to the hosted broker directly, and to the broker this computer
// joined. Reporting is a status update that the next round replaces, so a
// failed round is shown and simply not repeated early.
func (s *Service) RunPresence(ctx context.Context) {
	ticker := time.NewTicker(PresenceInterval)
	defer ticker.Stop()
	for {
		s.ReportPresence(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ReportPresence makes one round of reports.
func (s *Service) ReportPresence(ctx context.Context) {
	s.mu.Lock()
	fn := s.presence
	s.mu.Unlock()
	if fn == nil {
		return
	}
	p, err := fn()
	if err != nil {
		return
	}
	// A host name or coordinator label that is not one plain line is left
	// out rather than holding back the whole report.
	if p.Name != "" && !validDeviceName(p.Name) {
		p.Name = ""
	}
	if p.Coordinator != "" && !validDeviceName(p.Coordinator) {
		p.Coordinator = "coordinator"
	}
	if !p.valid() {
		return
	}
	s.mu.Lock()
	if s.broker != nil {
		_ = s.broker.Report(HostPeer, p)
	}
	var c *Connection
	if s.member != nil {
		copied := *s.member
		c = &copied
	}
	s.mu.Unlock()
	if c == nil {
		return
	}
	call, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	peers, err := Report(call, *c, p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.member == nil || s.member.DeviceID != c.DeviceID {
		return
	}
	s.checkedAt, s.reachable, s.memberErr = time.Now().UTC().Format(time.RFC3339), err == nil, ""
	if err != nil {
		s.memberErr = err.Error()
		return
	}
	s.peers, s.peersAt = peers, time.Now()
	// The broker now lists this computer under the reported name. A host
	// before preview.37 ignores the name and keeps the one it joined with.
	if p.Name != "" && s.member.DeviceName != p.Name {
		s.member.DeviceName = p.Name
		if raw, err := json.Marshal(s.member); err == nil {
			_ = writeFile(s.connectionPath(), raw)
		}
	}
}

// Network is the network this computer exchanges tasks in: the broker it
// hosts, or else the broker it joined. Self is this computer's ID there.
type Network struct {
	BrokerID string
	Self     string
}

// ErrNetworkChanged means the network changed while a message was on its
// way; nothing was sent or read.
var ErrNetworkChanged = errors.New("сеть изменилась")

// Network returns the current network, if any.
func (s *Service) Network() (Network, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.networkLocked()
}

func (s *Service) networkLocked() (Network, bool) {
	if s.broker != nil {
		return Network{BrokerID: s.broker.ID(), Self: HostPeer}, true
	}
	if s.member != nil {
		return Network{BrokerID: s.member.BrokerID, Self: s.member.DeviceID}, true
	}
	return Network{}, false
}

// Send sends a message to another computer of the network n.
func (s *Service) Send(ctx context.Context, n Network, to, opID string, body []byte) error {
	s.mu.Lock()
	current, ok := s.networkLocked()
	if !ok || current != n {
		s.mu.Unlock()
		return ErrNetworkChanged
	}
	if s.broker != nil {
		b := s.broker
		s.mu.Unlock()
		return b.Send(HostPeer, to, opID, body)
	}
	c := *s.member
	s.mu.Unlock()
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return SendMessage(call, c, to, opID, body)
}

// Inbox acknowledges this computer's messages up to after and returns the
// next ones from the network n.
func (s *Service) Inbox(ctx context.Context, n Network, after int64) ([]Message, error) {
	s.mu.Lock()
	current, ok := s.networkLocked()
	if !ok || current != n {
		s.mu.Unlock()
		return nil, ErrNetworkChanged
	}
	if s.broker != nil {
		b := s.broker
		s.mu.Unlock()
		return b.Inbox(HostPeer, after)
	}
	c := *s.member
	s.mu.Unlock()
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return ReadInbox(call, c, after)
}
