package updatefeed

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// While a signed archive downloads, the downloader writes how much has
// arrived to a small file in the channel. The window reads it to fill its
// update icon (owner, 2026-11-14), whichever process downloads: the
// background check on Windows and Linux, or the app's fetch helper on a Mac.
// It only describes the download; nothing reads it to decide anything.

// progressFile is the file's name in the channel directory.
const progressFile = "download.progress"

// progressFresh is how old the file may be and still describe a download
// under way: a downloader that stopped without removing it is not shown.
const progressFresh = 30 * time.Second

// Progress is a download under way.
type Progress struct {
	Version string `json:"version"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
}

// Progress reports the download under way in this channel, if any.
func (c Channel) Progress() (Progress, bool) {
	path := filepath.Join(c.Directory, progressFile)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) > progressFresh {
		return Progress{}, false
	}
	raw, err := regularRead(path, 1024)
	if err != nil {
		return Progress{}, false
	}
	var p Progress
	if json.Unmarshal(raw, &p) != nil || p.Total <= 0 || p.Done < 0 || p.Done > p.Total {
		return Progress{}, false
	}
	return p, true
}

// progressWriter counts the bytes of a download and writes them to the
// progress file at most a few times a second.
type progressWriter struct {
	path    string
	version string
	total   int64
	done    int64
	last    time.Time
}

func (c Channel) newProgress(version string, total int64) *progressWriter {
	p := &progressWriter{path: filepath.Join(c.Directory, progressFile), version: version, total: total}
	p.save()
	return p
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if time.Since(p.last) >= 250*time.Millisecond {
		p.save()
	}
	return len(b), nil
}

func (p *progressWriter) save() {
	p.last = time.Now()
	done := p.done
	if done > p.total {
		done = p.total
	}
	raw, _ := json.Marshal(Progress{Version: p.version, Done: done, Total: p.total})
	tmp := p.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, p.path)
	}
}

// end removes the file: the download is over, whatever its result.
func (p *progressWriter) end() {
	_ = os.Remove(p.path)
	_ = os.Remove(p.path + ".tmp")
}

var _ io.Writer = (*progressWriter)(nil)
