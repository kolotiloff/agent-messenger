// Package broker is the Messenger server: one component that runs either in
// the background client of a chosen computer or alone on a VPS (DEPLOYMENT.md).
//
// This first part registers devices and checks connections. A device joins
// with a one-time invitation that names the broker's address, its identity
// (broker_id) and the SHA-256 fingerprint of its self-signed certificate, so
// the device pins the certificate instead of trusting any TLS server. After
// registration the device holds a revocable key; the broker stores only its
// hash. Computers report their agents (presence.go) so every computer can
// list the others, and carries messages between them (mailbox.go), which is
// how tasks travel from one computer to another (T-037).
package broker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kolotiloff/agent-messenger/internal/platform"
	_ "modernc.org/sqlite"
)

// Protocol is the broker API version. A device refuses another version.
const Protocol = 1

// InviteTTL is how long an invitation can be used.
const InviteTTL = 15 * time.Minute

// MaxDeviceName limits a device name.
const MaxDeviceName = 64

// maxConnections bounds open connections to the broker. The broker shares the
// background client's process, so a flood of connections must not use up the
// client's file descriptors.
const maxConnections = 64

// maxConnectionsPerSource bounds open connections from one computer of the
// network (one IPv4 address or one IPv6 /64), so a single host cannot take
// every slot. This computer itself is not limited per source.
const maxConnectionsPerSource = 16

// maxFailedRegistrations bounds wrong registration attempts per minute from
// one source; past it the broker answers 429 to that source without
// touching the database. Other sources are not affected, so junk from one
// computer cannot lock out a real join from another.
const maxFailedRegistrations = 20

var (
	// ErrInvitation means the invitation is unknown, used or expired.
	ErrInvitation = errors.New("приглашение недействительно, уже использовано или устарело")
	// ErrRevoked means the device key is unknown or was revoked.
	ErrRevoked = errors.New("сервер не принимает это устройство: ключ отозван или неизвестен")
	// ErrUntrusted means the server's certificate is not the pinned one.
	ErrUntrusted = errors.New("сертификат сервера не совпадает с приглашением")
	// ErrIdentity means the server is another broker than the one expected.
	ErrIdentity = errors.New("по этому адресу работает другой сервер")
	// ErrBusy means another process already serves this broker's data.
	ErrBusy = errors.New("этот сервер уже запущен другим процессом")
)

// Broker is one broker's data directory: its identity, certificate and
// devices.
type Broker struct {
	dir         string
	db          *sql.DB
	id          string
	cert        tls.Certificate
	fingerprint string
	now         func() time.Time
	lock        *platform.Lock

	limit    sync.Mutex
	failedAt map[string][]time.Time

	// The current short invitation and its open exchanges (T-052).
	pair pairState
}

// Device is a registered device.
type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	LastSeen  string `json:"last_seen,omitempty"`
	RevokedAt string `json:"revoked_at,omitempty"`
}

// Open opens or creates a broker data directory.
func Open(dir string) (*Broker, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, "broker.sqlite"))+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	b := &Broker{dir: dir, db: db, now: time.Now}
	if err := b.init(); err != nil {
		db.Close()
		return nil, err
	}
	return b, nil
}

func (b *Broker) init() error {
	if _, err := b.db.Exec(`
CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS devices(id TEXT PRIMARY KEY, name TEXT NOT NULL, key_hash TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL, last_seen TEXT, revoked_at TEXT);
CREATE TABLE IF NOT EXISTS invitations(secret_hash TEXT PRIMARY KEY, created_at TEXT NOT NULL, expires_at TEXT NOT NULL, used_at TEXT, device_id TEXT);`); err != nil {
		return err
	}
	if err := b.initSharedTasks(); err != nil {
		return err
	}
	if err := b.initPresence(); err != nil {
		return err
	}
	if err := b.initMailbox(); err != nil {
		return err
	}
	err := b.db.QueryRow(`SELECT value FROM meta WHERE key='broker_id'`).Scan(&b.id)
	if errors.Is(err, sql.ErrNoRows) {
		id, rerr := randomHex(8)
		if rerr != nil {
			return rerr
		}
		if _, err = b.db.Exec(`INSERT OR IGNORE INTO meta(key,value) VALUES('broker_id',?)`, "brk-"+id); err != nil {
			return err
		}
		err = b.db.QueryRow(`SELECT value FROM meta WHERE key='broker_id'`).Scan(&b.id)
	}
	if err != nil {
		return err
	}
	return b.loadCertificate()
}

