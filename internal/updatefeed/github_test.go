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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixturePackage(t *testing.T, key ed25519.PrivateKey, build int) (Manifest, []byte, []byte, []byte) {
	t.Helper()
	archive := []byte("archive for build " + strconv.Itoa(build))
	h := sha256.Sum256(archive)
	m := Manifest{Schema: 1, Build: build, Version: "0.0.0-preview." + strconv.Itoa(build), Revision: strings.Repeat("a", 40), OS: runtime.GOOS, Arch: runtime.GOARCH, ClientAPI: 1, SHA256: hex.EncodeToString(h[:]), Length: int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, archive))}
	raw, _ := json.Marshal(m)
	signature := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw)))
	return m, raw, signature, archive
}
func TestGitHubDownloadCommitAndFailurePreservePreviousPackage(t *testing.T) {
	for _, scenario := range []string{"valid", "tampered manifest", "tampered archive", "interrupted", "oversized", "duplicate asset", "missing signature", "http error", "downgrade", "conflict", "tag mismatch", "redirect", "bad redirect", "foreign address", "rate limit"} {
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
			tag := "v" + m.Version
			if scenario == "tag mismatch" {
				tag = "v0.0.0-preview.8"
			}
			at := func(n string) string { return releaseDownload + tag + "/" + n }
			assets := []releaseAsset{{ID: 1, Name: name, State: "uploaded", Size: int64(len(raw)), URL: at(name)}, {ID: 2, Name: name + ".sig", State: "uploaded", Size: int64(len(sig)), URL: at(name + ".sig")}, {ID: 3, Name: m.SHA256 + ".zip", State: "uploaded", Size: int64(len(archive)), URL: at(m.SHA256 + ".zip")}}
			if scenario == "duplicate asset" {
				assets = append(assets, assets[0])
			}
			if scenario == "foreign address" {
				assets[2].URL = "https://github.com/someone/else/releases/download/" + tag + "/" + m.SHA256 + ".zip"
			}
			if scenario == "missing signature" {
				assets = append(assets[:1], assets[2:]...)
			}
			list, _ := json.Marshal([]release{{Tag: tag, Assets: assets}})
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
				if req.URL.Host == "api.github.com" && req.URL.Path != "/repos/kolotiloff/agent-messenger/releases" {
					t.Fatalf("release file fetched through the API: %s", req.URL)
				}
				if scenario == "rate limit" && req.URL.Host == "api.github.com" {
					headers.Set("X-RateLimit-Remaining", "0")
					headers.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(20*time.Minute).Unix(), 10))
					status = 403
				}
				download := "/kolotiloff/agent-messenger/releases/download/" + tag + "/"
				switch strings.TrimPrefix(req.URL.Path, download) {
				case name:
					data = raw
					if scenario == "tampered manifest" {
						data = bytes.ReplaceAll(raw, []byte("preview"), []byte("invalid"))
					}
				case name + ".sig":
					data = sig
				case m.SHA256 + ".zip":
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
			if scenario == "valid" {
				// Finding the release downloads nothing and changes nothing.
				found, err := c.syncGitHub(context.Background(), 6, client, false)
				if err != nil || found.State != "found" || found.Version != m.Version || found.Updated {
					t.Fatal(found, err)
				}
				if got, e := c.Load(); e == nil && got.Build == 7 {
					t.Fatal("a find published the update")
				}
				if _, e := os.Stat(filepath.Join(c.Directory, m.SHA256+".zip")); e == nil {
					t.Fatal("a find downloaded the archive")
				}
			}
			result, err := c.syncGitHub(context.Background(), 6, client, true)
			success := scenario == "valid" || scenario == "redirect"
			if _, ok := c.Progress(); ok {
				t.Fatal("download progress left after the sync")
			}
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
				var limit RateLimitError
				if (scenario == "rate limit") != errors.As(err, &limit) || (scenario == "rate limit" && limit.Reset.IsZero()) {
					t.Fatalf("rate limit reported as %v", err)
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

func TestGitHubSelectsHighestBuildInUnorderedList(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	channel := Channel{Directory: t.TempDir(), PublicKey: base64.StdEncoding.EncodeToString(pub)}
	name := "update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	list := []release{}
	files := map[string][]byte{}
	for _, build := range []int{99, 98, 100, 101} {
		m, raw, sig, archive := fixturePackage(t, key, build)
		r := release{Tag: "v" + m.Version, Draft: build == 101}
		for _, asset := range []struct {
			name string
			data []byte
		}{{name, raw}, {name + ".sig", sig}, {m.SHA256 + ".zip", archive}} {
			address := releaseDownload + r.Tag + "/" + asset.name
			r.Assets = append(r.Assets, releaseAsset{
				ID: int64(len(r.Assets) + 1), Name: asset.name, State: "uploaded",
				Size: int64(len(asset.data)), URL: address,
			})
			files[address] = asset.data
		}
		list = append(list, r)
	}
	listing, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		address := req.URL.String()
		data := files[address]
		if address == releaseAPI+"?per_page=20" {
			data = listing
		}
		if data == nil {
			t.Fatalf("unexpected request: %s", address)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)),
			ContentLength: int64(len(data)), Request: req,
		}, nil
	})}
	for _, check := range []struct {
		fetch bool
		state string
	}{{false, "found"}, {true, "downloaded"}} {
		got, err := channel.syncGitHub(context.Background(), 99, client, check.fetch)
		if err != nil || got.Version != "0.0.0-preview.100" || got.State != check.state {
			t.Fatalf("fetch=%t: got %+v, error %v", check.fetch, got, err)
		}
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
	if _, err = c.syncGitHub(context.Background(), 0, client, true); !errors.Is(err, ErrDownloadBusy) {
		t.Fatalf("parallel update must return typed busy status, got %v", err)
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

func TestInvalidDownloadLockIsNotReportedAsBusy(t *testing.T) {
	c := Channel{Directory: t.TempDir()}
	if err := os.Mkdir(filepath.Join(c.Directory, "download.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	client := updateHTTPClient()
	client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network reached without a lock")
		return nil, nil
	})
	if _, err := c.syncGitHub(context.Background(), 0, client, true); err == nil || errors.Is(err, ErrDownloadBusy) {
		t.Fatalf("invalid lock must fail, not suggest waiting: %v", err)
	}
}

// trickle sends one byte per step and then, if stop, blocks until the
// request is cancelled.
type trickle struct {
	ctx  context.Context
	data []byte
	step time.Duration
	stop bool
}

func (t *trickle) Read(b []byte) (int, error) {
	if len(t.data) == 0 {
		if !t.stop {
			return 0, io.EOF
		}
		<-t.ctx.Done()
		return 0, t.ctx.Err()
	}
	select {
	case <-time.After(t.step):
	case <-t.ctx.Done():
		return 0, t.ctx.Err()
	}
	b[0] = t.data[0]
	t.data = t.data[1:]
	return 1, nil
}
func (t *trickle) Close() error { return nil }

// T-203: a slow transfer that keeps moving is not cut off; a stalled one is.
func TestDownloadEndsOnlyWhenStalled(t *testing.T) {
	oldStall, oldLimit := downloadStall, downloadLimit
	downloadStall, downloadLimit = 150*time.Millisecond, time.Minute
	defer func() { downloadStall, downloadLimit = oldStall, oldLimit }()
	address := releaseDownload + "v0.0.0-preview.9/archive.zip"
	for _, stop := range []bool{false, true} {
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			body := &trickle{ctx: r.Context(), data: []byte("0123456789"), step: 50 * time.Millisecond, stop: stop}
			return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body, Header: http.Header{}, Request: r}, nil
		})}
		// Ten bytes take 500 ms: longer than the stall limit, each byte within it.
		data, err := remoteGet(context.Background(), client, address, "application/octet-stream", 100)
		if !stop && (err != nil || string(data) != "0123456789") {
			t.Fatalf("slow download: %q %v", data, err)
		}
		if stop && (err == nil || !strings.Contains(err.Error(), "stalled")) {
			t.Fatalf("stalled download: %q %v", data, err)
		}
	}
}
