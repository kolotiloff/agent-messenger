package updatefeed

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSignedChannelRejectsTamperingAndIncompatiblePackages(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	c := Channel{Directory: t.TempDir(), PublicKey: base64.StdEncoding.EncodeToString(pub)}
	archive := []byte("signed test archive")
	h := sha256.Sum256(archive)
	m := Manifest{Schema: 1, Build: 6, Version: "0.0.0-preview.6", Revision: strings.Repeat("a", 40), OS: runtime.GOOS, Arch: runtime.GOARCH, ClientAPI: 1, SHA256: hex.EncodeToString(h[:]), Length: int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, archive)), Notes: "Update <test> ]]>"}
	write := func(m Manifest) {
		t.Helper()
		raw, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(c.Directory, "manifest.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(c.Directory, "manifest.sig"), []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(m)
	os.WriteFile(filepath.Join(c.Directory, m.SHA256+".zip"), archive, 0600)
	if _, err := c.Load(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"other platform", func(m *Manifest) { m.Arch = "wrong" }},
		{"new protocol", func(m *Manifest) { m.ClientAPI = 2 }},
		{"traversal", func(m *Manifest) { m.SHA256 = "../../private" }},
		{"huge archive", func(m *Manifest) { m.Length = MaxArchive + 1 }},
		{"invalid signature", func(m *Manifest) { m.Signature = "invalid" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := m
			test.mutate(&bad)
			write(bad)
			if _, err := c.Load(); err == nil {
				t.Fatal("accepted incompatible update")
			}
		})
	}
	write(m)
	response := httptest.NewRecorder()
	c.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:1234/api/updates/feed.xml", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), m.Signature) || strings.Contains(response.Body.String(), "<test>") {
		t.Fatal(response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	c.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:1234/api/updates/"+m.SHA256+".zip", nil))
	if response.Code != 200 || response.Body.String() != string(archive) {
		t.Fatal("archive failed", response.Code)
	}
	os.WriteFile(filepath.Join(c.Directory, m.SHA256+".zip"), []byte("corruption"), 0600)
	response = httptest.NewRecorder()
	c.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:1234/api/updates/"+m.SHA256+".zip", nil))
	if response.Code != 503 {
		t.Fatal("corrupted archive served")
	}
	os.WriteFile(filepath.Join(c.Directory, "manifest.json"), []byte(`{"schema":1}`), 0600)
	if _, err := c.Load(); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	write(m)
	os.Remove(filepath.Join(c.Directory, "manifest.sig"))
	os.Symlink("manifest.json", filepath.Join(c.Directory, "manifest.sig"))
	if _, err := c.Load(); err == nil {
		t.Fatal("symlink accepted")
	}
}
