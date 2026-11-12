// Package journal keeps a short record of what the client did: worker runs
// started and ended, messages sent to and received from other computers,
// and network problems (T-142). It is for debugging Messenger itself, also
// from another computer through get_log. It holds no task texts, reports,
// session ids or keys: only task ids, kinds, names, states and short error
// texts, which get_log cleans once more.
package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// File is the journal's name in the data directory; Previous is the one it
// rotated to.
const (
	File     = "events.log"
	Previous = "events.1.log"
)

// maxSize is the size at which the journal rotates. Two files keep about
// two days of a busy network.
var maxSize int64 = 2 << 20

var (
	mu   sync.Mutex
	path string
)

// Open starts writing the journal in root. Until it is called, Printf does
// nothing: tests and short commands write no journal.
func Open(root string) {
	mu.Lock()
	defer mu.Unlock()
	path = filepath.Join(root, File)
}

// Close stops writing; for tests.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	path = ""
}

// Printf writes one line with the time in UTC. A journal that cannot be
// written never stops the client.
func Printf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if path == "" {
		return
	}
	line := fmt.Sprintf(format, args...)
	line = strings.Join(strings.Fields(line), " ")
	if r := []rune(line); len(r) > 600 {
		line = string(r[:600]) + "…"
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxSize {
		_ = os.Rename(path, filepath.Join(filepath.Dir(path), Previous))
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), line)
}
