package broker

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// Limits of a short invitation (Q-017): it lives ShortTTL and allows
// ShortTries exchanges; each started exchange counts, so a wrong code burns
// a try even if the other side never finishes.
const (
	ShortTTL     = 10 * time.Minute
	ShortTries   = 3
	pairDeadline = time.Minute
)

type pairState struct {
	mu       sync.Mutex
	code     string
	secret   string
	expires  time.Time
	tries    int
	sessions map[string]pairSession
}

type pairSession struct {
	keys    pairKeys
	expires time.Time
}

// newShortInvite replaces the current short invitation with a new one for
// the long invitation's secret.
func (b *Broker) newShortInvite(secret string, now time.Time) (string, string, error) {
	code, err := newShortCode()
	if err != nil {
		return "", "", err
	}
	b.pair.mu.Lock()
	defer b.pair.mu.Unlock()
	expires := now.Add(ShortTTL)
	b.pair.code, b.pair.secret, b.pair.expires, b.pair.tries, b.pair.sessions = code, secret, expires, 0, map[string]pairSession{}
	return code, expires.Format(time.RFC3339), nil
}

// ShortActive says whether a short invitation can still be used; the
// discovery answer is sent only then.
func (b *Broker) ShortActive() bool {
	b.pair.mu.Lock()
	defer b.pair.mu.Unlock()
	return b.shortActiveLocked(b.now())
}

func (b *Broker) shortActiveLocked(now time.Time) bool {
	return b.pair.code != "" && now.Before(b.pair.expires) && b.pair.tries < ShortTries
}

func (b *Broker) burnShortLocked() {
	b.pair.code, b.pair.secret, b.pair.sessions = "", "", nil
}

type pairStartRequest struct {
	Msg []byte `json:"msg"`
}

type pairStartResponse struct {
	Session string `json:"session"`
	Msg     []byte `json:"msg"`
	Confirm []byte `json:"confirm"`
}

type pairFinishRequest struct {
	Session string `json:"session"`
	Confirm []byte `json:"confirm"`
}

type pairFinishResponse struct {
	Box []byte `json:"box"`
}

func decodeSmall(r *http.Request, v any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v) == nil
}

func (b *Broker) handlePairStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var in pairStartRequest
	if !decodeSmall(r, &in) || len(in.Msg) != 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	b.pair.mu.Lock()
	defer b.pair.mu.Unlock()
	now := b.now()
	if !b.shortActiveLocked(now) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "no invitation"})
		return
	}
	b.pair.tries++
	y, msg, err := pairStart(b.pair.code, pairN)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	shared, err := pairShared(b.pair.code, y, in.Msg, pairM)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	id, err := randomHex(16)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	keys := derivePairKeys(b.pair.code, b.id, b.fingerprint, in.Msg, msg, shared)
	b.pair.sessions[id] = pairSession{keys: keys, expires: now.Add(pairDeadline)}
	writeJSON(w, http.StatusOK, pairStartResponse{Session: id, Msg: msg, Confirm: keys.hostConfirm})
}

func (b *Broker) handlePairFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var in pairFinishRequest
	if !decodeSmall(r, &in) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	b.pair.mu.Lock()
	defer b.pair.mu.Unlock()
	session, ok := b.pair.sessions[in.Session]
	delete(b.pair.sessions, in.Session)
	now := b.now()
	if !ok || b.pair.code == "" || !now.Before(session.expires) || !now.Before(b.pair.expires) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "no invitation"})
		return
	}
	if !hmac.Equal(in.Confirm, session.keys.joinConfirm) {
		if b.pair.tries >= ShortTries {
			b.burnShortLocked()
		}
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "code"})
		return
	}
	box, err := seal(session.keys.box, []byte(b.pair.secret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unavailable"})
		return
	}
	// The code has done its job; the secret now works once, as a long
	// invitation's does.
	b.burnShortLocked()
	writeJSON(w, http.StatusOK, pairFinishResponse{Box: box})
}
