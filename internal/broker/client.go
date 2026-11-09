package broker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// InvitePrefix starts every invitation code.
const InvitePrefix = "am1:"

var (
	hexKey     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	brokerID   = regexp.MustCompile(`^brk-[0-9a-f]{16}$`)
	deviceID   = regexp.MustCompile(`^dev-[0-9a-f]{16}$`)
	errAddress = errors.New("адрес сервера должен быть вида хост:порт")
)

// Invitation is what a device needs to join a broker. It is shown to the
// owner once and is valid for InviteTTL.
type Invitation struct {
	Address     string `json:"a"`
	BrokerID    string `json:"b"`
	Fingerprint string `json:"f"`
	Secret      string `json:"s"`
}

func (i Invitation) String() string {
	raw, _ := json.Marshal(i)
	return InvitePrefix + base64.RawURLEncoding.EncodeToString(raw)
}

// ParseInvitation reads an invitation code.
func ParseInvitation(code string) (Invitation, error) {
	var i Invitation
	bad := errors.New("это не приглашение Messenger: скопируйте код целиком")
	rest, ok := strings.CutPrefix(strings.TrimSpace(code), InvitePrefix)
	if !ok || rest == "" || len(rest) > 1024 {
		return i, bad
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
	if err != nil {
		return i, bad
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&i) != nil || decoder.Decode(new(any)) != io.EOF || !brokerID.MatchString(i.BrokerID) || !hexKey.MatchString(i.Fingerprint) || !hexKey.MatchString(i.Secret) {
		return Invitation{}, bad
	}
	if err := validAddress(i.Address); err != nil {
		return Invitation{}, err
	}
	return i, nil
}

func validAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" || len(address) > 255 || strings.ContainsAny(address, "/?#@ ") {
		return errAddress
	}
	return nil
}

// Connection is this device's membership in a broker. DeviceKey is a secret.
type Connection struct {
	Address     string `json:"address"`
	BrokerID    string `json:"broker_id"`
	Fingerprint string `json:"fingerprint"`
	DeviceID    string `json:"device_id"`
	DeviceKey   string `json:"device_key"`
	DeviceName  string `json:"device_name"`
	ConnectedAt string `json:"connected_at"`
}

// Valid reports whether a stored connection is well formed.
func (c Connection) Valid() bool {
	return validAddress(c.Address) == nil && brokerID.MatchString(c.BrokerID) && hexKey.MatchString(c.Fingerprint) && deviceID.MatchString(c.DeviceID) && hexKey.MatchString(c.DeviceKey)
}

// pinnedClient trusts exactly one certificate: the one whose SHA-256 is
// fingerprint. Host names are not used for trust; the pin replaces them.
func pinnedClient(fingerprint string) *http.Client {
	verify := func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 {
			return ErrUntrusted
		}
		sum := sha256.Sum256(raw[0])
		if hex.EncodeToString(sum[:]) != fingerprint {
			return ErrUntrusted
		}
		return nil
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 8 * time.Second}).DialContext,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		DisableKeepAlives:     true,
		// InsecureSkipVerify only turns off the CA and host name check; the
		// certificate is still verified against the pinned fingerprint.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: verify, MinVersion: tls.VersionTLS13},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: noRedirects}
}

// noRedirects keeps a client at the address it was given: a redirect would
// carry the device key to another server, possibly over plain HTTP. The
// redirect answer itself is returned, and call treats it as an error.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// call makes one request. A pin mismatch is ErrUntrusted; an unreachable
// server is a plain error that says so.
func call(ctx context.Context, client *http.Client, method, address, path, key string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://"+address+path, reader)
	if err != nil {
		return 0, errAddress
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, ErrUntrusted) {
			return 0, ErrUntrusted
		}
		return 0, fmt.Errorf("сервер %s недоступен: %w", address, unwrapNet(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return resp.StatusCode, fmt.Errorf("сервер %s ответил перенаправлением (код %d); оно не выполняется", address, resp.StatusCode)
	}
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(out); err != nil {
			return resp.StatusCode, errors.New("сервер ответил непонятно")
		}
	}
	if fail, ok := out.(*apiError); ok && resp.StatusCode != http.StatusOK {
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(fail)
	}
	return resp.StatusCode, nil
}

// maxResponse bounds an answer: a full inbox batch of the largest messages.
const maxResponse = InboxBatch*MaxMessageBody + 64<<10

// apiError is the broker's {"error": ...} answer.
type apiError struct {
	Error string `json:"error"`
}

// unwrapNet says in plain words why a server could not be reached.
func unwrapNet(err error) error {
	var dns *net.DNSError
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return errors.New("соединение отклонено: сервер не запущен или порт закрыт")
	case errors.As(err, &dns):
		return errors.New("такого имени компьютера нет в сети")
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &timeout) && timeout.Timeout():
		return errors.New("сервер не ответил вовремя: проверьте сеть, брандмауэр и адрес")
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return errors.New("до этого адреса нет сети")
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		return op.Err
	}
	return err
}

// identify checks that the server is the expected broker and speaks this
// protocol, before anything secret is sent.
func identify(ctx context.Context, client *http.Client, address, expected string) error {
	var h hello
	code, err := call(ctx, client, http.MethodGet, address, "/v1/hello", "", nil, &h)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return errors.New("по этому адресу отвечает не сервер Messenger")
	}
	if h.BrokerID != expected {
		return ErrIdentity
	}
	if h.Protocol != Protocol {
		return fmt.Errorf("версия сервера (%d) не совпадает с этой программой (%d): обновите обе стороны", h.Protocol, Protocol)
	}
	return nil
}

