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
	"strings"
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
	// LogShare is whether this computer gives the end of its log to the
	// coordinators of the other computers (T-057); the client fills it in.
	LogShare bool `json:"log_share"`
	// Network is the last problem with sending or reading task messages,
	// if any; the client fills it in.
	Network string `json:"network_error,omitempty"`
}

// Service runs the broker hosted by the background client and keeps this
// device's connection to a broker. Its files live in the client's data
// directory: broker/ for the hosted broker, server-connection.json for the
// connection (private to the owner, holds the device key).
// readyRetry is how long a computer reports without readiness after the
// host refused it.
const readyRetry = 10 * time.Minute

// usageRefresh is how often a computer repeats its unchanged usage, in
// case the host lost it (T-076). A change is sent at the next report.
const usageRefresh = 10 * time.Minute

type Service struct {
	root string
	base context.Context

	// op serializes starting and stopping the hosted broker; mu guards the
	// fields and is released while a stopping server finishes.
	op       sync.Mutex
	mu       sync.Mutex
	settings Settings
	// readySkip is until when reports leave out readiness, after the host
	// refused it (an older host, T-070).
	readySkip time.Time
	// versionSkip is the same for the program version (T-078).
	versionSkip time.Time
	// computerSkip is the same for the computer's lasting id (T-132).
	computerSkip time.Time
	// usageSkip is the same for usage. usageSent is the version of this
	// computer's usage that usageTo (broker/device) last accepted, at
	// usageAt; usageCopies holds the other computers' usage from there.
	usageSkip   time.Time
	usageSent   string
	usageAt     time.Time
	usageTo     string
	usageCopies map[string]PresenceUsageReport
	broker      *Broker
	stop        context.CancelFunc
	done        chan struct{}
	hostErr     string

	member *Connection
	// former are this computer's own devices from earlier connections.
	former     []formerDevice
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
	if raw, err := os.ReadFile(s.formerPath()); err == nil {
		_ = json.Unmarshal(raw, &s.former)
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
	// Discovery for short invitations (T-052). If the port is taken, only
	// the long code works; nothing else depends on it.
	discoveryAt := net.JoinHostPort("127.0.0.1", strconv.Itoa(DiscoveryPort))
	if s.settings.Scope == ScopeLAN {
		discoveryAt = net.JoinHostPort("", strconv.Itoa(DiscoveryPort))
	}
	if pc, err := net.ListenPacket("udp4", discoveryAt); err == nil {
		port := s.settings.Port
		go serveDiscovery(ctx, pc, func() (discoverAnswer, bool) {
			return discoverAnswer{V: 1, Broker: b.id, Port: port}, b.ShortActive()
		})
	}
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
	// A computer either hosts its network or belongs to another one, never
	// both: tasks would go through one while the owner watches the other.
	if s.member != nil || s.damaged {
		return errors.New("этот компьютер подключён к другому серверу; сначала отключите его")
	}
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
// sent to a new broker silently. A computer that hosts a broker stops it
// first: it hosts its network or joins another one, not both.
func (s *Service) Connect(ctx context.Context, code, name string) error {
	short := shortCodeOf(code)
	var inv Invitation
	if short == "" {
		var err error
		if inv, err = ParseInvitation(code); err != nil {
			return err
		}
	}
	s.mu.Lock()
	switch {
	case s.broker != nil || s.settings.Enabled:
		s.mu.Unlock()
		return errors.New("на этом компьютере запущен свой сервер; сначала остановите его")
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
	if short != "" {
		var err error
		if inv, err = pairOnNetwork(ctx, short); err != nil {
			return err
		}
	}
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

// Disconnect forgets this device's connection. The device first leaves the
// broker, so the others stop listing it; if the broker cannot be reached or
// is older than leaving, the device stays listed there until its owner
// revokes it. Either way this computer remembers the device as a former one
// of its own and never lists it as another computer.
func (s *Service) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	var c *Connection
	if s.member != nil {
		copied := *s.member
		c = &copied
	}
	own := s.broker
	s.mu.Unlock()
	if c != nil {
		if own != nil && own.ID() == c.BrokerID {
			_ = own.Revoke(c.DeviceID)
		} else {
			call, cancel := context.WithTimeout(ctx, 5*time.Second)
			_ = Leave(call, *c)
			cancel()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.connectionPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if c != nil {
		s.rememberFormerLocked(c.BrokerID, c.DeviceID)
	}
	s.member, s.checkedAt, s.reachable, s.memberErr, s.damaged, s.peers = nil, "", false, "", false, nil
	return nil
}

// formerDevice is a device this computer had on a broker before it
// disconnected.
type formerDevice struct {
	BrokerID string `json:"broker_id"`
	DeviceID string `json:"device_id"`
}

// maxFormer bounds the remembered former devices.
const maxFormer = 32

func (s *Service) formerPath() string { return filepath.Join(s.root, "server-former.json") }

func (s *Service) rememberFormerLocked(brokerID, deviceID string) {
	for _, f := range s.former {
		if f.BrokerID == brokerID && f.DeviceID == deviceID {
			return
		}
	}
	s.former = append(s.former, formerDevice{BrokerID: brokerID, DeviceID: deviceID})
	if len(s.former) > maxFormer {
		s.former = s.former[len(s.former)-maxFormer:]
	}
	// Only a list to hide; a failed write shows the device again after a
	// restart, nothing worse.
	if raw, err := json.Marshal(s.former); err == nil {
		_ = writeFile(s.formerPath(), raw)
	}
}

// formerLocked reports whether id was this computer's own device on broker.
func (s *Service) formerLocked(brokerID, id string) bool {
	for _, f := range s.former {
		if f.BrokerID == brokerID && f.DeviceID == id {
			return true
		}
	}
	return false
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
		if roster, err := s.broker.Roster(nil); err == nil {
			for _, p := range roster {
				// A computer that joined its own server is not its own peer.
				if p.ID != HostPeer && (s.member == nil || p.ID != s.member.DeviceID) && !s.formerLocked(s.broker.ID(), p.ID) {
					peers = append(peers, p)
				}
			}
		}
	} else if s.member != nil {
		// Ages grow while no new list arrives, so a lost connection shows
		// every computer as not reporting.
		late := int(time.Since(s.peersAt) / time.Second)
		for _, p := range s.peers {
			// A device under this computer's own name is its earlier one,
			// left there when it joined again (a host before preview.59
			// still lists it).
			if p.ID != s.member.DeviceID && !s.formerLocked(s.member.BrokerID, p.ID) && !strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(s.member.DeviceName)) {
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
	to := c.BrokerID + "/" + c.DeviceID
	sent := ""
	if p.Usage != nil {
		sent = UsageHash(p.Usage.Days)
	}
	s.mu.Lock()
	skipReady := time.Now().Before(s.readySkip)
	if time.Now().Before(s.versionSkip) {
		p.Version = ""
	}
	if time.Now().Before(s.computerSkip) {
		p.Computer = ""
	}
	if s.usageTo != to {
		s.usageTo, s.usageSent, s.usageCopies = to, "", nil
	}
	// Usage goes only when it changed, or rarely to refresh (T-076).
	if time.Now().Before(s.usageSkip) || (sent == s.usageSent && time.Since(s.usageAt) < usageRefresh) {
		p.Usage = nil
	}
	if len(s.usageCopies) > 0 && !time.Now().Before(s.usageSkip) {
		p.UsageSeen = map[string]string{}
		for peer, u := range s.usageCopies {
			p.UsageSeen[peer] = u.Hash
		}
	}
	s.mu.Unlock()
	if skipReady {
		p.Ready = nil
	}
	peers, err := Report(call, *c, p)
	if errors.Is(err, errPresenceRefused) && p.Computer != "" {
		// A host before 0.61 refuses the field (T-132).
		s.mu.Lock()
		s.computerSkip = time.Now().Add(readyRetry)
		s.mu.Unlock()
		p.Computer = ""
		peers, err = Report(call, *c, p)
	}
	if errors.Is(err, errPresenceRefused) && p.Version != "" {
		// A host older than versions refuses the field (T-078).
		s.mu.Lock()
		s.versionSkip = time.Now().Add(readyRetry)
		s.mu.Unlock()
		p.Version = ""
		peers, err = Report(call, *c, p)
	}
	if errors.Is(err, errPresenceRefused) && (p.Usage != nil || p.UsageSeen != nil) {
		// A host older than usage refuses the fields (T-076).
		s.mu.Lock()
		s.usageSkip = time.Now().Add(readyRetry)
		s.mu.Unlock()
		p.Usage, p.UsageSeen = nil, nil
		peers, err = Report(call, *c, p)
	}
	if errors.Is(err, errPresenceRefused) && p.Ready != nil {
		// A host older than readiness refuses the field; report without it
		// and try it again later, after the host may have been updated.
		s.mu.Lock()
		s.readySkip = time.Now().Add(readyRetry)
		s.mu.Unlock()
		p.Ready = nil
		peers, err = Report(call, *c, p)
	}
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
	if p.Usage != nil && s.usageTo == to {
		s.usageSent, s.usageAt = sent, time.Now()
	}
	s.peers, s.peersAt = s.keepUsage(to, peers), time.Now()
	// The broker now lists this computer under the name in its own roster
	// entry: the reported name, or the old one if the host refused it (a
	// name another computer has). A host before preview.37 ignores the name
	// and keeps the one it joined with.
	name := p.Name
	for _, peer := range peers {
		if peer.ID == c.DeviceID && validDeviceName(peer.Name) {
			name = peer.Name
		}
	}
	if p.Name != "" && s.member.DeviceName != name {
		s.member.DeviceName = name
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

// shortCodeOf returns the 6 digits of a short invitation code, which the
// owner may type with spaces or a dash; empty if code is not one.
func shortCodeOf(code string) string {
	digits := strings.NewReplacer(" ", "", "-", "", "\u00a0", "").Replace(strings.TrimSpace(code))
	if !ValidShortCode(digits) {
		return ""
	}
	return digits
}

// pairOnNetwork finds the host with an open short invitation and proves the
// code to it.
func pairOnNetwork(ctx context.Context, code string) (Invitation, error) {
	hosts, err := Discover(ctx, 2*time.Second)
	if err != nil {
		return Invitation{}, err
	}
	if len(hosts) == 0 {
		return Invitation{}, ErrNoHost
	}
	return pairCandidates(ctx, hosts, code)
}

// pairCandidates tries the code on the hosts that answered discovery, in
// order. Anyone on the network can answer, so a candidate that cannot prove
// the code or has no invitation does not end the search. Each server (told
// apart by its certificate) is asked once: the real host may answer on
// several addresses, and every exchange with it costs one of its tries. A
// host that proved the code and then refused this computer's proof is
// final.
func pairCandidates(ctx context.Context, hosts []string, code string) (Invitation, error) {
	asked := map[string]bool{}
	var unproven, gone, last error
	for _, address := range hosts {
		inv, fp, err := pairShort(ctx, address, code, asked)
		if fp != "" {
			asked[fp] = true
		}
		switch {
		case err == nil, errors.Is(err, ErrShortCode):
			return inv, err
		case errors.Is(err, errSkipped):
		case errors.Is(err, errUnproven):
			unproven = ErrShortCode
		case errors.Is(err, ErrShortGone):
			gone = err
		default:
			last = err
		}
	}
	for _, err := range []error{unproven, gone, last} {
		if err != nil {
			return Invitation{}, err
		}
	}
	return Invitation{}, ErrNoHost
}

// keepUsage fills in the usage the roster left out because this computer
// already holds that version, and remembers the rest. Called with mu held.
func (s *Service) keepUsage(to string, peers []Peer) []Peer {
	if s.usageTo != to {
		return peers
	}
	copies := map[string]PresenceUsageReport{}
	for i, p := range peers {
		if p.Usage == nil {
			continue
		}
		u := *p.Usage
		if old, ok := s.usageCopies[p.ID]; ok && u.Days == nil && old.Hash == u.Hash {
			u = old
		}
		copies[p.ID] = u
		peers[i].Usage = &u
	}
	s.usageCopies = copies
	return peers
}