// loadCertificate reads the broker's certificate, creating it once. The key
// file is private to the owner.
func (b *Broker) loadCertificate() error {
	certPath, keyPath := filepath.Join(b.dir, "tls-cert.pem"), filepath.Join(b.dir, "tls-key.pem")
	if _, err := os.Stat(certPath); errors.Is(err, os.ErrNotExist) {
		if err := writeCertificate(certPath, keyPath, b.id); err != nil {
			return err
		}
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("сертификат сервера не читается: %w", err)
	}
	sum := sha256.Sum256(cert.Certificate[0])
	b.cert, b.fingerprint = cert, hex.EncodeToString(sum[:])
	return nil
}

func writeCertificate(certPath, keyPath, id string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Agent Messenger " + id},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(20, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	// The key is written first; a certificate without its key is never left.
	if err := writeFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})); err != nil {
		return err
	}
	return writeFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// writeFile replaces path atomically and durably with a file only the owner
// can read: a fresh temporary file (never a stale one with other rights),
// flushed to disk before the rename, then the folder flushed.
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	// Flushing a folder is not supported everywhere (Windows); the file
	// itself is already on disk.
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return nil
}

// validDeviceName allows one line of printable text: no control or format
// characters (such as right-to-left overrides that disguise a name).
func validDeviceName(name string) bool {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > MaxDeviceName {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return false
		}
	}
	return true
}

// allowRegistration reports whether a registration attempt from source may
// run now. Old failures of every source are dropped, so the table holds only
// the sources that failed in the last minute.
func (b *Broker) allowRegistration(source string) bool {
	b.limit.Lock()
	defer b.limit.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	for src, times := range b.failedAt {
		kept := times[:0]
		for _, at := range times {
			if at.After(cutoff) {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			delete(b.failedAt, src)
		} else {
			b.failedAt[src] = kept
		}
	}
	return len(b.failedAt[source]) < maxFailedRegistrations
}

func (b *Broker) registrationFailed(source string) {
	b.limit.Lock()
	if b.failedAt == nil {
		b.failedAt = map[string][]time.Time{}
	}
	b.failedAt[source] = append(b.failedAt[source], time.Now())
	b.limit.Unlock()
}

// sourceOf names the computer a request or connection comes from: its IPv4
// address, or the /64 network of its IPv6 address, since one computer
// easily holds many addresses of its /64.
func sourceOf(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	if len(ip) == net.IPv6len {
		return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip.String()
}

// requestSource is sourceOf for an HTTP request.
func requestSource(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return sourceOf(ip)
	}
	return host
}

// ID is the broker's stable identity.
func (b *Broker) ID() string { return b.id }

// Fingerprint is the SHA-256 of the broker's certificate, in hex.
func (b *Broker) Fingerprint() string { return b.fingerprint }

// Close closes the database and releases the serving lock.
func (b *Broker) Close() error {
	if b.lock != nil {
		_ = b.lock.Release()
		b.lock = nil
	}
	return b.db.Close()
}

// InviteCode is a new invitation and when it expires.
type InviteCode struct {
	// ID names the invitation in the host status without revealing it.
	ID        string `json:"id"`
	Code      string `json:"code"`
	Address   string `json:"address"`
	ExpiresAt string `json:"expires_at"`
	// Short is the 6-digit code for a computer on the local network
	// (T-052); it ends earlier, at ShortExpiresAt.
	Short          string `json:"short,omitempty"`
	ShortExpiresAt string `json:"short_expires_at,omitempty"`
}

// Invite creates a one-time invitation for a device that will reach the
// broker at address (host:port).
func (b *Broker) Invite(address string) (InviteCode, error) {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return InviteCode{}, fmt.Errorf("адрес сервера должен быть вида хост:порт")
	}
	secret, err := randomHex(32)
	if err != nil {
		return InviteCode{}, err
	}
	now := b.now().UTC()
	expires := now.Add(InviteTTL).Format(time.RFC3339)
	if _, err := b.db.Exec(`INSERT INTO invitations(secret_hash,created_at,expires_at) VALUES(?,?,?)`, hash(secret), now.Format(time.RFC3339), expires); err != nil {
		return InviteCode{}, err
	}
	inv := Invitation{Address: address, BrokerID: b.id, Fingerprint: b.fingerprint, Secret: secret}
	short, shortExpires, err := b.newShortInvite(secret, now)
	if err != nil {
		return InviteCode{}, err
	}
	return InviteCode{ID: hash(secret)[:16], Code: inv.String(), Address: address, ExpiresAt: expires, Short: short, ShortExpiresAt: shortExpires}, nil
}