// Register joins the broker named by the invitation. The certificate and
// the broker identity are checked before the invitation secret is sent.
func Register(ctx context.Context, inv Invitation, name string) (Connection, error) {
	client := pinnedClient(inv.Fingerprint)
	if err := identify(ctx, client, inv.Address, inv.BrokerID); err != nil {
		return Connection{}, err
	}
	var out registerResponse
	code, err := call(ctx, client, http.MethodPost, inv.Address, "/v1/register", "", registerRequest{Secret: inv.Secret, Name: strings.TrimSpace(name)}, &out)
	if err != nil {
		return Connection{}, err
	}
	switch code {
	case http.StatusOK:
	case http.StatusForbidden:
		return Connection{}, ErrInvitation
	case http.StatusTooManyRequests:
		return Connection{}, errors.New("сервер получил слишком много неверных приглашений; подождите минуту и повторите")
	case http.StatusBadRequest:
		return Connection{}, fmt.Errorf("имя устройства: 1..%d символов в одну строку", MaxDeviceName)
	default:
		return Connection{}, fmt.Errorf("сервер не смог зарегистрировать устройство (код %d)", code)
	}
	c := Connection{Address: inv.Address, BrokerID: inv.BrokerID, Fingerprint: inv.Fingerprint, DeviceID: out.DeviceID, DeviceKey: out.DeviceKey, DeviceName: strings.TrimSpace(name), ConnectedAt: time.Now().UTC().Format(time.RFC3339)}
	if out.BrokerID != inv.BrokerID || !c.Valid() {
		return Connection{}, errors.New("сервер ответил непонятно")
	}
	return c, nil
}

// Check asks the broker whether this device is still a member.
func Check(ctx context.Context, c Connection) (PingResult, error) {
	client := pinnedClient(c.Fingerprint)
	if err := identify(ctx, client, c.Address, c.BrokerID); err != nil {
		return PingResult{}, err
	}
	var out PingResult
	code, err := call(ctx, client, http.MethodGet, c.Address, "/v1/ping", c.DeviceKey, nil, &out)
	if err != nil {
		return PingResult{}, err
	}
	if code == http.StatusUnauthorized {
		return PingResult{}, ErrRevoked
	}
	if code != http.StatusOK || out.BrokerID != c.BrokerID {
		return PingResult{}, fmt.Errorf("сервер не подтвердил устройство (код %d)", code)
	}
	return out, nil
}

// Leave removes this device from the broker. A broker older than leaving
// answers 404; the device then stays listed until the host revokes it.
func Leave(ctx context.Context, c Connection) error {
	var fail apiError
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/leave", c.DeviceKey, struct{}{}, &fail)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusUnauthorized {
		return fmt.Errorf("сервер не удалил устройство (код %d)", code)
	}
	return nil
}

// Report sends this device's presence and returns the broker's list of
// computers. The pinned certificate and the broker ID in the answer identify
// the broker, so no separate hello is needed every time.
// errPresenceRefused means the broker found the report invalid, as an older
// host does with a field it does not know.
var errPresenceRefused = errors.New("сервер не принял сведения об агентах (код 400)")

func Report(ctx context.Context, c Connection, p Presence) ([]Peer, error) {
	var out rosterResponse
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/presence", c.DeviceKey, p, &out)
	if err != nil {
		return nil, err
	}
	if code == http.StatusUnauthorized {
		return nil, ErrRevoked
	}
	if code == http.StatusBadRequest {
		return nil, errPresenceRefused
	}
	if code != http.StatusOK || out.BrokerID != c.BrokerID || out.DeviceID != c.DeviceID {
		return nil, fmt.Errorf("сервер не принял сведения об агентах (код %d)", code)
	}
	if out.Peers == nil {
		out.Peers = []Peer{}
	}
	return out.Peers, nil
}

// SendMessage stores a message for another computer of the broker. It is
// safe to send the same opID again after a lost answer.
func SendMessage(ctx context.Context, c Connection, to, opID string, body []byte) error {
	var fail apiError
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/send", c.DeviceKey, sendRequest{To: to, OpID: opID, Body: json.RawMessage(body)}, &fail)
	if err != nil {
		return err
	}
	switch {
	case code == http.StatusOK:
		return nil
	case code == http.StatusUnauthorized:
		return ErrRevoked
	case code == http.StatusNotFound && fail.Error == "recipient":
		return ErrRecipient
	case code == http.StatusNotFound:
		return ErrNoMailbox
	case code == http.StatusTooManyRequests:
		return ErrMailboxFull
	case code == http.StatusBadRequest:
		return ErrBadMessage
	}
	return fmt.Errorf("сервер не принял сообщение (код %d)", code)
}

// ReadInbox acknowledges this device's messages up to after and returns the
// next ones.
func ReadInbox(ctx context.Context, c Connection, after int64) ([]Message, error) {
	var out inboxResponse
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/inbox", c.DeviceKey, inboxRequest{After: after}, &out)
	if err != nil {
		return nil, err
	}
	switch {
	case code == http.StatusUnauthorized:
		return nil, ErrRevoked
	case code == http.StatusNotFound:
		return nil, ErrNoMailbox
	case code != http.StatusOK || out.BrokerID != c.BrokerID || out.DeviceID != c.DeviceID:
		return nil, fmt.Errorf("сервер не выдал сообщения (код %d)", code)
	}
	if out.Messages == nil {
		out.Messages = []Message{}
	}
	return out.Messages, nil
}
