package supervisor

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// Giving this computer's log to the network (T-057, D-083). The client
// answers other computers' requests only when the owner turned on
// «Отдавать журнал по сети»; it sends the last lines of its newest
// background log, at most maxLogBytes, with session ids, invitation codes,
// keys and long tokens removed. Paths and task texts stay: they are what a
// debugging coordinator needs.

const maxLogBytes = core.MaxLogText

// logRefusedOff is the reply when the owner did not turn sharing on.
const logRefusedOff = "the owner of that computer has not turned on «Отдавать журнал по сети» on its «Сервер» tab"

// answerLogRequests answers the other computers' log requests.
func (s *Supervisor) answerLogRequests(broker string) string {
	reqs, err := s.store.NetLogRequests(broker)
	if err != nil {
		return "запросы журнала не читаются: " + err.Error()
	}
	if len(reqs) == 0 {
		return ""
	}
	share, err := s.store.NetLogShare()
	if err != nil {
		return "настройка журнала не читается: " + err.Error()
	}
	for _, r := range reqs {
		text, refusal := "", logRefusedOff
		if share {
			if t, err := logTail(s.root, r.Lines); err != nil {
				refusal = err.Error()
			} else {
				text, refusal = redactLog(t), ""
			}
		}
		if err := s.store.ReplyNetLog(r, text, refusal); err != nil {
			return "ответ на запрос журнала не записан: " + err.Error()
		}
	}
	return ""
}

// logTail returns the last lines of the newest background log in root,
// within maxLogBytes.
func logTail(root string, lines int) (string, error) {
	files, _ := filepath.Glob(filepath.Join(root, "background-*.log"))
	newest, newestAt := "", int64(0)
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil && fi.Mode().IsRegular() && fi.ModTime().UnixNano() >= newestAt {
			newest, newestAt = f, fi.ModTime().UnixNano()
		}
	}
	if newest == "" {
		return "", errors.New("this computer has no background log: its client was not started in the background")
	}
	f, err := os.Open(newest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return "", err
	}
	start := size - 4*maxLogBytes
	if start < 0 {
		start = 0
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	text := string(buf)
	if start > 0 {
		// Drop the line cut by the read.
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	all := strings.SplitAfter(text, "\n")
	if len(all) > 0 && all[len(all)-1] == "" {
		all = all[:len(all)-1]
	}
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	total := 0
	for _, l := range all {
		total += len(l)
	}
	for len(all) > 1 && total > maxLogBytes {
		total -= len(all[0])
		all = all[1:]
	}
	out := strings.Join(all, "")
	if len(out) > maxLogBytes {
		// One line longer than the limit keeps its end.
		out = strings.ToValidUTF8(out[len(out)-maxLogBytes:], "")
	}
	return out, nil
}

var (
	logSecretWord = regexp.MustCompile(`(?i)\b(authorization|bearer|token|secret|password|passwd|api[_-]?key|access[_-]?key|cookie)\b(["']?\s*[:=]\s*|\s+)("[^"]*"|\S+)`)
	logInvite     = regexp.MustCompile(`am1:\S+`)
	logUUID       = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	logHex        = regexp.MustCompile(`\b[0-9a-fA-F]{24,}\b`)
	logLongToken  = regexp.MustCompile(`[A-Za-z0-9+_=-]{32,}`)
)

// redactLog removes what must not leave the computer: session ids,
// invitation codes, keys and long tokens.
func redactLog(text string) string {
	text = logSecretWord.ReplaceAllString(text, "${1}${2}[скрыто]")
	text = logInvite.ReplaceAllString(text, "am1:[скрыто]")
	text = logUUID.ReplaceAllString(text, "[id]")
	text = logHex.ReplaceAllString(text, "[hex]")
	return logLongToken.ReplaceAllStringFunc(text, func(s string) string {
		// A run of letters with digits is a token; a long word or a line
		// of dashes is not.
		if strings.ContainsAny(s, "0123456789") && strings.IndexFunc(s, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0 {
			return "[скрыто]"
		}
		return s
	})
}
