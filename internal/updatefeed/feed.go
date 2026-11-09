// Package updatefeed verifies signed update packages and exposes them to Sparkle.
// Update transport never launches agents or touches client/session data.
package updatefeed

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const MaxArchive = 256 << 20
const ClientAPI = 1

type Manifest struct {
	Schema    int    `json:"schema"`
	Build     int    `json:"build"`
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	ClientAPI int    `json:"client_api"`
	SHA256    string `json:"sha256"`
	Length    int64  `json:"length"`
	Signature string `json:"signature"`
	Notes     string `json:"notes"`
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ShortVersion is the version people see: 0.58 for 0.0.0-preview.58. The
// full version stays in manifests and tags, which installed programs check.
func ShortVersion(version string) string {
	if versionPattern.MatchString(version) && strings.HasPrefix(version, "0.0.0-preview.") {
		return "0." + strings.TrimPrefix(version, "0.0.0-preview.")
	}
	return version
}

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-preview\.[1-9][0-9]{0,8}$`)

type Channel struct{ Directory, PublicKey string }

func (c Channel) Load() (Manifest, error) {
	if raw, err := regularRead(filepath.Join(c.Directory, "current.json"), 46000); err == nil {
		var envelope signedEnvelope
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&envelope) != nil || d.Decode(new(any)) != io.EOF {
			return Manifest{}, errors.New("invalid update envelope")
		}
		return VerifyManifest(c.PublicKey, envelope.Manifest, envelope.Signature)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, err
	}
	raw, err := regularRead(filepath.Join(c.Directory, "manifest.json"), 32768)
	if err != nil {
		return Manifest{}, err
	}
	sigText, err := regularRead(filepath.Join(c.Directory, "manifest.sig"), 256)
	if err != nil {
		return Manifest{}, err
	}
	return VerifyManifest(c.PublicKey, raw, sigText)
}

func VerifyManifest(publicKey string, raw, sigText []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > 32768 || len(sigText) > 256 {
		return m, errors.New("update metadata exceeds limits")
	}
	key, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return m, errors.New("update trust key unavailable")
	}
	sig, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(sigText)))
	if err != nil || !ed25519.Verify(key, raw, sig) {
		return m, errors.New("invalid update manifest signature")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil {
		return m, err
	}
	if d.Decode(new(any)) != io.EOF {
		return m, errors.New("invalid update manifest")
	}
	archiveSig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(archiveSig) != ed25519.SignatureSize || m.Schema != 1 || m.ClientAPI != ClientAPI ||
		m.OS != runtime.GOOS || m.Arch != runtime.GOARCH || m.Build <= 0 || m.Build > 999999999 ||
		!versionPattern.MatchString(m.Version) || !strings.HasSuffix(m.Version, fmt.Sprintf(".%d", m.Build)) || !revisionPattern.MatchString(m.Revision) || !digestPattern.MatchString(m.SHA256) ||
		m.Length <= 0 || m.Length > MaxArchive || len(m.Notes) > 16000 {
		return Manifest{}, errors.New("incompatible update manifest")
	}
	return m, nil
}

// Handler must be wrapped by the desktop HTTP capability/Host checks.
func (c Channel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m, err := c.Load()
	if err != nil {
		http.Error(w, "signed local update channel unavailable", http.StatusServiceUnavailable)
		return
	}
	name := m.SHA256 + ".zip"
	switch r.URL.Path {
	case "/api/updates/feed.xml":
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle"><channel><title>Agent Messenger — проверенные обновления</title><item><title>%s</title><sparkle:version>%d</sparkle:version><sparkle:shortVersionString>%s</sparkle:shortVersionString><sparkle:minimumSystemVersion>15.0</sparkle:minimumSystemVersion><description><![CDATA[%s]]></description><enclosure url="http://%s/api/updates/%s" sparkle:edSignature="%s" length="%d" type="application/octet-stream"/></item></channel></rss>`, escape(ShortVersion(m.Version)), m.Build, escape(ShortVersion(m.Version)), escape(m.Notes)+"\n\nФоновый клиент и активные сессии сохраняются. Обновляется приложение; версия уже работающего клиента может отличаться.", escape(r.Host), name, m.Signature, m.Length)
	case "/api/updates/" + name:
		f, err := openRegular(filepath.Join(c.Directory, name), MaxArchive)
		if err != nil {
			http.Error(w, "archive unavailable", 503)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil || stat.Size() != m.Length {
			http.Error(w, "archive size mismatch", 503)
			return
		}
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil || hex.EncodeToString(h.Sum(nil)) != m.SHA256 {
			http.Error(w, "archive checksum mismatch", 503)
			return
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "archive unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, name, stat.ModTime(), f)
	default:
		http.NotFound(w, r)
	}
}

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func regularRead(path string, max int64) ([]byte, error) {
	f, err := openRegular(path, max)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if len(raw) > int(max) {
		return nil, errors.New("file exceeds limit")
	}
	return raw, err
}

func openRegular(path string, max int64) (*os.File, error) {
	s, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !s.Mode().IsRegular() || s.Size() > max {
		return nil, errors.New("expected bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	t, err := f.Stat()
	if err != nil || !os.SameFile(s, t) {
		f.Close()
		return nil, errors.New("file changed while opening")
	}
	return f, nil
}
