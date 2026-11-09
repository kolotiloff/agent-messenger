package updatefeed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

var ErrDownloadBusy = errors.New("another update download owns the channel")

const releaseAPI = "https://api.github.com/repos/kolotiloff/agent-messenger/releases"

// releaseDownload is where release files are downloaded from. Unlike the
// API, these downloads do not count against GitHub's hourly limit per
// address, which several computers behind one address shared before.
const releaseDownload = "https://github.com/kolotiloff/agent-messenger/releases/download/"

var releaseTag = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

type releaseAsset struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	State string `json:"state"`
	URL   string `json:"browser_download_url"`
}
type release struct {
	Draft  bool           `json:"draft"`
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}
type SyncResult struct {
	Updated bool   `json:"updated"`
	Version string `json:"version,omitempty"`
	State   string `json:"state"`
	RetryAt string `json:"retry_at,omitempty"`
}

// RateLimitError says GitHub refused the check for its hourly limit per
// address. It is not a broken update: the check may be repeated after Reset.
type RateLimitError struct{ Reset time.Time }

func (e RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return "GitHub limits update checks from this address for now"
	}
	return "GitHub limits update checks from this address until " + e.Reset.Format(time.RFC3339)
}

// rateLimited reads GitHub's limit answer: 403 or 429 with no requests
// left, or with a Retry-After.
func rateLimited(response *http.Response) (RateLimitError, bool) {
	if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusTooManyRequests {
		return RateLimitError{}, false
	}
	if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds >= 0 && seconds < 86400 {
		return RateLimitError{Reset: time.Now().Add(time.Duration(seconds) * time.Second)}, true
	}
	if response.Header.Get("X-RateLimit-Remaining") != "0" {
		return RateLimitError{}, false
	}
	e := RateLimitError{}
	if unix, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && unix > 0 {
		if reset := time.Unix(unix, 0); reset.After(time.Now().Add(-time.Minute)) && reset.Before(time.Now().Add(24*time.Hour)) {
			e.Reset = reset
		}
	}
	return e, true
}

type signedEnvelope struct {
	Manifest  []byte `json:"manifest"`
	Signature []byte `json:"signature"`
}