// register uses an invitation once and creates a device.
func (b *Broker) register(secret, name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if !validDeviceName(name) {
		return "", "", fmt.Errorf("имя устройства: 1..%d символов в одну строку", MaxDeviceName)
	}
	idHex, err := randomHex(8)
	if err != nil {
		return "", "", err
	}
	key, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	id := "dev-" + idHex
	now := b.now().UTC().Format(time.RFC3339)
	// A read first, so a wrong secret never takes the database's write lock.
	var usable int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM invitations WHERE secret_hash=? AND used_at IS NULL AND expires_at>?`, hash(secret), now).Scan(&usable); err != nil {
		return "", "", err
	}
	if usable != 1 {
		return "", "", ErrInvitation
	}
	tx, err := b.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	// RFC 3339 UTC strings compare in time order.
	res, err := tx.Exec(`UPDATE invitations SET used_at=?, device_id=? WHERE secret_hash=? AND used_at IS NULL AND expires_at>?`, now, id, hash(secret), now)
	if err != nil {
		return "", "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return "", "", ErrInvitation
	}
	if _, err := tx.Exec(`INSERT INTO devices(id,name,key_hash,created_at) VALUES(?,?,?,?)`, id, name, hash(key), now); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, key, nil
}

// device returns the device that holds key, and records that it was seen.
func (b *Broker) device(key string) (Device, error) {
	var d Device
	var revoked sql.NullString
	err := b.db.QueryRow(`SELECT id,name,created_at,revoked_at FROM devices WHERE key_hash=?`, hash(key)).Scan(&d.ID, &d.Name, &d.CreatedAt, &revoked)
	if errors.Is(err, sql.ErrNoRows) || revoked.Valid {
		return Device{}, ErrRevoked
	}
	if err != nil {
		return Device{}, err
	}
	d.LastSeen = b.now().UTC().Format(time.RFC3339)
	_, err = b.db.Exec(`UPDATE devices SET last_seen=? WHERE id=?`, d.LastSeen, d.ID)
	return d, err
}

// OpenInvitations lists the IDs of invitations that can still be used.
func (b *Broker) OpenInvitations() ([]string, error) {
	rows, err := b.db.Query(`SELECT substr(secret_hash,1,16) FROM invitations WHERE used_at IS NULL AND expires_at>?`, b.now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Devices lists registered devices, newest first.
func (b *Broker) Devices() ([]Device, error) {
	rows, err := b.db.Query(`SELECT id,name,created_at,COALESCE(last_seen,''),COALESCE(revoked_at,'') FROM devices ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.CreatedAt, &d.LastSeen, &d.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Revoke revokes a device's key. It cannot be undone; the device joins again
// with a new invitation.
func (b *Broker) Revoke(id string) error {
	res, err := b.db.Exec(`UPDATE devices SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, b.now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("такого действующего устройства нет")
	}
	return b.forget(id)
}

// Listen takes the broker's serving lock and opens a TLS listener. Only one
// process serves a broker's data at a time.
func (b *Broker) Listen(addr string) (net.Listener, error) {
	if b.lock == nil {
		lock, err := platform.Acquire(filepath.Join(b.dir, "serve.lock"))
		if err != nil {
			if errors.Is(err, platform.ErrAlreadyRunning) {
				return nil, ErrBusy
			}
			return nil, err
		}
		b.lock = lock
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(newGuardedListener(ln), &tls.Config{Certificates: []tls.Certificate{b.cert}, MinVersion: tls.VersionTLS13}), nil
}

// newGuardedListener wraps ln with the peer check and connection limits.
func newGuardedListener(ln net.Listener) *guardedListener {
	return &guardedListener{Listener: ln, slots: make(chan struct{}, maxConnections), perSource: map[string]int{}}
}

// guardedListener accepts only peers on this computer or a private network
// and bounds open connections, in all and per source. "Local network"
// listens on every interface, so a peer from a public address (a public
// IPv6 address, for example) is dropped before TLS.
type guardedListener struct {
	net.Listener
	slots chan struct{}

	mu        sync.Mutex
	perSource map[string]int
}

func (l *guardedListener) Accept() (net.Conn, error) {
	for {
		l.slots <- struct{}{}
		conn, err := l.Listener.Accept()
		if err != nil {
			<-l.slots
			return nil, err
		}
		ip, ok := peerIP(conn.RemoteAddr())
		if !ok || !(ip.IsLoopback() || ip.IsPrivate()) {
			conn.Close()
			<-l.slots
			continue
		}
		if ip.IsLoopback() {
			return &slotConn{Conn: conn, release: func() { <-l.slots }}, nil
		}
		source := sourceOf(ip)
		l.mu.Lock()
		if l.perSource[source] >= maxConnectionsPerSource {
			l.mu.Unlock()
			conn.Close()
			<-l.slots
			continue
		}
		l.perSource[source]++
		l.mu.Unlock()
		return &slotConn{Conn: conn, release: func() {
			l.mu.Lock()
			if l.perSource[source]--; l.perSource[source] <= 0 {
				delete(l.perSource, source)
			}
			l.mu.Unlock()
			<-l.slots
		}}, nil
	}
}

// peerIP is the IP address of a TCP or UDP peer, IPv4 in its 4-byte form.
func peerIP(addr net.Addr) (net.IP, bool) {
	var ip net.IP
	switch a := addr.(type) {
	case *net.TCPAddr:
		ip = a.IP
	case *net.UDPAddr:
		ip = a.IP
	default:
		return nil, false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip, ip != nil
}

// localPeer reports whether addr is this computer or a private network.
func localPeer(addr net.Addr) bool {
	ip, ok := peerIP(addr)
	return ok && (ip.IsLoopback() || ip.IsPrivate())
}

type slotConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *slotConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}

// Serve answers the broker API on ln until ctx ends. It returns only after
// every request has finished or been cut off, so the caller may close the
// database right after.
func (b *Broker) Serve(ctx context.Context, ln net.Listener) error {
	server := &http.Server{Handler: b.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	server.SetKeepAlivesEnabled(false)
	stopped := make(chan struct{})
	served := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if server.Shutdown(shutdown) != nil {
				_ = server.Close()
			}
			cancel()
		case <-served:
			_ = server.Close()
		}
	}()
	err := server.Serve(ln)
	close(served)
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type hello struct {
	BrokerID string `json:"broker_id"`
	Protocol int    `json:"protocol"`
}

type registerRequest struct {
	Secret string `json:"secret"`
	Name   string `json:"name"`
}

type registerResponse struct {
	BrokerID  string `json:"broker_id"`
	DeviceID  string `json:"device_id"`
	DeviceKey string `json:"device_key"`
}

// PingResult is what the broker says about the calling device.
type PingResult struct {
	BrokerID string `json:"broker_id"`
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Time     string `json:"time"`
}

// Handler is the broker API.
func (b *Broker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, hello{BrokerID: b.id, Protocol: Protocol})
	})
	mux.HandleFunc("/v1/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		source := requestSource(r)
		if !b.allowRegistration(source) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts"})
			return
		}
		var in registerRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil || len(in.Secret) != 64 {
			b.registrationFailed(source)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		id, key, err := b.register(in.Secret, in.Name)
		switch {
		case errors.Is(err, ErrInvitation):
			b.registrationFailed(source)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invitation"})
		case err != nil && strings.HasPrefix(err.Error(), "имя устройства"):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		default:
			writeJSON(w, http.StatusOK, registerResponse{BrokerID: b.id, DeviceID: id, DeviceKey: key})
		}
	})
	mux.HandleFunc("/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		d, ok := b.authorized(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, PingResult{BrokerID: b.id, DeviceID: d.ID, Name: d.Name, Time: d.LastSeen})
	})
	// A device leaves the broker when its computer disconnects, so the
	// others do not keep seeing it as a computer that went away.
	mux.HandleFunc("/v1/leave", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		d, ok := b.authorized(w, r)
		if !ok {
			return
		}
		if err := b.Revoke(d.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{})
	})
	mux.HandleFunc("/v1/pair/start", b.handlePairStart)
	mux.HandleFunc("/v1/pair/finish", b.handlePairFinish)
	mux.HandleFunc("/v1/presence", b.handlePresence)
	mux.HandleFunc("/v1/tasks", b.handleSharedTasks)
	mux.HandleFunc("/v1/send", b.handleSend)
	mux.HandleFunc("/v1/inbox", b.handleInbox)
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
