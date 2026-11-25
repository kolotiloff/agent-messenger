package journal

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// These are the temporary files created by the background client and
// update helper. Other logs, user files and task history are independent.
var helperLogName = regexp.MustCompile(`^(background|remote-update)-[0-9]+\.log$`)

func pruneHelperLogs(root string, cutoff time.Time, active ...os.FileInfo) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	type oldLog struct {
		name, kind string
		info       os.FileInfo
	}
	var expired []oldLog
	latest := map[string]oldLog{}
	for _, e := range entries {
		match := helperLogName.FindStringSubmatch(e.Name())
		if match == nil || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		log := oldLog{e.Name(), match[1], info}
		prev := latest[log.kind]
		if info.Size() > 0 && (prev.info == nil || info.ModTime().After(prev.info.ModTime())) {
			latest[log.kind] = log
		}
		if info.ModTime().Before(cutoff) {
			expired = append(expired, log)
		}
	}
	for _, log := range expired {
		if latest[log.kind].name == log.name {
			continue
		}
		using := false
		for _, info := range active {
			using = using || (info != nil && os.SameFile(info, log.info))
		}
		if !using {
			_ = os.Remove(filepath.Join(root, log.name))
		}
	}
}