// Only GitHub's API and fixed asset CDN hosts may receive requests. No account
// token, provider credential or local loopback capability enters this client.
func allowedUpdateURL(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	switch u.Hostname() {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}
func updateHTTPClient() *http.Client {
	return &http.Client{Timeout: 4 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 4 || !allowedUpdateURL(req.URL) {
			return errors.New("update redirect refused")
		}
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		return nil
	}}
}
func remoteGet(ctx context.Context, client *http.Client, address, accept string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil || !allowedUpdateURL(req.URL) {
		return nil, errors.New("update address refused")
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "Agent-Messenger-Updater")
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("GitHub update download unavailable")
	}
	defer response.Body.Close()
	if limit, ok := rateLimited(response); ok {
		return nil, limit
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub update returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > max {
		return nil, errors.New("update exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, max+1))
	if err != nil {
		return nil, errors.New("update download interrupted")
	}
	if int64(len(data)) > max {
		return nil, errors.New("update exceeds size limit")
	}
	return data, nil
}

// uniqueAsset finds the one asset with this name. Its download address must
// be exactly the release's own download address for that name.
func uniqueAsset(assets []releaseAsset, tag, name string, max int64) (releaseAsset, bool, error) {
	var found releaseAsset
	exists := false
	for _, a := range assets {
		if a.Name != name {
			continue
		}
		if exists || a.ID <= 0 || a.Size <= 0 || a.Size > max || a.State != "uploaded" || !releaseTag.MatchString(tag) || strings.ContainsAny(name, "/?#%\\") || a.URL != releaseDownload+tag+"/"+name {
			return found, false, errors.New("invalid or duplicate release asset")
		}
		found = a
		exists = true
	}
	return found, exists, nil
}

func (c Channel) SyncGitHub(ctx context.Context, minimumBuild int) (SyncResult, error) {
	return c.syncGitHub(ctx, minimumBuild, updateHTTPClient())
}
func (c Channel) syncGitHub(ctx context.Context, minimumBuild int, client *http.Client) (SyncResult, error) {
	var result SyncResult
	if minimumBuild < 0 {
		return result, errors.New("invalid installed build")
	}
	if err := os.MkdirAll(c.Directory, 0700); err != nil {
		return result, err
	}
	lock, err := platform.Acquire(filepath.Join(c.Directory, "download.lock"))
	if err != nil {
		if errors.Is(err, platform.ErrAlreadyRunning) {
			return result, ErrDownloadBusy
		}
		return result, errors.New("update channel lock unavailable")
	}
	defer lock.Release()
	current, currentErr := c.Load()
	if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) {
		return result, errors.New("existing update metadata invalid; channel unchanged")
	}
	floor := minimumBuild
	if currentErr == nil && current.Build > floor {
		floor = current.Build
	}
	raw, err := remoteGet(ctx, client, releaseAPI+"?per_page=20", "application/vnd.github+json", 2<<20)
	if err != nil {
		return result, err
	}
	var releases []release
	if json.Unmarshal(raw, &releases) != nil {
		return result, errors.New("invalid GitHub release listing")
	}
	manifestName := "update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	for _, r := range releases {
		if r.Draft {
			continue
		}
		ma, found, err := uniqueAsset(r.Assets, r.Tag, manifestName, 32768)
		if err != nil {
			return result, err
		}
		if !found {
			continue
		}
		sa, found, err := uniqueAsset(r.Assets, r.Tag, manifestName+".sig", 256)
		if err != nil || !found {
			return result, errors.New("release signature asset missing or invalid")
		}
		download := func(a releaseAsset, max int64) ([]byte, error) {
			b, e := remoteGet(ctx, client, a.URL, "application/octet-stream", max)
			if e == nil && int64(len(b)) != a.Size {
				e = errors.New("release asset length mismatch")
			}
			return b, e
		}
		manifest, err := download(ma, 32768)
		if err != nil {
			return result, err
		}
		signature, err := download(sa, 256)
		if err != nil {
			return result, err
		}
		m, err := VerifyManifest(c.PublicKey, manifest, signature)
		if err != nil {
			return result, err
		}
		if currentErr == nil && m.Build == current.Build && m.SHA256 != current.SHA256 {
			return result, errors.New("conflicting package for existing build")
		}
		if m.Build <= floor {
			return SyncResult{State: "current", Version: m.Version}, nil
		}
		archive, found, err := uniqueAsset(r.Assets, r.Tag, m.SHA256+".zip", MaxArchive)
		if err != nil || !found || archive.Size != m.Length {
			return result, errors.New("signed archive missing or wrong size")
		}
		// The size is bounded both by signed metadata and the response reader.
		// Download completes and is verified before any active channel file changes.
		data, err := download(archive, m.Length)
		if err != nil {
			return result, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != m.SHA256 {
			return result, errors.New("download checksum mismatch")
		}
		if err = c.publishDownloaded(m, manifest, signature, data); err != nil {
			return result, err
		}
		return SyncResult{Updated: true, Version: m.Version, State: "downloaded"}, nil
	}
	return SyncResult{State: "no_release"}, nil
}

// Archive is immutable; a single atomic envelope replaces the active metadata.
// Network/signature/size failures leave the previous update completely intact.
func (c Channel) publishDownloaded(m Manifest, manifest, signature, archive []byte) error {
	staging, err := os.MkdirTemp(c.Directory, ".download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	file := filepath.Join(staging, "archive.zip")
	if err = writeSynced(file, archive); err != nil {
		return err
	}
	dest := filepath.Join(c.Directory, m.SHA256+".zip")
	if err = os.Link(file, dest); err != nil {
		old, e := regularRead(dest, MaxArchive)
		if e != nil {
			return errors.New("archive cache unavailable")
		}
		sum := sha256.Sum256(old)
		if hex.EncodeToString(sum[:]) != m.SHA256 {
			return errors.New("archive cache checksum mismatch")
		}
	}
	envelope, err := json.Marshal(signedEnvelope{Manifest: manifest, Signature: signature})
	if err != nil {
		return err
	}
	metadata := filepath.Join(staging, "current.json")
	if err = writeSynced(metadata, envelope); err != nil {
		return err
	}
	if err = os.Rename(metadata, filepath.Join(c.Directory, "current.json")); err != nil {
		return err
	}
	dir, err := os.Open(c.Directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func writeSynced(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
