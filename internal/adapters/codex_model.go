package adapters

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/modelcatalog"
)

// CodexTurnModel reads only the model settings recorded for this worker's
// turn. The exec JSON stream identifies the thread but may omit its model.
func CodexTurnModel(profile, thread string, launched, taskCreated time.Time) string {
	return codexRollout(profile, thread, launched, taskCreated, func(path string) string {
		return readCodexTurnModel(path, launched)
	})
}

func codexRollout[T comparable](profile, thread string, launched, taskCreated time.Time, read func(string) T) T {
	var missing T
	if !uuidPattern.MatchString(thread) {
		return missing
	}
	dates := []time.Time{launched, taskCreated}
	if thread[14] == '7' {
		if ms, err := strconv.ParseInt(thread[:8]+thread[9:13], 16, 64); err == nil {
			dates = append(dates, time.UnixMilli(ms))
		}
	}
	seen := make(map[string]bool)
	suffix := "-" + thread + ".jsonl"
	for _, at := range dates {
		if at.IsZero() {
			continue
		}
		for _, offset := range []int{0, -1, 1} {
			day := at.Local().AddDate(0, 0, offset).Format("2006/01/02")
			if seen[day] {
				continue
			}
			seen[day] = true
			dir := filepath.Join(profile, "sessions", filepath.FromSlash(day))
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), suffix) {
					continue
				}
				if value := read(filepath.Join(dir, entry.Name())); value != missing {
					return value
				}
			}
		}
	}
	return missing
}

func readCodexRolloutTail(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	const maxTail = 1 << 20
	start := max(0, info.Size()-maxTail)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxTail))
	if err != nil {
		return nil
	}
	return raw
}

func readCodexTurnModel(path string, launched time.Time) string {
	raw := readCodexRolloutTail(path)
	lines := bytes.Split(raw, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		var row struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Payload   struct {
				Model  string `json:"model"`
				Effort string `json:"effort"`
			} `json:"payload"`
		}
		line := lines[i]
		if !bytes.Contains(line, []byte(`"turn_context"`)) ||
			json.Unmarshal(line, &row) != nil || row.Type != "turn_context" || row.Timestamp.Before(launched) {
			continue
		}
		base, _, err := modelcatalog.Split(row.Payload.Model)
		if err != nil || base == "" || base != row.Payload.Model {
			continue
		}
		if effort := row.Payload.Effort; effort != "" {
			value := base + "[effort=" + effort + "]"
			if _, _, err := modelcatalog.SplitReported(value); err == nil {
				return value
			}
		}
		return base
	}
	return ""
}
