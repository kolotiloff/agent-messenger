package broker

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	// ErrShortCode means the host refused the code: it is wrong, or the
	// exchange was tampered with.
	ErrShortCode = errors.New("код не подошёл: проверьте цифры. После трёх ошибок код перестаёт действовать, тогда создайте новое приглашение")
	// ErrShortGone means the host has no usable short invitation.
	ErrShortGone = errors.New("код больше не действует: он использован, устарел или было три ошибки. Создайте новое приглашение")
	// ErrNoHost means no host answered on the local network.
	ErrNoHost = errors.New("сервер в локальной сети не найден. Проверьте, что оба компьютера в одной сети и на сервере открыто приглашение, или вставьте длинный код")
)

// observingClient accepts any certificate and remembers its fingerprint.
// Nothing secret is sent through it: the exchange itself checks the
// certificate it saw.
func observingClient() (*http.Client, func() string) {
	var mu sync.Mutex
	var seen string
	verify := func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 {
			return ErrUntrusted
		}
		sum := sha256.Sum256(raw[0])
		mu.Lock()
		defer mu.Unlock()
		fp := hex.EncodeToString(sum[:])
		if seen != "" && seen != fp {
			return ErrUntrusted
		}
		seen = fp
		return nil
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 8 * time.Second}).DialContext,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		DisableKeepAlives:     true,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: verify, MinVersion: tls.VersionTLS13},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: noRedirects}, func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

// errUnproven means the server did not prove that it knows the code: a
// wrong code, or a server that is not the host. Nothing was sent that the
// server could use to check another code.
var errUnproven = errors.New("сервер не подтвердил код")

// errSkipped means the server was already asked in this search.
var errSkipped = errors.New("этот сервер уже проверен")

// PairShort proves the short code to the host at address and returns the
// long invitation it hands over.
func PairShort(ctx context.Context, address, code string) (Invitation, error) {
	inv, _, err := pairShort(ctx, address, code, nil)
	if errors.Is(err, errUnproven) {
		return Invitation{}, ErrShortCode
	}
	return inv, err
}

// pairShort is PairShort that also returns the certificate fingerprint the
// server showed, and asks nothing of a server whose fingerprint skip names.
func pairShort(ctx context.Context, address, code string, skip map[string]bool) (Invitation, string, error) {
	if !ValidShortCode(code) {
		return Invitation{}, "", ErrShortCode
	}
	if err := validAddress(address); err != nil {
		return Invitation{}, "", err
	}
	inv, err := pairExchange(ctx, address, code, skip)
	return inv, inv.Fingerprint, err
}

func pairExchange(ctx context.Context, address, code string, skip map[string]bool) (inv Invitation, err error) {
	client, seen := observingClient()
	defer func() {
		// The fingerprint goes back even with an error, so the search can
		// skip this server's other addresses.
		if err != nil {
			inv = Invitation{Fingerprint: seen()}
		}
	}()
	var h hello
	status, err := call(ctx, client, http.MethodGet, address, "/v1/hello", "", nil, &h)
	if err != nil {
		return Invitation{}, err
	}
	if status != http.StatusOK || !brokerID.MatchString(h.BrokerID) {
		return Invitation{}, errors.New("по этому адресу отвечает не сервер Messenger")
	}
	if h.Protocol != Protocol {
		return Invitation{}, fmt.Errorf("версия сервера (%d) не совпадает с этой программой (%d): обновите обе стороны", h.Protocol, Protocol)
	}
	if fp := seen(); fp == "" || skip[fp] {
		return Invitation{}, errSkipped
	}
	x, msg, err := pairStart(code, pairM)
	if err != nil {
		return Invitation{}, err
	}
	var started pairStartResponse
	status, err = call(ctx, client, http.MethodPost, address, "/v1/pair/start", "", pairStartRequest{Msg: msg}, &started)
	switch {
	case err != nil:
		return Invitation{}, err
	case status == http.StatusNotFound:
		return Invitation{}, errors.New("сервер старой версии не знает коротких кодов: обновите его или вставьте длинный код")
	case status == http.StatusGone:
		return Invitation{}, ErrShortGone
	case status != http.StatusOK:
		return Invitation{}, fmt.Errorf("сервер не принял код (код %d)", status)
	}
	fingerprint := seen()
	shared, err := pairShared(code, x, started.Msg, pairN)
	if err != nil || fingerprint == "" {
		return Invitation{}, errUnproven
	}
	keys := derivePairKeys(code, h.BrokerID, fingerprint, msg, started.Msg, shared)
	if !hmac.Equal(started.Confirm, keys.hostConfirm) {
		return Invitation{}, errUnproven
	}
	// From here the certificate is pinned to the one the exchange covered.
	var finished pairFinishResponse
	status, err = call(ctx, pinnedClient(fingerprint), http.MethodPost, address, "/v1/pair/finish", "", pairFinishRequest{Session: started.Session, Confirm: keys.joinConfirm}, &finished)
	switch {
	case err != nil:
		return Invitation{}, err
	case status == http.StatusForbidden:
		return Invitation{}, ErrShortCode
	case status == http.StatusGone:
		return Invitation{}, ErrShortGone
	case status != http.StatusOK:
		return Invitation{}, fmt.Errorf("сервер не принял код (код %d)", status)
	}
	secret, err := open(keys.box, finished.Box)
	if err != nil || !hexKey.MatchString(string(secret)) {
		return Invitation{}, errors.New("сервер ответил непонятно")
	}
	return Invitation{Address: address, BrokerID: h.BrokerID, Fingerprint: fingerprint, Secret: string(secret)}, nil
}
