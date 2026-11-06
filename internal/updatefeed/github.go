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
	"runtime"
	"strconv"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const releaseAPI = "https://api.github.com/repos/kolotiloff/agent-messenger/releases"

type releaseAsset struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	State string `json:"state"`
}
type release struct {
	Draft  bool           `json:"draft"`
	Assets []releaseAsset `json:"assets"`
}
type SyncResult struct {
	Updated bool   `json:"updated"`
	Version string `json:"version,omitempty"`
	State   string `json:"state"`
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
	case "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
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
func uniqueAsset(assets []releaseAsset, name string, max int64) (releaseAsset, bool, error) {
	var found releaseAsset
	exists := false
	for _, a := range assets {
		if a.Name != name {
			continue
		}
		if exists || a.ID <= 0 || a.Size <= 0 || a.Size > max || a.State != "uploaded" {
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
		return result, errors.New("another update download owns the channel")
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
		ma, found, err := uniqueAsset(r.Assets, manifestName, 32768)
		if err != nil {
			return result, err
		}
		if !found {
			continue
		}
		sa, found, err := uniqueAsset(r.Assets, manifestName+".sig", 256)
		if err != nil || !found {
			return result, errors.New("release signature asset missing or invalid")
		}
		download := func(a releaseAsset, max int64) ([]byte, error) {
			b, e := remoteGet(ctx, client, releaseAPI+"/assets/"+strconv.FormatInt(a.ID, 10), "application/octet-stream", max)
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
		archive, found, err := uniqueAsset(r.Assets, m.SHA256+".zip", MaxArchive)
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
