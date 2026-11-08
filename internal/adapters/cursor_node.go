package adapters

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Cursor on Windows (D-052). Its installer puts each version in
// %LOCALAPPDATA%\cursor-agent\versions\<version>\ as node.exe and index.js,
// and the command the owner types (agent.cmd) only runs that pair through
// PowerShell. Messenger never starts .cmd scripts (D-044), so a Cursor agent
// on Windows is registered as the node.exe of a version folder, and each
// start uses the newest version folder of that installation, as Cursor's own
// script does.

var cursorVersionName = regexp.MustCompile(`^(\d{4})\.(\d{1,2})\.(\d{1,2})(-\d{2}-\d{2}-\d{2})?-[a-f0-9]+$`)

// cursorNodeVersion reports whether cli is the node.exe of a Cursor version
// folder, and returns that installation's versions folder.
func cursorNodeVersion(cli string) (string, bool) {
	if !strings.EqualFold(filepath.Base(cli), "node.exe") {
		return "", false
	}
	dir := filepath.Dir(cli)
	versions := filepath.Dir(dir)
	if !strings.EqualFold(filepath.Base(versions), "versions") || !cursorVersionName.MatchString(filepath.Base(dir)) {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(dir, "index.js")); err != nil {
		return "", false
	}
	return versions, true
}

// LatestCursorVersion returns the newest version folder in a Cursor
// versions folder that has node.exe and index.js, or "".
func LatestCursorVersion(versions string) string {
	entries, err := os.ReadDir(versions)
	if err != nil {
		return ""
	}
	type candidate struct {
		date int
		name string
	}
	var found []candidate
	for _, e := range entries {
		m := cursorVersionName.FindStringSubmatch(e.Name())
		if !e.IsDir() || m == nil {
			continue
		}
		dir := filepath.Join(versions, e.Name())
		if !regularFile(filepath.Join(dir, "node.exe")) || !regularFile(filepath.Join(dir, "index.js")) {
			continue
		}
		month, day := m[2], m[3]
		date, _ := strconv.Atoi(m[1] + strings.Repeat("0", 2-len(month)) + month + strings.Repeat("0", 2-len(day)) + day)
		found = append(found, candidate{date, e.Name()})
	}
	if len(found) == 0 {
		return ""
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].date != found[j].date {
			return found[i].date > found[j].date
		}
		return found[i].name > found[j].name
	})
	return filepath.Join(versions, found[0].name)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// CursorProgram is how a registered Cursor CLI path is started: the path
// itself, or for a Windows version folder the newest version's node.exe with
// its index.js. Nothing is run here.
func CursorProgram(cli string) []string {
	versions, ok := cursorNodeVersion(cli)
	if !ok {
		return []string{cli}
	}
	dir := LatestCursorVersion(versions)
	if dir == "" {
		dir = filepath.Dir(cli)
	}
	return []string{filepath.Join(dir, "node.exe"), filepath.Join(dir, "index.js")}
}

// cursorProgramEnv is what Cursor's own script sets before it starts
// node.exe.
func cursorProgramEnv(cli string) []string {
	if _, ok := cursorNodeVersion(cli); !ok {
		return nil
	}
	return []string{"CURSOR_INVOKED_AS=agent.cmd"}
}
