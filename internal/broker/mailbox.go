package broker

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"
)

// The mailbox carries messages between the computers of a broker (T-037).
// The broker only stores and hands out messages; it never reads their body.
// A sender names each message with an operation ID, and the same (sender,
// operation ID) is stored once while it waits, so a sender may send again
// after a lost answer. A recipient reads its messages in order and
// acknowledges them by passing the last sequence number it applied; the
// broker then deletes them. Receivers still deduplicate by operation ID,
// because a message can be sent again after it was acknowledged.

// Mailbox limits.
const (
	MaxMessageBody  = 256 << 10
	maxSendRequest  = MaxMessageBody + 4<<10
	maxPending      = 500
	InboxBatch      = 16
	maxInboxRequest = 1 << 10
)

var messageOp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,159}$`)

// Errors of the mailbox.
var (
	// ErrRecipient means the recipient is not a computer of the broker:
	// unknown or revoked. The message was not stored.
	ErrRecipient = errors.New("этого компьютера нет в сети")
	// ErrMailboxFull means the recipient has too many unread messages.
	ErrMailboxFull = errors.New("у компьютера слишком много непрочитанных сообщений")
	// ErrNoMailbox means the broker is older and cannot carry messages.
	ErrNoMailbox = errors.New("сервер не умеет передавать задачи: обновите программу на компьютере с сервером")
	errBadMessage = errors.New("сообщение не прошло проверку")
)

// Message is one message waiting for its recipient.
type Message struct {
	Seq  int64           `json:"seq"`
	From string          `json:"from"`
	OpID string          `json:"op_id"`
	Body json.RawMessage `json:"body"`
	At   string          `json:"at"`
}

type sendRequest struct {
	To   string          `json:"to"`
	OpID string          `json:"op_id"`
	Body json.RawMessage `json:"body"`
}

type inboxRequest struct {
	After int64 `json:"after"`
}

type inboxResponse struct {
	BrokerID string    `json:"broker_id"`
	DeviceID string    `json:"device_id"`
	Messages []Message `json:"messages"`
}

func (b *Broker) initMailbox() error {
	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS messages(
 seq INTEGER PRIMARY KEY AUTOINCREMENT,
 sender TEXT NOT NULL,
 recipient TEXT NOT NULL,
 op_id TEXT NOT NULL,
 body TEXT NOT NULL,
 created_at TEXT NOT NULL,
 UNIQUE(sender, op_id)
);
CREATE INDEX IF NOT EXISTS messages_recipient ON messages(recipient, seq);`)
	return err
}

// member reports whether peer is a computer of the broker: the hosting
// computer or a device that is not revoked.
func (b *Broker) member(peer string) (bool, error) {
	if peer == HostPeer {
		return true, nil
	}
	var n int
	err := b.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE id=? AND revoked_at IS NULL`, peer).Scan(&n)
	return n == 1, err
}

// Send stores a message from one computer of the broker to another. Sending
// the same (from, opID) again while it waits stores nothing new.
func (b *Broker) Send(from, to, opID string, body []byte) error {
	if !messageOp.MatchString(opID) || len(body) == 0 || len(body) > MaxMessageBody || !json.Valid(body) || body[0] != '{' || from == to {
		return errBadMessage
	}
	ok, err := b.member(to)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRecipient
	}
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var dup int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE sender=? AND op_id=?`, from, opID).Scan(&dup); err != nil {
		return err
	}
	if dup != 0 {
		return nil
	}
	var pending int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE recipient=?`, to).Scan(&pending); err != nil {
		return err
	}
	if pending >= maxPending {
		return ErrMailboxFull
	}
	if _, err := tx.Exec(`INSERT INTO messages(sender,recipient,op_id,body,created_at) VALUES (?,?,?,?,?)`, from, to, opID, string(body), b.now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// Inbox deletes the recipient's messages up to and including after, which
// the recipient has applied, and returns the next ones in order.
func (b *Broker) Inbox(peer string, after int64) ([]Message, error) {
	if _, err := b.db.Exec(`DELETE FROM messages WHERE recipient=? AND seq<=?`, peer, after); err != nil {
		return nil, err
	}
	rows, err := b.db.Query(`SELECT seq,sender,op_id,body,created_at FROM messages WHERE recipient=? AND seq>? ORDER BY seq LIMIT ?`, peer, after, InboxBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var body string
		if err := rows.Scan(&m.Seq, &m.From, &m.OpID, &body, &m.At); err != nil {
			return nil, err
		}
		m.Body = json.RawMessage(body)
		out = append(out, m)
	}
	return out, rows.Err()
}

// forget drops the messages to and from a revoked device. They can no longer
// be delivered or trusted.
func (b *Broker) forget(id string) error {
	_, err := b.db.Exec(`DELETE FROM messages WHERE recipient=? OR sender=?`, id, id)
	return err
}

func (b *Broker) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	d, ok := b.authorized(w, r)
	if !ok {
		return
	}
	var in sendRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxSendRequest))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	switch err := b.Send(d.ID, in.To, in.OpID, in.Body); {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"broker_id": b.id})
	case errors.Is(err, ErrRecipient):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "recipient"})
	case errors.Is(err, ErrMailboxFull):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "full"})
	case errors.Is(err, errBadMessage):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
	}
}

func (b *Broker) handleInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	d, ok := b.authorized(w, r)
	if !ok {
		return
	}
	var in inboxRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxInboxRequest))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || in.After < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	messages, err := b.Inbox(d.ID, in.After)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, inboxResponse{BrokerID: b.id, DeviceID: d.ID, Messages: messages})
}
