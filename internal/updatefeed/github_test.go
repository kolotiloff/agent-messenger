package updatefeed

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixturePackage(t *testing.T, key ed25519.PrivateKey, build int) (Manifest, []byte, []byte, []byte) {
	t.Helper()
	archive := []byte("archive for build " + string(rune('0'+build)))
	h := sha256.Sum256(archive)
	m := Manifest{Schema: 1, Build: build, Version: "0.0.0-preview." + string(rune('0'+build)), Revision: strings.Repeat("a", 40), OS: runtime.GOOS, Arch: runtime.GOARCH, ClientAPI: 1, SHA256: hex.EncodeToString(h[:]), Length: int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, archive))}
	raw, _ := json.Marshal(m)
	signature := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw)))
	return m, raw, signature, archive
}
func TestGitHubDownloadCommitAndFailurePreservePreviousPackage(t *testing.T) {
	for _, scenario := range []string{"valid", "tampered manifest", "tampered archive", "interrupted", "oversized", "duplicate asset", "missing signature", "http error", "downgrade", "conflict", "redirect", "bad redirect"} {
		t.Run(scenario, func(t *testing.T) {
			pub, key, _ := ed25519.GenerateKey(rand.Reader)
			c := Channel{Directory: t.TempDir(), PublicKey: base64.StdEncoding.EncodeToString(pub)}
			old, oldRaw, oldSig, oldZip := fixturePackage(t, key, 6)
			if err := c.publishDownloaded(old, oldRaw, oldSig, oldZip); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(c.Directory, "current.json"))
			m, raw, sig, archive := fixturePackage(t, key, 7)
			if scenario == "downgrade" {
				m, raw, sig, archive = fixturePackage(t, key, 5)
			}
			if scenario == "conflict" {
				m.Build = 6
				m.Version = "0.0.0-preview.6"
				raw, _ = json.Marshal(m)
				sig = []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw)))
			}
			name := "update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
			assets := []releaseAsset{{ID: 1, Name: name, State: "uploaded", Size: int64(len(raw))}, {ID: 2, Name: name + ".sig", State: "uploaded", Size: int64(len(sig))}, {ID: 3, Name: m.SHA256 + ".zip", State: "uploaded", Size: int64(len(archive))}}
			if scenario == "duplicate asset" {
				assets = append(assets, assets[0])
			}
			if scenario == "missing signature" {
				assets = append(assets[:1], assets[2:]...)
			}
			list, _ := json.Marshal([]release{{Assets: assets}})
			client := updateHTTPClient()
			requests := 0
			cdn := false
			client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
					t.Fatal("credential sent")
				}
				data := list
				status := 200
				headers := make(http.Header)
				switch req.URL.Path {
				case "/repos/kolotiloff/agent-messenger/releases/assets/1":
					data = raw
					if scenario == "tampered manifest" {
						data = bytes.ReplaceAll(raw, []byte("preview"), []byte("invalid"))
					}
				case "/repos/kolotiloff/agent-messenger/releases/assets/2":
					data = sig
				case "/repos/kolotiloff/agent-messenger/releases/assets/3":
					data = archive
					switch scenario {
					case "tampered archive":
						data = bytes.Repeat([]byte("x"), len(archive))
					case "oversized":
						data = append(archive, 'x')
					case "interrupted":
						return nil, errors.New("network failed")
					case "http error":
						status = 503
					case "redirect", "bad redirect":
						status = 302
						location := "https://release-assets.githubusercontent.com/package"
						if scenario == "bad redirect" {
							location = "https://attacker.example/package"
						}
						headers.Set("Location", location)
					}
				case "/package":
					cdn = true
					if req.URL.Host != "release-assets.githubusercontent.com" {
						t.Fatal("untrusted redirect was requested")
					}
					data = archive
				}
				return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req}, nil
			})
			result, err := c.syncGitHub(context.Background(), 6, client)
			success := scenario == "valid" || scenario == "redirect"
			if success {
				if err != nil || !result.Updated {
					t.Fatal(result, err)
				}
				got, e := c.Load()
				if e != nil || got.Build != 7 {
					t.Fatal(got, e)
				}
				zip, e := os.ReadFile(filepath.Join(c.Directory, m.SHA256+".zip"))
				if e != nil || !bytes.Equal(zip, archive) {
					t.Fatal("archive mismatch")
				}
				if scenario == "redirect" && !cdn {
					t.Fatal("CDN not used")
				}
			} else {
				if scenario == "downgrade" {
					if err != nil || result.Updated {
						t.Fatal(result, err)
					}
				} else if err == nil {
					t.Fatal("invalid update accepted")
				}
				after, _ := os.ReadFile(filepath.Join(c.Directory, "current.json"))
				if !bytes.Equal(before, after) {
					t.Fatal("previous channel changed")
				}
				got, e := c.Load()
				if e != nil || got.Build != 6 {
					t.Fatal("previous package unavailable")
				}
			}
			if requests == 0 {
				t.Fatal("transport not exercised")
			}
		})
	}
}
func TestUpdateChannelRejectsBrokenEnvelopeWithoutLegacyFallback(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	c := Channel{Directory: t.TempDir(), PublicKey: base64.StdEncoding.EncodeToString(pub)}
	_, raw, sig, _ := fixturePackage(t, key, 6)
	os.WriteFile(filepath.Join(c.Directory, "manifest.json"), raw, 0600)
	os.WriteFile(filepath.Join(c.Directory, "manifest.sig"), sig, 0600)
	os.WriteFile(filepath.Join(c.Directory, "current.json"), []byte("{"), 0600)
	if _, err := c.Load(); err == nil {
		t.Fatal("broken envelope fell back to older manifest")
	}
}
func TestGitHubDownloadLockRefusesConcurrentWriter(t *testing.T) {
	c := Channel{Directory: t.TempDir()}
	lock, err := platform.Acquire(filepath.Join(c.Directory, "download.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	client := updateHTTPClient()
	client.Transport = transportFunc(func(*http.Request) (*http.Response, error) { t.Fatal("network reached while locked"); return nil, nil })
	if _, err = c.syncGitHub(context.Background(), 0, client); err == nil {
		t.Fatal("parallel update accepted")
	}
}
func TestUpdateURLAllowlist(t *testing.T) {
	for _, value := range []string{"http://api.github.com/releases", "https://api.github.com.evil.test/releases", "https://user:pass@api.github.com/releases", "https://api.github.com:443/releases", "https://127.0.0.1/releases", "https://release-assets.githubusercontent.com.evil.test/x"} {
		u, _ := url.Parse(value)
		if allowedUpdateURL(u) {
			t.Fatal("accepted", value)
		}
	}
}
