package broker

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"strconv"

	"filippo.io/edwards25519"
)

// Short invitations (T-052, D-081). The host shows a 6-digit code; the
// joining computer finds the host on the local network and both prove they
// know the code with SPAKE2 over edwards25519, without sending it. Only after
// both confirmations does the host hand over the long invitation's secret,
// encrypted with the agreed key. A listener learns nothing it could test
// codes against offline; an active attacker gets one guess per try, and the
// host burns the code after ShortTries tries.
//
// The transcript binds the host's certificate fingerprint as the joining
// computer saw it, so a computer in the middle with its own certificate
// cannot complete the exchange even with the right code.

// ShortCodeDigits is the length of a short invitation code.
const ShortCodeDigits = 6

// pairM and pairN are SPAKE2's fixed points, derived from fixed strings so
// that nobody knows their discrete logarithms.
var pairM, pairN = seedPoint("agent-messenger pair M"), seedPoint("agent-messenger pair N")

func seedPoint(seed string) *edwards25519.Point {
	for i := 0; ; i++ {
		h := sha256.Sum256([]byte(seed + "/" + strconv.Itoa(i)))
		p, err := new(edwards25519.Point).SetBytes(h[:])
		if err != nil {
			continue
		}
		p.MultByCofactor(p)
		if p.Equal(edwards25519.NewIdentityPoint()) != 1 {
			return p
		}
	}
}

// ValidShortCode says whether s is a short invitation code.
func ValidShortCode(s string) bool {
	if len(s) != ShortCodeDigits {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// newShortCode draws a uniform 6-digit code.
func newShortCode() (string, error) {
	var b [4]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		// Rejection sampling keeps the code uniform.
		n := binary.BigEndian.Uint32(b[:])
		if n >= 4294000000 {
			continue
		}
		s := strconv.Itoa(int(n % 1000000))
		for len(s) < ShortCodeDigits {
			s = "0" + s
		}
		return s, nil
	}
}

func codeScalar(code string) *edwards25519.Scalar {
	h := sha512.Sum512([]byte("agent-messenger pair v1\x00" + code))
	s, _ := new(edwards25519.Scalar).SetUniformBytes(h[:])
	return s
}

func randomScalar() (*edwards25519.Scalar, error) {
	var b [64]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	return new(edwards25519.Scalar).SetUniformBytes(b[:])
}

// pairStart is one side's first step: its secret and its message
// x·G + w·blind, where blind is M for the joining computer and N for the
// host.
func pairStart(code string, blind *edwards25519.Point) (*edwards25519.Scalar, []byte, error) {
	x, err := randomScalar()
	if err != nil {
		return nil, nil, err
	}
	p := new(edwards25519.Point).ScalarBaseMult(x)
	p.Add(p, new(edwards25519.Point).ScalarMult(codeScalar(code), blind))
	return x, p.Bytes(), nil
}

var errPair = errors.New("pair: bad message")

// pairShared computes the shared point from the other side's message;
// theirBlind is the other side's fixed point.
func pairShared(code string, x *edwards25519.Scalar, theirs []byte, theirBlind *edwards25519.Point) ([]byte, error) {
	p, err := new(edwards25519.Point).SetBytes(theirs)
	if err != nil {
		return nil, errPair
	}
	p.Subtract(p, new(edwards25519.Point).ScalarMult(codeScalar(code), theirBlind))
	p.MultByCofactor(p)
	if p.Equal(edwards25519.NewIdentityPoint()) == 1 {
		return nil, errPair
	}
	return p.ScalarMult(x, p).Bytes(), nil
}

// pairKeys are the keys both sides derive from one exchange.
type pairKeys struct {
	hostConfirm, joinConfirm, box []byte
}

func derivePairKeys(code, brokerID, fingerprint string, joinMsg, hostMsg, shared []byte) pairKeys {
	t := sha256.New()
	for _, part := range [][]byte{[]byte("agent-messenger pair v1"), []byte(brokerID), []byte(fingerprint), joinMsg, hostMsg, shared, codeScalar(code).Bytes()} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(part)))
		t.Write(n[:])
		t.Write(part)
	}
	key := t.Sum(nil)
	mac := func(label string) []byte {
		m := hmac.New(sha256.New, key)
		m.Write([]byte(label))
		return m.Sum(nil)
	}
	return pairKeys{hostConfirm: mac("host confirms"), joinConfirm: mac("joining computer confirms"), box: mac("invitation secret")}
}

// seal and open protect the long invitation's secret with the agreed key.
func seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func open(key, box []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(box) < gcm.NonceSize() {
		return nil, errPair
	}
	return gcm.Open(nil, box[:gcm.NonceSize()], box[gcm.NonceSize():], nil)
}
